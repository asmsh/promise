// Copyright 2023 Ahmad Sameh(asmsh)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promise

import (
	"context"
	"errors"
	"time"
)

// panic messages
const (
	nilResChanPanicMsg  = "promise: the provided channel value is nil"
	nilCtxPanicMsg      = "promise: the provided Context value is nil"
	nilCallbackPanicMsg = "promise: the provided function value is nil"
)

const (
	chainStatusEmpty = iota
	chainStatusWait
	chainStatusRead
	chainStatusHandled
)

func (p *Promise[T]) regChainWait() {
	// this will do nothing if the chain was already wait, read or handled.
	p.chainStatus.CompareAndSwap(chainStatusEmpty, chainStatusWait)
}

func (p *Promise[T]) regChainRead() {
	// fast-path, in case we are trying to do a redundant CAS, returns early.
	if p.chainStatus.Load() >= chainStatusRead {
		return
	}

	// this will first try to swap the read value with the empty value,
	// otherwise, will assume the current value is wait and try to swap it.
	// this will do nothing if the chain was already read or handled.
	if !p.chainStatus.CompareAndSwap(chainStatusEmpty, chainStatusRead) {
		p.chainStatus.CompareAndSwap(chainStatusWait, chainStatusRead)
	}
}

func (p *Promise[T]) setChainHandled() (validHandle bool) {
	// this will return true, only if the chain wasn't already handled.
	oldChain := p.chainStatus.Swap(chainStatusHandled)
	return oldChain != chainStatusHandled
}

func (p *Promise[T]) extsChan() chan extQueue[T] {
	extChanP := p.extChanP.Load()
	if extChanP != nil {
		return *extChanP
	}
	// initiate the callsQ value, even though the chan might not be used after all,
	// because if we initiated it after the CAS below, it might be available to some
	// call and make that call block until we initiate it.
	extChan := make(chan extQueue[T], 1)
	extChan <- extQueue[T]{
		initState: State(p.resolveState.Load()),
	}
	if p.extChanP.CompareAndSwap(nil, &extChan) {
		return extChan
	}
	extChanP = p.extChanP.Load()
	return *extChanP
}

// wait waits for the promise p to be resolved, by either blocking on receiving
// from the syncChan, or by using the resolveState value of the promise.
func (p *Promise[T]) wait() State {
	// if the resolve status is non-zero, then no need to wait, as
	// this is guaranteed to happen before closing the sync channel.
	s := p.resolveState.Load()
	if s != 0 {
		return State(s)
	}

	// wait until the promise is resolved.
	// the Context will be closed by the previous promise,
	// after setting the res and status fields as expected.
	<-p.syncCtx.Done()

	s = p.resolveState.Load()
	return State(s)
}

func (p *Promise[T]) isOnetimePromise() bool {
	if p.group == nil {
		return false
	}
	return p.group.core.onetimeHandling
}

func (p *Promise[T]) unhandledPanic() {
	if p.group == nil {
		return
	}

	debug(p, startUnhandledPanicLogic)

	// if it's request to cancel all the promises of the group on
	// uncaught panics, call the cancel function before the handler.
	if p.group.core.cancel != nil {
		p.group.core.cancel()
	}

	// call the callback if one is provided
	if p.group.core.unhandledPanicCB != nil {
		debug(p, callUnhandledPanicCallback)
		p.group.core.unhandledPanicCB(getPanicVFromRes(p.res))
	}
}

func (p *Promise[T]) unhandledError() {
	if p.group == nil {
		return
	}

	debug(p, startUnhandledErrorLogic)

	// if it's request to cancel all the promises of the group on
	// uncaught errors, call the cancel function before the handler.
	if p.group.core.cancel != nil {
		p.group.core.cancel()
	}

	// call the callback if one is provided
	if p.group.core.unhandledErrorCB != nil {
		debug(p, callUnhandledErrorCallback)
		p.group.core.unhandledErrorCB(p.res.Err())
	}
}

func (p *Promise[T]) resolveToRes(res Result[T]) {
	// res will be nil after a Finally callback on a Promise with nil Result,
	// after a callback that doesn't support returning Result, or when it's
	// explicitly returned from a callback that supports returning Result.
	if res == nil {
		p.resolveToFulfilledRes(nil)
		return
	}

	// resolve the provided Promise to the provided Result, accordingly.
	// Note: if res is a Promise value, the State call will block until that
	// Promise is resolved.
	switch s := res.State(); s {
	case Panicked:
		p.resolveToPanickedRes(res)
	case Rejected:
		p.resolveToRejectedRes(res)
	case Fulfilled:
		p.resolveToFulfilledRes(res)
	default:
		panic("promise: unexpected Result state: " + s.String())
	}
}

