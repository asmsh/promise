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
	"time"
)

// panic messages
const (
	nilResChanPanicMsg  = "promise: the provided channel value is nil"
	nilCtxPanicMsg      = "promise: the provided Context value is nil"
	nilCallbackPanicMsg = "promise: the provided function value is nil"
)

type chainStatus = uint32

const (
	chainStatusEmpty chainStatus = iota
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

// readState checks if the promise p is resolved, and returns its [State],
// otherwise it returns [unknown].
func (p *Promise[T]) readState() State {
	// non-blocking read of res, which is closed by the previous
	// promise, once it's resolved.
	select {
	case <-p.syncCtx.Done():
		if p.res == nil {
			return Success
		}
		return p.res.State()
	default:
		return unknown
	}
}

// waitState waits for the promise p to be resolved, and returns its [State].
func (p *Promise[T]) waitState() State {
	// the Context will be closed by the previous promise,
	// after setting the res and status fields as expected.
	<-p.syncCtx.Done()
	if p.res == nil {
		return Success
	}
	return p.res.State()
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
	extChan <- extQueue[T]{initState: p.readState()}
	if p.extChanP.CompareAndSwap(nil, &extChan) {
		return extChan
	}
	extChanP = p.extChanP.Load()
	return *extChanP
}

func (p *Promise[T]) isOnetimePromise() bool {
	if p.group == nil {
		return false
	}
	return p.group.core.options.IsOnetimeHandling()
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
		p.resolveToSuccessRes(nil)
		return
	}

	// resolve the provided Promise to the provided Result, accordingly.
	// note: if res is a Promise value, the State call will block until that
	// Promise is resolved.
	switch s := res.State(); s {
	case Panic:
		p.resolveToPanicRes(res)
	case Error:
		p.resolveToErrorRes(res)
	case Success:
		p.resolveToSuccessRes(res)
	default:
		panic("promise: unexpected Result State: " + s.String())
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
		p.resolveToSuccessRes(nil)
		return
	}

	switch s := res.State(); s {
	case Panic:
		if flags.onPanic {
			time.Sleep(dd)
		}
		p.resolveToPanicRes(res)
	case Error:
		if flags.onError {
			time.Sleep(dd)
		}
		p.resolveToErrorRes(res)
	case Success:
		if flags.onSuccess {
			time.Sleep(dd)
		}
		p.resolveToSuccessRes(res)
	default:
		panic("promise: unexpected Result State: " + s.String())
	}
}

func (p *Promise[T]) resolveToPanicRes(
	res Result[T],
) {
	debug(p, resolve, resolvePanic)
	// save the result before executing any callbacks.
	p.res = res
	// execute the side effects, if the chain doesn't have a wait or read calls,
	// nor is already handled
	p.chainSideEffects(p.chainStatus.Load(), false)
	// unblock all calls waiting on p.
	p.syncCtx.(syncCtx).cancel()
	// handle all extension calls that involve p.
	handleExtCalls(p)
	// handle all group calls for p's group.
	handleGroupCalls(p)

	// note: any code that gets added after closeSyncCtx isn't guaranteed
	// to be executed without extra wait arrangements (via Group Wait methods,
	// or extension functions).
}

func (p *Promise[T]) resolveToErrorRes(
	res Result[T],
) {
	debug(p, resolve, resolveError)
	p.res = res
	p.chainSideEffects(p.chainStatus.Load(), false)
	p.syncCtx.(syncCtx).cancel()
	handleExtCalls(p)
	handleGroupCalls(p)
}

func (p *Promise[T]) resolveToSuccessRes(
	res Result[T],
) {
	debug(p, resolve, resolveSuccess)
	p.res = res
	// no side effects to be done for Success result.
	p.syncCtx.(syncCtx).cancel()
	handleExtCalls(p)
	handleGroupCalls(p)
}

// note: if there's a result to be set, it must be set before calling it.
func (p *Promise[T]) chainSideEffects(cs chainStatus, waitCall bool) {
	if p.res == nil {
		return
	}

	// if the chain is not empty (there's a follow, read or wait calls), return
	// early, unless it's a wait call and the waitCall logic is the caller.
	// as the unhandled logic will be delayed until the last call in the chain.
	// otherwise, execute the unhandled panic logic.
	switch {
	case cs == chainStatusHandled:
		return
	case cs == chainStatusRead:
		return
	case cs == chainStatusWait && !waitCall:
		return
	}

	// at this point, the promise is not handled and doesn't have a read call,
	// so call the unhandled handlers if the state is one of a failure.
	switch state := p.res.State(); state {
	case Panic:
		p.unhandledPanic()
	case Error:
		p.unhandledError()
	}
}

