package promise

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Group[T any] struct {
	core groupCore
}

func NewGroup[T any](opts ...GroupOption) *Group[T] {
	g := &Group[T]{}
	for _, opt := range opts {
		opt(&g.core)
	}
	return g
}

func noopRegFunc() {
	// do nothing
}

func (g *Group[T]) Go(cb func()) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoHandler)
	go goHandler(p, cb, ctx, cancel)
	return p
}

func goHandler[T any](
	p *Promise[T],
	cb goCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, false, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoHandler)
}

func (g *Group[T]) GoErr(cb func() error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoErrHandler)
	go goErrHandler(p, cb, ctx, cancel)
	return p
}

func goErrHandler[T any](
	p *Promise[T],
	cb goErrCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoErrHandler)
}

func (g *Group[T]) GoRes(cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoResHandler)
	go goResHandler(p, cb, ctx, cancel)
	return p
}

func goResHandler[T any](
	p *Promise[T],
	cb goResCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoResHandler)
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
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
	// execute the trigger functions to block the wait logic until they return.
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true) // blocks new calls from being started.
	g.core.sg.Wait()
}

func (g *Group[T]) SelectRes(triggers ...func()) Result[GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	return g.selectRes()
}

func (g *Group[T]) selectRes() Result[GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])
	syncChan := make(chan struct{})

	// add the new call to the callsQ of calls for this group.
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// loop over the resChan, recording each Result, and update the final
	// state based on the provided stateFunc.
	res := <-resChan

	// close the syncChan to prevent other promises from waiting on sending
	// to the above resChan, once the wanted result is received.
	close(syncChan)

	// remove this groupCall from the callsQ, now that it's no longer active.
	// note: this is just a cleanup action, as this groupCall will not be used
	// by anything (there are no running promises to use anymore).
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	switch res.State() {
	case Panic:
		return panicResultSingleRes[T, GroupRes[T]]{res}
	case Error:
		return errorResultSingleRes[T, GroupRes[T]]{res}
	case Success:
		return successResultSingleRes[T, GroupRes[T]]{res}
	default:
		// an internal panic, because it's supposed to be caught earlier.
		panic("promise: internal: unexpected Result state: " + res.State().String())
	}
}

func (g *Group[T]) getAllResState() State {
	groupState := State(g.core.state.Load())

	// the order here matters.
	switch true {
	case groupState&Panic == Panic:
		return Panic
	case groupState&Error == Error:
		return Error
	case groupState&Success == Success:
		return Success
	}

	// unexpected state.
	return groupState
}

func (g *Group[T]) checkAllResState(state State) bool {
	// All breaks on [Error] only, as it ignores [Panic] and is okay with [Success].
	return state == Error
}

func (g *Group[T]) getAnyResState() State {
	groupState := State(g.core.state.Load())

	// the order here matters.
	switch true {
	case groupState&Panic == Panic:
		return Panic
	case groupState&Success == Success:
		return Success
	case groupState&Error == Error:
		return Error
	}

	// unexpected state.
	return groupState
}

func (g *Group[T]) checkAnyResState(state State) bool {
	// Any breaks on [Success] only, as it waits for the first [Success].
	return state == Success
}

func (g *Group[T]) getJoinResState() State {
	return Success
}

func (g *Group[T]) AllRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	res := g.joinRes(false, g.getAllResState, calcAllResState, g.checkAllResState)
	return res
}

// AllWaitRes behaves like the [AllWait] extension function, but only operates
// on the promises that belong to this [Group].
// It causes the [Group] to enter the wait mode, and wait for all ongoing promises
// to return, then examine their [Result] values.
// It returns the promises [Result] values either from the moment of calling this
// method.
// The [Result] of this call will be resolved to [Success] iff all the promises
// were resolved to [Success].
// It will be resolved to [Panic] if at least one promise was resolved to [Panic].
// It will be resolved to [Error] if at least one promise was resolved to [Error].
//
// TODO: // It returns the promises [Result] values either from the moment of calling this
// // method, or from the start of this [Group], based on the Group option [GroupConfig.RecordAllGroupResults].
func (g *Group[T]) AllWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(true, g.getAllResState, calcAllResState, nil)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) AnyRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	res := g.joinRes(false, g.getAnyResState, calcAnyResState, g.checkAnyResState)
	return res
}