func (p *Promise[T]) resolveToResWithDelay(
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	if res == nil {
		if flags.onSuccess {
			time.Sleep(dd)
		}
		p.resolveToFulfilledRes(nil)
		return
	}

	switch s := res.State(); s {
	case Panicked:
		if flags.onPanic {
			time.Sleep(dd)
		}
		p.resolveToPanickedRes(res)
	case Rejected:
		if flags.onError {
			time.Sleep(dd)
		}
		p.resolveToRejectedRes(res)
	case Fulfilled:
		if flags.onSuccess {
			time.Sleep(dd)
		}
		p.resolveToFulfilledRes(res)
	default:
		panic("promise: unexpected Result state: " + s.String())
	}
}

func (p *Promise[T]) resolveToPanickedRes(
	res Result[T],
) {
	debug(p, resolve, resolvePanicked)
	// save the result before executing any callbacks.
	p.res = res
	if p.chainStatus.Load() == chainStatusEmpty {
		// if the chain is empty (no follow, read or wait calls), execute
		// the unhandled panic logic.
		// otherwise, it will be delayed until the last call in the chain.
		p.unhandledPanic()
	}
	// resolve with the Panicked status.
	p.resolveState.Store(uint32(Panicked))
	closeSyncCtx(p.syncCtx) // unblocks all calls waiting on p.
	handleExtCalls(p)       // handles all extension calls that involve p.
	handleGroupCalls(p)     // handles all group calls which p is part of.

	// note: any code that gets added after closeSyncCtx isn't guaranteed
	// to be executed without extra wait arrangements (via Group Wait methods,
	// or extension functions).
}

func (p *Promise[T]) resolveToRejectedRes(
	res Result[T],
) {
	debug(p, resolve, resolveRejected)
	p.res = res
	if p.chainStatus.Load() == chainStatusEmpty {
		p.unhandledError()
	}
	p.resolveState.Store(uint32(Rejected))
	closeSyncCtx(p.syncCtx)
	handleExtCalls(p)
	handleGroupCalls(p)
}

func (p *Promise[T]) resolveToFulfilledRes(
	res Result[T],
) {
	debug(p, resolve, resolveFulfilled)
	p.res = res
	p.resolveState.Store(uint32(Fulfilled))
	closeSyncCtx(p.syncCtx)
	handleExtCalls(p)
	handleGroupCalls(p)
}

func (p *Promise[T]) resolveToResSync(res Result[T]) {
	if res == nil {
		p.resolveState.Store(uint32(Fulfilled))
		return
	}

	p.res = res
	state := res.State()
	switch state {
	case Panicked:
		p.unhandledPanic()
	case Rejected:
		p.unhandledError()
	case Fulfilled:
		// nothing special to be done.
	default:
		panic("promise: unexpected Result state: " + state.String())
	}
	p.resolveState.Store(uint32(state))
}

