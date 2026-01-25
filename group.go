package promise

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asmsh/sema"
)

// Group represents a group of [Promise] values that operate on the same
// input and result types, offering methods to start new promises, wait
// for all ongoing ones, and have a centralized handling for their failures.
//
// Promises from the same [Group] share the same goroutines pool, if one
// is set.
//
// A Group is safe for concurrent use by multiple goroutines.
//
// The zero value is a ready to use Group with the [DefaultGroupConfig].
type Group[T any] struct {
	core *groupCore
	once sync.Once

	// callsQ is used for registering and tracking the [Group]'s join calls,
	// like [Group.AnyRes], [Group.AllWaitRes], etc.
	callsQMu sync.RWMutex
	callsQ   list.List // of groupCall[T]

	// resQ is used to hold all [Result] values returned from promises
	// that belong to this [Group] when the [GroupConfig.SaveAllGroupResults]
	// option is set.
	// it's a linked-list instead of an array, because the size is unpredictable.
	// resHist holds the history of [Result] values for either first values
	// or the last ones, depending on the [saveLastSingleGroupResult] flag.
	// resMu protects reads and writes to them.
	resMu   sync.RWMutex
	resQ    list.List // of GroupRes[T]
	resHist groupResHistory[T]
}

func NewGroup[T any](opts ...GroupOption) *Group[T] {
	g := &Group[T]{}
	g.init()
	for _, opt := range opts {
		opt(g.core)
	}
	return g
}

func (g *Group[T]) init() {
	if g == nil {
		return
	}
	g.once.Do(func() {
		g.core = &groupCore{}
	})
}

func (g *Group[T]) initValidateState() *Promise[T] {
	// make sure the group is initiated.
	g.init()

	// return an error if the group is in the waiting mode,
	// disallowing the initiation of any new work.
	if errRes := g.validateActive(); errRes != nil {
		return newPromSync[T](g, errRes)
	}

	// attempt to reserve a goroutine for the callback,
	// or return an error if there's no available ones.
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errGroupBusyResult[T]{})
	}

	return nil
}

func noopRegFunc() {
	// do nothing
}

func (g *Group[T]) Go(cb func()) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(goFunc[T, T](cb))
}

func (g *Group[T]) GoErr(cb func() error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(goNextErrFunc[T, T](cb))
}

func (g *Group[T]) GoValErr(cb func() (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(goNextValErrFunc[T, T](cb))
}

func (g *Group[T]) GoCtxErr(cb func(ctx context.Context) error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(ctxNextErrFunc[T, T](cb))
}

func (g *Group[T]) GoCtxValErr(cb func(ctx context.Context) (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(ctxNextValErrFunc[T, T](cb))
}

func (g *Group[T]) GoCtxRes(cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return g.GoCallback(ctxNextResFunc[T, T](cb))
}

func (g *Group[T]) GoCallback(cb Callback[T, T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	// make sure the group and in a valid state, otherwise return
	// the error promise created.
	if errProm := g.initValidateState(); errProm != nil {
		return errProm
	}

	// create the new promise that will track the provided callback.
	nextProm := newPromInter[T](g)

	// derive the Context used in the callback from the group's, or from
	// the new promise, effectively making the created Context closed
	// once the callback returns.
	ctx, cancel := callbackCtx(g, nextProm.syncChan)

	debug(nextProm, startHandler, startGroupHandler, startGroupGoHandler)
	go goCallbackHandler(nextProm, cb, ctx, cancel)

	return nextProm
}

func goCallbackHandler[NextT, PrevT any](
	nextProm *Promise[NextT],
	cb Callback[NextT, PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer nextProm.group.freeGoroutine()
	runCallbackHandler[NextT, PrevT](nextProm, cb, nil, ctx, cancel)
	debug(nextProm, endHandler, endGroupHandler, endGroupGoHandler)
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if errProm := g.initValidateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](g)
	flags := getDelayFlags(cond)
	debug(nextProm, startHandler, startGroupHandler, startGroupDelayHandler)
	go delayHandler(nextProm, res, d, flags)
	return nextProm
}

func delayHandler[T any](
	nextProm *Promise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer nextProm.group.freeGoroutine()
	nextProm.resolveToResWithDelay(res, dd, flags)
	debug(nextProm, endHandler, endGroupHandler, endGroupDelayHandler)
}

func (g *Group[T]) Chan(resChan <-chan Result[T]) *Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	if errProm := g.initValidateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](g)
	debug(nextProm, startHandler, startGroupHandler, startGroupChanHandler)
	go chanHandler(nextProm, resChan)
	return nextProm
}

func chanHandler[T any](nextProm *Promise[T], resChan <-chan Result[T]) {
	defer nextProm.group.freeGoroutine()
	res := <-resChan
	nextProm.resolveToRes(res)
	debug(nextProm, endHandler, endGroupHandler, endGroupChanHandler)
}

func (g *Group[T]) Ctx(ctx context.Context) *Promise[T] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}

	g.init()
	if errRes := g.validateActive(); errRes != nil {
		return newPromSync[T](g, errRes)
	}

	// handle contexts with nil done channel, as a nil channel is never closed,
	// and if used with this package it will block follow calls on it forever.
	if ctx.Done() == nil {
		if g != nil && g.core.options.IsNoNilCtxDoneChan() {
			return newPromSync[T](g, errCtxNilDoneResult[T]{})
		}

		// since this ctx value will never be closed, the equivalent
		// outcome would be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded
		// resources.
		return newPromBlocked[T]()
	}

	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errGroupBusyResult[T]{})
	}

	nextProm := newPromCtx[T](g, ctx)
	debug(nextProm, startHandler, startGroupHandler, startGroupCtxHandler)
	go ctxHandler(nextProm)
	return nextProm
}

