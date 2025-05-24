package promise

import (
	"container/list"
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
	// init the wait chan of this group.
	groupDone := g.core.sg.WaitChan()

	// check whether this is a sync call or an async call.
	// it will be a sync call iff the [Group] is already done,
	// which happens when the 'groupDone' is closed.
	// this will be used to not add the 'groupCall' to the 'callsQ',
	// and instead just read directly from the 'resQ'.
	select {
	case <-groupDone:
		// TODO: how to know which saved Result is the first, to return here,
		// like what we do with the AllRes, etc.
		// fast-path, with no unneeded allocations in 'selectResSlow'.
		// return a [Success] [GroupRes] with empty [Result].
		// this could happen either on a zero [Group], or when
		// the [Group] has no ongoing promises, and in both cases
		// the [Select] can't observe any [Result] values.
		return newSingleRes[T, GroupRes[T]](Success, GroupRes[T]{})
	default:
		return g.selectResSlow(groupDone)
	}
}

func (g *Group[T]) selectResSlow(groupDone <-chan struct{}) Result[GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])
	syncChan := make(chan struct{})

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// wait for a [Result] or for the [Group] to be done.
	var callResState State
	var callRes GroupRes[T]
	select {
	case <-groupDone:
	case res := <-resChan:
		callResState = res.State()
		callRes = res
	}

	// close the 'syncChan' to prevent other promises from blocking on
	// sending to the 'resChan', once the wanted [Result] is received.
	close(syncChan)

	// remove this groupCall from the callsQ, now that it's no longer active.
	// note: this is just a cleanup action, as this groupCall will not be used
	// by anything (there are no running promises to use anymore).
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	// handle not receiving any [Result], or calls on a zero [Group].
	// it could still happen that we don't receive any [Result] if we
	// propagated this 'groupCall' after the ongoing promises passed
	// the 'handleGroupCall' loop in the 'handleGroupCalls'.
	// so, in this case, it will be treated as a sync call.
	if callResState == unknown {
		return newSingleRes[T, GroupRes[T]](Success, GroupRes[T]{})
	}

	return newSingleRes(callResState, callRes)
}

func (g *Group[T]) AllRes(triggers ...func()) Result[[]GroupRes[T]] {
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
// It returns the promises [Result] values either from the moment of calling this
// method.
// The [Result] of this call will be resolved to [Success] iff all the promises
// were resolved to [Success].
// It will be resolved to [Panic] if at least one promise was resolved to [Panic].
// It will be resolved to [Error] if at least one promise was resolved to [Error].
//
// TODO: // It returns the promises [Result] values either from the moment of calling this
// // method, or from the start of this [Group], based on the Group option [GroupConfig.SaveAllGroupResults].
func (g *Group[T]) AllWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(allWaitOp)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) AnyRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	res := g.joinRes(anyOp)
	return res
}

func (g *Group[T]) AnyWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(anyWaitOp)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) JoinRes(triggers ...func()) Result[[]GroupRes[T]] {
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(joinOp)
	g.core.sg.Wait()
	return res
}

// checkTargetStateFunc is optional and only  needed if we want to return early.
func (g *Group[T]) joinRes(op joinOperationLogic) Result[[]GroupRes[T]] {
	// init the wait chan of this group.
	groupDone := g.core.sg.WaitChan()

	// check whether this is a sync call or an async call.
	// it will be a sync call iff the [Group] is already done,
	// which happens when the 'groupDone' is closed.
	// this will be used to not add the 'groupCall' to the 'callsQ',
	// and instead just read directly from the 'resQ'.
	select {
	case <-groupDone:
		// get the 'callResState' and 'callRes' from this [Group].
		g.core.resMu.RLock()
		// if the state history is 0, then this is a zero [Group],
		// so return a [Success] [GroupRes] with empty [Result].
		callResState := op.InitState(g.core.resStateHist)
		if callResState == unknown {
			callResState = calcGroupResState(g.core.resStateHist)
		}
		callRes := g.getCallResSnapshot(callResState, 0)
		g.core.resMu.RUnlock()

		return newMultiRes(callResState, callRes)
	default:
		// if we want to return early, check if the current [Group]
		// state is the target [State] for this call.
		// note: if the check failed, we will need to check the history
		// fields again, because the target [State] might be sent later
		// on the resQ.
		// note: the history fields set happens-before the resQ send,
		// so, to be sure that we didn't miss anything, we need to check
		// the history fields _after_ the resQ send.
		if op.ReturnOnTargetState() {
			var callResState State
			var callRes []GroupRes[T]
			var targetFound bool

			g.core.resMu.RLock()
			callResState = op.InitState(g.core.resStateHist)
			if op.IsTargetState(callResState) {
				// we found our target, so return the expected [Result] values.
				targetFound = true
				callRes = g.getCallResSnapshot(callResState, 0)
			}
			g.core.resMu.RUnlock()

			if targetFound {
				return newMultiRes(callResState, callRes)
			}

			// target is not found, move forward to joinResSlow...
		}

		return g.joinResSlow(groupDone, op)
	}
}