func handleFollow[PrevT, NextT any](
	prevProm *Promise[PrevT],
	nextProm *Promise[NextT],
	resolveOnErr bool,
) (resToBeHandled Result[PrevT], valid bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or reject with an erroneous one.
	validHandle := prevProm.setChainHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !prevProm.isOnetimePromise() {
		validHandle = true
	}

	// if the promise result has been used, either return or resolve with the expected error
	if !validHandle {
		if resolveOnErr {
			nextProm.resolveToRejectedRes(errPromiseConsumedResult[NextT]{})
			return nil, false
		}
		return errPromiseConsumedResult[PrevT]{}, false
	}

	// the promise result can be accessed multiple times...
	// Note: res might be nil if the previous promise was fulfilled via a nil return.
	return prevProm.res, true
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
// TODO: pass a new value, panicked (similar to valid from the sync.OnceFunc implementation),
// and make the handleReturns function uses this value to tell whether the nil value is valid or not.
func handleReturns[PrevResT, NewResT any](
	p *Promise[NewResT],
	prevRes Result[PrevResT],
	newResP *Result[NewResT],
) {
	// get the new Result value based on the state of the callback
	var newRes Result[NewResT]
	if v := recover(); v != nil {
		// the callback panicked, create the appropriate Result value.
		newRes = panicResult[NewResT]{v: v}
	} else {
		// the callback returned normally, called runtime.Goexit, or
		// called panic with nil value.
		newRes = getEffectiveNewRes(prevRes, newResP)
	}

	// resolve the provided Promise to the new Result value.
	p.resolveToRes(newRes)
}

func getEffectiveNewRes[PrevResT, NewResT any](
	prevRes Result[PrevResT],
	newResP *Result[NewResT],
) (effRes Result[NewResT]) {
	// if a new Result is set, return it.
	if newResP != nil {
		return *newResP
	}

	// if there was no previous Result provided, return the zero value
	// of the new Result.
	if prevRes == nil {
		return effRes
	}

	// no new result is set, and the previous Result is non-nil, so try
	// to cast the previous Result to the new Result's type...

	// handle having the previous Result value as nil.
	anyPrevRes := any(prevRes.Val())
	if anyPrevRes == nil {
		return result[NewResT]{
			err:   prevRes.Err(),
			state: prevRes.State(),
		}
	}

	// handle having the previous Result's type compatible with the new one.
	prevResVal, ok := anyPrevRes.(NewResT)
	if ok {
		return result[NewResT]{
			val:   prevResVal,
			err:   prevRes.Err(),
			state: prevRes.State(),
		}
	}

	// TODO: this can't happen in the current implementation,
	//  as all type parameters used so far is the same type.
	return ErrRes[NewResT](errors.New("TODO: unexpected"))
}

func handleExtCalls[T any](p *Promise[T]) (handled bool) {
	debug(p, startHandleExtCalls)
	extChanP := p.extChanP.Load()
	if extChanP == nil {
		debug(p, missingExtChan, endHandleExtCalls)
		return false
	}
	debug(p, foundExtChan)
	extQ := <-*extChanP
	debug(p, foundExtQueue)

	// handle not having any extension calls
	if !extQ.valid {
		debug(p, emptyExtQueue, endHandleExtCalls)
		return false
	}

	// get the final and ready-to-use result
	res := getFinalRes(p.res)

	// handle having a single extension call
	handled = handleExtCall(extQ.call, res)

	// handle having multiple extension calls
	for _, call := range extQ.extra {
		handled = handleExtCall(call, res) || handled
		debug(p, doneHandleExtCall)
	}

	debug(p, endHandleExtCalls)
	return handled
}

func handleExtCall[T any](call extCall[T], res Result[T]) bool {
	select {
	case call.resChan <- IdxRes[T]{
		Idx:    call.idx,
		Result: res,
	}:
		return true
	case <-call.syncChan:
		return false
	}
}

func handleGroupCalls[T any](p *Promise[T]) (handled bool) {
	if p.group == nil {
		return false
	}

	debug(p, startHandleGroupCalls) // debug starts here, since no Group == no debug.

	res := getFinalRes(p.res)

	// record group state for calls added later.
	p.group.core.state.Or(uint32(res.State()))

	// get a snapshot of the group calls queue to be handled.
	p.group.core.callsQMu.RLock()
	for call := p.group.core.callsQ.Front(); call != nil; call = call.Next() {
		handled = handleGroupCall(call.Value, res) || handled
		debug(p, doneHandleGroupCall)
	}
	p.group.core.callsQMu.RUnlock()

	debug(p, endHandleGroupCalls)
	return handled
}

func handleGroupCall[T any](callVal any, res Result[T]) bool {
	call := callVal.(groupCall[T])
	select {
	case call.resChan <- GroupRes[T]{
		Result: res,
	}:
		return true
	case <-call.syncChan:
		return false
	}
}

// newPromInter creates a new Promise which is resolved internally,
// using an internal allocated channel.
func newPromInter[T any](g *Group[T]) *Promise[T] {
	return &Promise[T]{
		group:   g,
		syncCtx: newSyncCtx(),
	}
}

func newPromCtx[T any](g *Group[T], ctx context.Context) *Promise[T] {
	return &Promise[T]{
		group:   g,
		syncCtx: ctx,
		res:     ctxResult[T]{ctx: ctx},
	}
}

// newPromBlocked returns a promise that will never be resolved.
// it's used for promises for Ctx calls with empty context.Context value,
// or for follow calls on such promises.
func newPromBlocked[T any]() *Promise[T] {
	return &Promise[T]{
		syncCtx: neverClosedSyncCtx,
		// no other fields need to be initialized, since this promise will never be resolved.
	}
}

// newPromSync returns a new Promise which is resolved synchronously and
// immediately to the provided [Result] value.
func newPromSync[T any](g *Group[T], res Result[T]) *Promise[T] {
	p := &Promise[T]{
		group:   g,
		syncCtx: closedSyncCtx,
		// no other fields are needed, since sync promises are resolved directly
		// after created, so any extension call will depend on the syncChan.
	}
	p.resolveToResSync(res)
	return p
}