func ctxHandler[T any](nextProm *Promise[T]) {
	defer nextProm.group.freeGoroutine()
	nextProm.waitState()
	nextProm.resolveToRes(nextProm.res)
	debug(nextProm, endHandler, endGroupHandler, endGroupCtxHandler)
}

func (g *Group[T]) Wrap(res Result[T]) *Promise[T] {
	g.init()

	if errRes := g.validateActive(); errRes != nil {
		return newPromSync[T](g, errRes)
	}

	if res != nil && res.State() == Panic {
		if _, ok := res.Err().(resultPanicV); !ok {
			res = PanicRes[T](res)
		}
	}

	return newPromSync[T](g, res)
}

// Wait enters the wait mode and waits until all [Promise]s returns.
// If there are any triggers provided, it executes them first.
//
// Example:
//
//	g.Wait(func() { <-signalChan })
func (g *Group[T]) Wait(triggers ...func()) {
	g.init()
	// execute the trigger functions to block the wait logic until they return.
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true) // blocks new calls from being started.
	g.core.sg.Wait()
}

func (g *Group[T]) SelectRes(triggers ...func()) Result[GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.selectRes()
}

func (g *Group[T]) AllRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.joinRes(allOp)
}

// AllWaitRes behaves like the [AllWait] extension function, but only operates
// on the promises that belong to this [Group].
// It causes the [Group] to enter the wait mode, and wait for all ongoing promises
// to return, then examine their [Result] values.
// It returns the promises [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the Group option
// [GroupConfig.SaveAllGroupResults].
//
// The [Result] of this call will be a [Success] iff all the promises were
// resolved to [Success], or it's called on a zero [Group].
// It will be a [Panic] if at least one promise was resolved to [Panic].
// It will be an [Error] if at least one promise was resolved to [Error].
func (g *Group[T]) AllWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(allWaitOp)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) AnyRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.joinRes(anyOp)
}

func (g *Group[T]) AnyWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(anyWaitOp)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) JoinRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(joinOp)
	g.core.sg.Wait()
	return res
}

// groupCall describes a Group call and how to communicate back to it.
type groupCall[T any] struct {
	// resChan is used to send the Result back to the groupCall's goroutine.
	// this is a new, per call, unbuffered and never closed channel.
	resChan chan<- GroupRes[T]

	// syncChan is used to communicate that the groupCall has been resolved,
	// so that the sending promise(s) can return without blocking on resChan.
	// this is a new, per call, unbuffered channel.
	syncChan <-chan struct{}
}

type groupResHistory[T any] struct {
	// vals holds 1 [Result] of each [State], either the first or the last,
	// based on the [GroupConfig.SetSaveLastSingleGroupResult] flag.
	// the first most or last most [Result] will be saved at index 0.
	vals [3]GroupRes[T]
}