// getCallResSnapshot returns a copy of this [Group]'s results,
// at the time of calling, based on the 'saveAllGroupResults' flag.
//
// the callResState is the result of the respective 'calcInitStateFunc'
// call, given the [Group]'s state history, at the time of calling.
//
// callResLen is the capacity of the returned [GroupRes] slice that's
// requested by the caller.
// the actual capacity will be bigger than this value by a difference
// based on the 'saveAllGroupResults' flag.
// the difference will be the length of resQ if 'saveAllGroupResults'
// is true, and 1 otherwise.
//
// the resMu must be locked before entering.
func (g *Group[T]) getCallResSnapshot(callResState State, callResLen int) []GroupRes[T] {
	if g.core.saveAllGroupResults {
		callResLen += g.core.resQ.Len()
		callRes := make([]GroupRes[T], 0, callResLen)
		for res := g.core.resQ.Front(); res != nil; res = res.Next() {
			callRes = append(callRes, res.Value.(GroupRes[T]))
		}
		return callRes
	}

	// fetch the result from the history based on the callResState.
	var val any
	switch callResState {
	case Panic:
		if res := g.core.resHist.panic; res != nil {
			val = res
			callResLen += 1
		}
	case Error:
		if res := g.core.resHist.error; res != nil {
			val = res
			callResLen += 1
		}
	case Success:
		if res := g.core.resHist.success; res != nil {
			val = res
			callResLen += 1
		}
	default:
		// do nothing, as it means the call doesn't expect a result
		// to be added from the history.
	}

	// create the callRes with the final expected cap, only if there
	// are results expected, otherwise leave it nil.
	var callRes []GroupRes[T]
	if callResLen > 0 {
		callRes = make([]GroupRes[T], 0, callResLen)
	}

	// if we found a result that matches the group call, include it.
	if val != nil {
		callRes = append(callRes, val.(GroupRes[T]))
	}

	return callRes
}

// checkTargetStateFunc is optional and only needed if we want to return early.
func (g *Group[T]) joinResSlow(
	groupDone <-chan struct{},
	op joinOperationLogic,
) Result[[]GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])

	// only create the syncChan if we are expecting an early return.
	var syncChan chan struct{}
	if op.ReturnOnTargetState() {
		syncChan = make(chan struct{})
	}

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// init the result array, and account for the [Group] state only
	// if we are waiting for all promises.
	var callResState State
	var callRes []GroupRes[T]
	var targetFound bool
	if op.ReturnOnTargetState() {
		g.core.resMu.RLock()
		callResState = op.InitState(g.core.resStateHist)
		if op.IsTargetState(callResState) {
			// we found our target, so return the expected [Result] values.
			targetFound = true
			callRes = g.getCallResSnapshot(callResState, 0)
		} else {
			// expect at least all active promises to be included.
			callResLen := g.core.sg.ActiveCount()
			callRes = g.getCallResSnapshot(callResState, int(callResLen))
		}
		g.core.resMu.RUnlock()
	} else {
		// we want to wait for all promises, active and pending, so
		// init the result array with the expected results number.
		//
		// note: we account for ALL ongoing promises (active and pending),
		// because at this point, we propagated this 'groupCall' to all
		// ongoing promises, and since they block on either a sending
		// to this 'groupCall''s resChan, or receiving from its syncChan,
		// and as both operations will only unblock in the resultLoop
		// below, then all results will be seen by the resultLoop.
		//
		// note: to return the results of all promises that ever ran on
		// this [Group], done and ongoing, we start with a snapshot of
		// the 'resQ', which at this point, will include all results
		// that have been sent on this [Group] before and already
		// consumed by other 'groupCall', except this one.
		//
		// this is guaranteed by how the 'handleGroupCalls' is implemented,
		// which blocks once this 'groupCall' is propagated to all promises,
		// and inserts to the 'resQ' after this 'groupCall' unblocks them.
		// (see the note above for details).
		//
		// since the inserts to the 'resQ' are protected by a write lock
		// (Lock) on the resMu.
		// and the copy below is protected by a read lock (RLock);
		// we know that once we RLock the 'resMu', NO ongoing calls
		// will update the 'resQ' until this 'groupCall' unblocks in the
		// 'resultLoop' below.
		//
		// this also means that all further inserts to the 'resQ' will
		// also be sent to the 'resChan' of this 'groupCall'.
		//
		// after starting with the snapshot, we will move forward
		// to the 'resultLoop' below, which will unblock all ongoing
		// promises and append the got [Result] to the 'callRes'.
		// the 'callRes' now includes all results so far.

		// get the [State] of this [Group], and a snapshot of its [Result],
		// expecting at least all ongoing promises to be included.
		g.core.resMu.RLock()
		callResState = op.InitState(g.core.resStateHist)
		callResLen := g.core.sg.ActiveCount() + g.core.sg.PendingCount()
		callRes = g.getCallResSnapshot(callResState, int(callResLen))
		g.core.resMu.RUnlock()
	}

	// loop over the 'resChan', if the target [State] is not found yet,
	// recording each [Result], and updating the final [State] based on
	// the provided 'calcNextStateFunc'.