var (
	callbackOp followOperationLogic = callbackOperation{}
	followOp   followOperationLogic = followOperation{}
	thenOp     followOperationLogic = thenOperation{}
	catchOp    followOperationLogic = catchOperation{}
	recoverOp  followOperationLogic = recoverOperation{}
	finallyOp  followOperationLogic = finallyOperation{}
)

// followOperationLogic encapsulates the logic for each of the follow calls.
type followOperationLogic interface {
	// IsTargetState takes the current [State] value and returns whether
	// it's the target for this call, and we should resolve/return.
	IsTargetState(currState State) bool

	// CanHandle is an identity method telling whether the operation
	// can attempt to mark the target promise as handled or not.
	// this will trigger the [handleFollow] logic for that operation.
	CanHandle() bool

	// ResolveOnInvalidHandle is an identity method telling whether the
	// operation should resolve the next promise once handling the target
	// promise is invalid or not.
	// an invalid handling is communicated via the [handleFollow] method
	// returning valid=false.
	// the resolve will be taken care of by the call to [handleFollow].
	ResolveOnInvalidHandle() bool

	// ReturnOnInvalidHandle is an identity method telling whether the
	// operation should return once handling the target promise is invalid
	// or it should still move forward and run the callback.
	// an invalid handling is communicated via the [handleFollow] method
	// returning valid=false.
	ReturnOnInvalidHandle() bool

	SupportHandleReturns() bool
}

type callbackOperation struct{}

func (callbackOperation) IsTargetState(_ State) bool   { return true }
func (callbackOperation) CanHandle() bool              { return false }
func (callbackOperation) ResolveOnInvalidHandle() bool { return false }
func (callbackOperation) ReturnOnInvalidHandle() bool  { return false }
func (callbackOperation) SupportHandleReturns() bool   { return false }

type followOperation struct{}

func (followOperation) IsTargetState(_ State) bool   { return true }
func (followOperation) CanHandle() bool              { return true }
func (followOperation) ResolveOnInvalidHandle() bool { return true }
func (followOperation) ReturnOnInvalidHandle() bool  { return true }
func (followOperation) SupportHandleReturns() bool   { return true }

type thenOperation struct{}

func (thenOperation) IsTargetState(s State) bool   { return s == Success }
func (thenOperation) CanHandle() bool              { return true }
func (thenOperation) ResolveOnInvalidHandle() bool { return true }
func (thenOperation) ReturnOnInvalidHandle() bool  { return true }
func (thenOperation) SupportHandleReturns() bool   { return true }

type catchOperation struct{}

func (catchOperation) IsTargetState(s State) bool   { return s == Error }
func (catchOperation) CanHandle() bool              { return true }
func (catchOperation) ResolveOnInvalidHandle() bool { return false }
func (catchOperation) ReturnOnInvalidHandle() bool  { return false }
func (catchOperation) SupportHandleReturns() bool   { return true }

type recoverOperation struct{}

func (recoverOperation) IsTargetState(s State) bool   { return s == Panic }
func (recoverOperation) CanHandle() bool              { return true }
func (recoverOperation) ResolveOnInvalidHandle() bool { return true }
func (recoverOperation) ReturnOnInvalidHandle() bool  { return true }
func (recoverOperation) SupportHandleReturns() bool   { return true }

// finallyOperation can't set the 'Handled' flag(handle the promise),
// and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a Success promise,
// and the panic will be dismissed implicitly, which is something we don't want.
//
// Follow vs Finally: Finally doesn't mark the receiver Promise as handled,
// which make it useful for cleanup without changing the call flow that would
// trigger the Unhanded callbacks logic of a Group.
// However, Follow marks the receiver Promise as handled.
type finallyOperation struct{}

func (finallyOperation) IsTargetState(_ State) bool   { return true }
func (finallyOperation) CanHandle() bool              { return false }
func (finallyOperation) ResolveOnInvalidHandle() bool { return false }
func (finallyOperation) ReturnOnInvalidHandle() bool  { return false }
func (finallyOperation) SupportHandleReturns() bool   { return true }