// the 'resMu' must be write-locked before entering.
func (h *groupResHistory[T]) insertRes(res GroupRes[T]) {
	// if we only save the first [Result], then save it in the first empty place.
	for i := range h.vals {
		// found an empty place?
		if h.vals[i].Result == nil {
			h.vals[i] = res
			break
		}

		// found another [Result] with the same [State]?
		if h.vals[i].State() == res.State() {
			break
		}
	}
}

// getRes returns the first most or last most [Result], according to how
// the values have been saved.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getRes() (res GroupRes[T]) {
	// the [Result] we're interested in is always at index 0.
	return h.vals[0]
}

// getStateHist returns the Bitwise-OR of all different states that have
// been sent on this group.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getStateHist() (state State) {
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		state |= h.vals[i].State()
	}
	return state
}

// getResForState returns the GroupRes for the provided [State], or nothing,
// if no matching GroupRes has been saved for the provided [State].
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getResForState(state State) (res GroupRes[T]) {
	// the call doesn't expect a result to be added from the history.
	if state == unknown {
		return res
	}

	// fetch the result from the history based on the state.
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		if h.vals[i].Result.State() == state {
			return h.vals[i]
		}
	}

	// no result is found for that call's state.
	return res
}

// groupCore contains read-only, share-able, or type-agnostic fields only.
type groupCore struct {
	debugCB func([]debugEvent)

	unhandledPanicCB func(any)
	unhandledErrorCB func(error)

	// waiting represents whether this group has entered the waiting
	// mode or not.
	// it can enter the waiting mode via one of the Wait methods.
	waiting atomic.Bool

	// canceled represents whether this Group has been canceled or not,
	// which happens if there's been an unhandled failure and
	// the [GroupConfig.FailuresCancelGroup] flag was set.
	canceled atomic.Bool

	// sg is used for limiting concurrency and implementing
	// the waiting functions.
	sg sema.Group

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it panics or returns an error.
	// if neverCancelCBCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc

	// options is generated from groupOptions.
	options groupOptionsBitFlags
}

//go:generate genflagged -type=groupOptions -outType=groupOptionsBitFlags -outFile=group_options_flagged.go
type groupOptions struct {
	onetimeHandling     bool
	neverCancelCBCtx    bool
	failuresCancelCBCtx bool
	failuresCancelGroup bool
	noNilCtxDoneChan    bool
	noWaitingBusyGroup  bool
	saveAllGroupResults bool
}

// validateActive will return an [Error] [Result] if the [Group] has been closed
// because of either it is in Waiting mode, or it has been Canceled due to a failure.
func (g *Group[T]) validateActive() Result[T] {
	if g == nil {
		return nil
	}

	if g.isWaiting() {
		return errGroupWaitingResult[T]{}
	}
	if g.isCanceled() {
		return errGroupCanceledResult[T]{}
	}

	return nil
}

// isWaiting should be only used through validateActive.
func (g *Group[T]) isWaiting() bool {
	return g.core.waiting.Load()
}

// isCanceled should be only used through validateActive.
func (g *Group[T]) isCanceled() bool {
	if !g.core.options.IsFailuresCancelGroup() {
		// since we only write to the canceled field if that option is set,
		// then there's no point in attempting to read it if not set.
		return false
	}
	return g.core.canceled.Load()
}

// note: once it returns true, an arrangement must be made for calling freeGoroutine.
func (g *Group[T]) reserveGoroutine(chainRegFunc func()) bool {
	if g == nil {
		// execute the register func and mark this reservation successful.
		chainRegFunc()
		return true
	}

	// either block until a place is available, or return an error with no waiting.
	if g.core.options.IsNoWaitingBusyGroup() {
		if g.core.sg.TryReserveN(1) {
			// since we entered this case, and this is a non-blocking select,
			// this case will happen immediately.
			chainRegFunc()
			return true
		}
		return false
	}

	// it's guaranteed to make a successful reservation in this flow (after waiting),
	// execute the register func before waiting.
	chainRegFunc()
	g.core.sg.Reserve()
	return true
}

// note: if the program is exiting, this call might not be executed,
// so no important clean up or logic should be included here.
func (g *Group[T]) freeGoroutine() {
	if g == nil {
		return
	}
	g.core.sg.Free()
}
