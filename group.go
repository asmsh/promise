package promise

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

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
	// resStateHist holds the history of [State] values for all [Promise]
	// values that was run from this [Group].
	// resHist holds the history of [Result] values for either first values
	// or the last ones, depending on the [saveLastSingleGroupResult] flag.
	// TODO: merge [resStateHist] and [resHist] somehow.
	//  maybe by making resHist an array of 3 and check them to get
	//  the resStateHist value, and make saving the 3 result history
	//  mandatory for all calls.
	// resMu protects reads and writes to them.
	resMu        sync.RWMutex
	resQ         list.List // of GroupRes[T]
	resStateHist State     // Bitwise OR of previous values
	resHist      *groupResHistory[T]
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

func noopRegFunc() {
	// do nothing
}

func (g *Group[T]) Go(cb func()) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	g.init()
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := callbackCtx(g, p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoHandler)
	go goHandler(p, cb, ctx, cancel)
	return p
}

func goHandler[T any](
	p *Promise[T],
	cb GoFunc,
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, goFunc[T, T](cb), nil, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoHandler)
}

func (g *Group[T]) GoCtxRes(cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	g.init()
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := callbackCtx(g, p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoResHandler)
	go goCtxResHandler(p, cb, ctx, cancel)
	return p
}

func goCtxResHandler[T any](
	p *Promise[T],
	cb CtxResFunc[T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, ctxResFunc[T, T](cb), nil, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoResHandler)
}

func (g *Group[T]) GoCallback(cb Callback[T, T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	g.init()
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := callbackCtx(g, p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoResHandler)
	go goCallbackHandler(p, cb, ctx, cancel)
	return p
}

func goCallbackHandler[NextT, PrevT any](
	p *Promise[NextT],
	cb Callback[NextT, PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[NextT, PrevT](p, cb, nil, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoResHandler)
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	g.init()
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	flags := getDelayFlags(cond)
	debug(p, startHandler, startConstrHandler, startConstrDelayHandler)
	go delayHandler(p, res, d, flags)
	return p
}

func delayHandler[T any](
	p *Promise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer p.group.freeGoroutine()
	p.resolveToResWithDelay(res, dd, flags)
	debug(p, endHandler, endConstrHandler, endConstrDelayHandler)
}

func (g *Group[T]) Chan(resChan <-chan Result[T]) *Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}
	g.init()
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	debug(p, startHandler, startConstrHandler, startConstrChanHandler)
	go chanHandler(p, resChan)
	return p
}

func chanHandler[T any](p *Promise[T], resChan <-chan Result[T]) {
	defer p.group.freeGoroutine()
	res := <-resChan
	p.resolveToRes(res)
	debug(p, endHandler, endConstrHandler, endConstrChanHandler)
}

func (g *Group[T]) Ctx(ctx context.Context) *Promise[T] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}
	g.init()
	if ctx.Done() != nil {
		return newPromCtx[T](g, ctx)
	} else if g != nil && g.core.noNilCtxDoneChan {
		return newPromSync[T](g, errPromiseCtxNilDoneResult[T]{})
	}

	// since this ctx value will never be closed, the equivalent outcome would
	// be a Promise that's never resolved.
	// so, return that equivalent value without creating any unneeded resources.
	return newPromBlocked[T]()
}

func (g *Group[T]) Wrap(res Result[T]) *Promise[T] {
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
	res := g.joinRes(allOp)
	return res
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
	res := g.joinRes(anyOp)
	return res
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

// the 'resMu' must be locked before entering.
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
// the 'resMu' must be locked before entering.
func (h *groupResHistory[T]) getRes() (res GroupRes[T]) {
	// if it's not been initialized, return nothing.
	if h == nil {
		return res
	}

	// the [Result] we're interested in is always at index 0.
	return h.vals[0]
}

// getStateRes returns the GroupRes for the provided [State], or nothing,
// if no matching GroupRes has been saved for the provided [State].
//
// the 'resMu' must be locked before entering.
func (h *groupResHistory[T]) getStateRes(state State) (res GroupRes[T]) {
	// if it's not been initialized, return nothing.
	if h == nil {
		return res
	}

	// the call doesn't expect a result to be added from the history.
	if state == unknown {
		return res
	}

	// fetch the result from the history based on the state.
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		if h.vals[i].State() == state {
			return h.vals[i]
		}
	}

	// no result is found for that call's state.
	return res
}

// TODO: reach the below criteria.
// groupCore contains read-only, share-able, or type-agnostic fields only.
type groupCore struct {
	debugCB func([]debugEvent)

	unhandledPanicCB func(any)
	unhandledErrorCB func(error)

	// waiting represents whether this group has entered the waiting
	// mode or not.
	// it can enter the waiting mode via one of the Wait methods.
	waiting atomic.Bool

	// sg is used for limiting concurrency and implementing
	// the waiting functions.
	sg sema.Group

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it panics or returns an error.
	// if neverCancelCBCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc

	// flags for options...
	neverCancelCBCtx    bool
	onetimeHandling     bool
	noNilCtxDoneChan    bool
	noWaitingBusyGroup  bool
	saveAllGroupResults bool
}

func (g *Group[T]) isWaiting() bool {
	if g == nil {
		return false
	}
	return g.core.waiting.Load()
}

func (g *Group[T]) reserveGoroutine(chainRegFunc func()) bool {
	if g == nil {
		// execute the register func and mark this reservation successful.
		chainRegFunc()
		return true
	}

	// either block until a place is available, or return an error with no waiting.
	if g.core.noWaitingBusyGroup {
		if g.core.sg.TryReserve() {
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