func (g *Group[T]) AnyWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(true, g.getAnyResState, calcAnyResState, nil)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) JoinRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(true, g.getJoinResState, calcJoinResState, nil)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) joinRes(
	waitAll bool,
	readStateFunc func() State,
	calcStateFunc func(State, State) State,
	checkStateFunc func(State) bool, // optional. only needed if waitAll == true.
) Result[[]GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])

	// only create the syncChan if we are expecting an early return.
	var syncChan chan struct{}
	if !waitAll {
		syncChan = make(chan struct{})
	}

	// add the new call to the callsQ of calls for this group.
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// now that the call has been recorded, get the expected number
	// of results we will process, and initialize the result array.
	groupDone := g.core.sg.WaitChan()

	// init the result array, and account for the group state only
	// if we are waiting for all promises.
	var groupRes []GroupRes[T]
	var groupState State
	if !waitAll {
		// expect at least all ongoing promises to be included.
		groupLen := g.core.sg.ActiveCount()
		groupRes = make([]GroupRes[T], 0, groupLen)
	} else {
		// we are going to wait for all promises, ongoing and blocked,
		// so init the result array with the expected number.
		groupLen := g.core.sg.ActiveCount() + g.core.sg.PendingCount()
		groupRes = make([]GroupRes[T], 0, groupLen)

		// read the group state, which holds the history of the group.
		groupState = readStateFunc()
	}

	// loop over the resChan, recording each Result, and update the final
	// state based on the provided calcStateFunc.
resultLoop:
	for {
		// the 2 select cases below can't be ready at the same time.
		// because the groupDone channel is closed once all Promise values
		// exits, which happens after the 'handleGroupCall' method returns,
		// and the 'handleGroupCall' method is responsible for sending on
		// the 'resChan'.
		select {
		case res := <-resChan:
			groupRes = append(groupRes, res)
			groupState = calcStateFunc(res.State(), groupState)
			if !waitAll && checkStateFunc(res.State()) {
				break resultLoop
			}
		case <-groupDone:
			break resultLoop
		}
	}

	// if we are expected to return early, make sure no promises will block
	// on sending to the resChan.
	if !waitAll {
		close(syncChan)
	}

	// remove this groupCall from the callsQ, now that it's no longer active.
	// note: this is just a cleanup action, as this groupCall will not be used
	// by anything (there are no running promises to use anymore).
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	switch groupState {
	case Panic:
		return panicResultMultiRes[T, GroupRes[T]]{groupRes}
	case Error:
		return errorResultMultiRes[T, GroupRes[T]]{groupRes}
	case Success:
		return successResultMultiRes[T, GroupRes[T]]{groupRes}
	default:
		// an internal panic, because it's supposed to be caught earlier.
		panic("promise: internal: unexpected Result state: " + groupState.String())
	}
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

type groupCore struct {
	debugCB func([]debugEvent)

	unhandledPanicCB func(any)
	unhandledErrorCB func(error)

	// waiting represents whether this group has entered the waiting
	// mode or not.
	// it can enter the waiting mode via one of the Wait methods.
	waiting atomic.Bool

	// state is the [Group]'s [State] value, which holds the value
	// for all [Promise] values that was run from this [Group].
	state atomic.Uint32

	// sg is used for limiting concurrency and implementing
	// the waiting functions.
	sg sema.Group

	callsQMu sync.RWMutex
	callsQ   list.List // of groupCall[T]

	// flags for options...
	neverCancelCBCtx   bool
	onetimeHandling    bool
	noNilCtxDoneChan   bool
	noWaitingBusyGroup bool

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it panics or returns an error.
	// if neverCancelCBCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc
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

func noopCancelFunc() {
	// do nothing
}

// callbackCtx returns the effective Context for a callback, and its CancelFunc,
// if one is required, given the promise's syncCtx value.
// syncCtx should be a non-closed Context, or nil.
func (g *Group[T]) callbackCtx(syncCtx context.Context) (context.Context, context.CancelFunc) {
	// default scenario, either no Group or a Group with default behavior.
	// we return the syncCtx with no cancellation, if one is provided,
	// otherwise we return Background with cancellation.
	if g == nil || (g.core.ctx == nil && !g.core.neverCancelCBCtx) {
		if syncCtx == nil {
			return newSyncCtx(), nil
		}
		return syncCtx, noopCancelFunc
	}

	// there's a Group, if it's requested to never cancel callback Context,
	// then we return early with Background and no cancellation.
	if g.core.neverCancelCBCtx {
		return context.Background(), noopCancelFunc
	}

	// there's a Group with a group Context, so create the Context to be returned,
	// and arrange to close it when the promise's syncCtx is closed, if provided.
	if syncCtx == nil {
		return context.WithCancel(g.core.ctx)
	}

	// TODO: these 2 context calls can be replaced by a JoinContext that will be
	//  cancelled when any of them is cancelled.
	ctx, cancel := context.WithCancel(g.core.ctx)
	context.AfterFunc(syncCtx, cancel)
	return ctx, cancel
}