resultLoop:
	for !targetFound {
		// the 2 select cases below can't be ready at the same time.
		// because the 'groupDone' channel is closed once all Promise values
		// exits, which happens after the 'handleGroupCall' method returns,
		// and the 'handleGroupCall' method is responsible for sending on
		// the 'resChan'.
		select {
		case res := <-resChan:
			callRes = append(callRes, res)
			callResState = op.NextState(res.State(), callResState)
			if op.ReturnOnTargetState() && op.IsTargetState(res.State()) {
				break resultLoop
			}
		case <-groupDone:
			break resultLoop
		}
	}

	// if we are expected to return early, make sure no promises will block
	// on sending to the resChan.
	if op.ReturnOnTargetState() {
		close(syncChan)
	}

	// remove this 'groupCall' from the 'callsQ', now that it's no longer active.
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	// TODO: I don't think this can happen anymore, because now we set
	// the 'resStateHist' and the 'resHist' (or 'resQ') under the same
	// lock, and to enter the 'joinResSlow', we have to pass first by
	// the zero [Group] check.
	// so, at this point, this can't be a zero [Group], which means
	// that the 'resStateHist' must be already set.
	//
	// handle not receiving any [Result], or calls on a zero [Group].
	// it could still happen that we don't receive any [Result] if we
	// propagated this 'groupCall' after the ongoing promises passed
	// the 'handleGroupCall' loop in the 'handleGroupCalls'.
	// and in this case, the [Group] 'state' and 'resQ' fields will have
	// the up-to-date [Result] values.
	// so, in this case, it will be treated as a sync call.
	//
	// note: in this case, the 'callRes' will also be empty, because
	// we always set it after reading the 'callResState' from the [Group],
	// which can only be 'unknown' if this is a zero [Group], or it's
	// a no-waitAll call that received no [Result] values.
	if callResState == unknown {
		return newMultiRes[T, GroupRes[T]](Success, nil)
	}

	return newMultiRes(callResState, callRes)
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

type groupResHistory struct {
	panic   any // of GroupRes[T]
	error   any // of GroupRes[T]
	success any // of GroupRes[T]
}

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

	// callsQ is used for registering and tracking the [Group] Res calls,
	// like AnyRes, AllWaitRes, etc.
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
	//  the resStateHist value.
	// resMu protects reads and writes to them.
	resMu        sync.RWMutex
	resQ         list.List // of GroupRes[T]
	resStateHist State     // Bitwise OR of previous values
	resHist      groupResHistory

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it panics or returns an error.
	// if neverCancelCBCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc

	// flags for options...
	neverCancelCBCtx          bool
	onetimeHandling           bool
	noNilCtxDoneChan          bool
	noWaitingBusyGroup        bool
	saveAllGroupResults       bool
	saveLastSingleGroupResult bool
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