func handleFollow[PrevT, NextT any](
	prevProm *Promise[PrevT],
	nextProm *Promise[NextT],
	resolveOnErr bool,
) (resToBeHandled Result[PrevT], valid bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or resolve to an erroneous one.
	validHandle := prevProm.setChainHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !prevProm.isOnetimePromise() {
		validHandle = true
	}

	// if the promise result has been used, either return or resolve with the expected error
	if !validHandle {
		if resolveOnErr {
			nextProm.resolveToErrorRes(errPromiseConsumedResult[NextT]{})
			return nil, false
		}
		return errPromiseConsumedResult[PrevT]{}, false
	}

	// the promise result can be accessed multiple times...
	// note: res might be nil if the previous promise was Success via a nil return.
	return prevProm.res, true
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
// TODO: pass a new value, panicked (similar to valid from the sync.OnceFunc implementation),
// and make the handleReturns function uses this value to tell whether the nil value is valid
// or not, which will be useful to tell whether runtime.Goexit is called, a panic with nil
// value occurred, or not.
func handleReturns[PrevT, NextT any](
	nextProm *Promise[NextT],
	prevRes Result[PrevT],
	nextResP *Result[NextT],
) {
	// get the new Result value based on the state of the callback
	var nextRes Result[NextT]
	if v := recover(); v != nil {
		// the callback panicked, create the appropriate Result value.
		nextRes = panicResult[NextT]{v: v}
	} else {
		// the callback returned normally, called runtime.Goexit, or
		// called panic with nil value.
		nextRes = getEffectiveNextRes(prevRes, nextResP)
	}

	// resolve the provided Promise to the new Result value.
	nextProm.resolveToRes(nextRes)
}

func getEffectiveNextRes[NextT, PrevT any](
	prevRes Result[PrevT],
	nextResP *Result[NextT],
) (effRes Result[NextT]) {
	// if a next Result is set, return it.
	// this happens for callbacks that support returning a new [Result].
	if nextResP != nil {
		return *nextResP
	}

	// if there was no previous [Result] provided, return the zero value
	// of the next [Result].
	// this happens for callbacks that aren't following a previous [Promise],
	// and doesn't support returning a new [Result], which is the [Go] callback.
	if prevRes == nil {
		return effRes
	}

	// no new result is set, and the previous Result is non-nil, so try
	// to cast the previous Result to the new Result's type...

	// first, try casting the whole prev Result instance...
	if nextRes, ok := any(prevRes).(Result[NextT]); ok {
		return nextRes
	}

	// otherwise, reconstruct the Result from the prev value...
	nextRes := result[NextT]{
		err:   prevRes.Err(),
		state: prevRes.State(),
	}

	// try to set the next Result's val...
	if nextResVal, ok := any(prevRes.Val()).(NextT); ok {
		// this will only fail if prevRes.Val is nil, or if it's not
		// of NextT type.
		//
		// TODO: when can it be not of NextT type?
		// this can't happen, as the go type system guarantees that
		// the PrevT of the previous [Promise] is assignable to NextT
		// of the next [Promise].
		// and since we only call this function when
		nextRes.val = nextResVal
	}

	return nextRes
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

	// get a snapshot of the group calls queue to be handled.
	p.group.callsQMu.RLock()
	for call := p.group.callsQ.Front(); call != nil; call = call.Next() {
		handled = handleGroupCall(call.Value, res) || handled
		debug(p, doneHandleGroupCall)
	}
	p.group.callsQMu.RUnlock()

	// save the group state and result for calls added later.
	p.group.resMu.Lock()
	groupRes := GroupRes[T]{Result: res}

	p.group.resHist.insertRes(groupRes)

	if p.group.core.options.IsSaveAllGroupResults() {
		p.group.resQ.PushBack(groupRes)
	}

	debug(p, doneSaveGroupResult)
	p.group.resMu.Unlock()

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

// newPromInter creates a new Promise which is resolved asynchronously,
// via an internal allocated channel.
func newPromInter[T any](g *Group[T]) *Promise[T] {
	return &Promise[T]{
		group:   g,
		syncCtx: newSyncCtx(),
	}
}

// newPromCtx creates a new Promise which is resolved asynchronously,
// via the channel from the passed context.
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
		res:     res,
		// no other fields are needed, since sync promises are resolved directly
		// after created, so any extension call will depend on the syncCtx chan.
	}
	p.chainSideEffects(chainStatusRead, false) // anything other than empty or handled.
	return p
}
