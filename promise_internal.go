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
	case <-p.WaitChan():
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
	<-p.WaitChan()
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
	return p.extsChanSlow()
}

func (p *Promise[T]) extsChanSlow() chan extQueue[T] {
	// create the potential new extChan and attempt to set it.
	// note: we only init the extQueue once we win the CAS, to make sure
	// the readState returns the State of the promise at the time the in-use
	// extChan is created.
	// this is guaranteed as after the CAS, the select in the ext call will
	// block on either a receive that will happen once the readState returns
	// with the current State and the extQueue is initiated.
	// or, once the waitChan is closed.
	// and, if the send below was instead received by the promise's handleExtCalls,
	// then it means that the waitChan is already closed, so the ext call will
	// unblock nonetheless.
	extChan := make(chan extQueue[T], 1)
	if p.extChanP.CompareAndSwap(nil, &extChan) {
		extChan <- extQueue[T]{initState: p.readState()}
		return extChan
	}
	extChanP := p.extChanP.Load()
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

	// block new calls from being started, if it's requested to cancel
	// the group on unhandled failures.
	if p.group.core.options.IsFailuresCancelGroup() {
		p.group.core.canceled.Store(true)
	}

	// cancel the group's context, if it's request to cancel all
	// the promises of the group on unhandled failures.
	if p.group.core.cancel != nil {
		p.group.core.cancel()
	}

	// call the callback if one is provided.
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

	if p.group.core.options.IsFailuresCancelGroup() {
		p.group.core.canceled.Store(true)
	}

	if p.group.core.cancel != nil {
		p.group.core.cancel()
	}

	if p.group.core.unhandledErrorCB != nil {
		debug(p, callUnhandledErrorCallback)
		p.group.core.unhandledErrorCB(p.res.Err())
	}
}

// note: if there's a result to be set, it must be set before calling it.
func (p *Promise[T]) chainSideEffects(waitCall bool) {
	// Note: if p.res is itself a Promise, the State call will resolve it,
	// hence it will be blocking.
	if p.res == nil || p.res.State() == Success {
		return
	}
	p.chainSideEffectsSlow(p.chainStatus.Load(), waitCall)
}

func (p *Promise[T]) chainSideEffectsSlow(cs chainStatus, waitCall bool) {
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

func (p *Promise[T]) resolveToRes(res Result[T]) {
	debug(p, resolve)

	// save the result before executing any callbacks.
	p.res = res
	// execute the side effects, if the chain doesn't have a wait or read calls,
	// nor is already handled
	p.chainSideEffects(false)
	// unblock all calls waiting on p.
	if p.syncChan != nil {
		close(p.syncChan)
	}
	// handle all extension calls that involve p.
	handleExtCalls(p)
	// handle all group calls for p's group.
	handleGroupCalls(p)

	// note: any code that gets added after close(p.syncChan) isn't guaranteed
	// to be executed without extra wait arrangements (via Group Wait methods,
	// or extension functions).
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
	// IsTargetState takes the previous [Promise] [State] value and returns
	// whether the current follow operation is allowed on it, or we should
	// resolve to an error.
	IsTargetState(prevState State) bool

	// CanHandle is an identity method telling whether the operation
	// can attempt to mark the target promise as handled or not.
	//
	// this will trigger the [handleFollow] logic for that operation,
	// which depends on both previous and next promises.
	CanHandle() bool

	// ResolveOnInvalidHandle is an identity method telling whether the
	// operation should resolve the next promise once handling the target
	// is invalid or it should still move forward and run the callback.
	//
	// an invalid handling is communicated via the [handleFollow] method
	// returning valid=false.
	// the resolve will be taken care of by the call to [handleFollow].
	//
	// this is only relevant if CanHandle() returns true.
	ResolveOnInvalidHandle() bool
}

type callbackOperation struct{}

func (callbackOperation) IsTargetState(_ State) bool   { return true }
func (callbackOperation) CanHandle() bool              { return false }
func (callbackOperation) ResolveOnInvalidHandle() bool { return false }

type followOperation struct{}

func (followOperation) IsTargetState(_ State) bool   { return true }
func (followOperation) CanHandle() bool              { return true }
func (followOperation) ResolveOnInvalidHandle() bool { return true }

type thenOperation struct{}

func (thenOperation) IsTargetState(s State) bool   { return s == Success }
func (thenOperation) CanHandle() bool              { return true }
func (thenOperation) ResolveOnInvalidHandle() bool { return true }

type catchOperation struct{}

func (catchOperation) IsTargetState(s State) bool   { return s == Error }
func (catchOperation) CanHandle() bool              { return true }
func (catchOperation) ResolveOnInvalidHandle() bool { return false }

type recoverOperation struct{}

func (recoverOperation) IsTargetState(s State) bool   { return s == Panic }
func (recoverOperation) CanHandle() bool              { return true }
func (recoverOperation) ResolveOnInvalidHandle() bool { return true }

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
			nextProm.resolveToRes(errPromiseConsumedResult[NextT]{})
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
func handleReturns[PrevT, NextT any](
	nextProm *Promise[NextT],
	prevRes Result[PrevT],
	nextResP *Result[NextT],
	validNextResP *bool,
) {
	// get the new Result value based on the state of the callback.
	var nextRes Result[NextT]
	if v := recover(); v != nil {
		// the callback panicked, create the appropriate Result value.
		nextRes = panicResult[NextT]{v: v}
	} else if !*validNextResP {
		// it's not a valid result, and there was no panic, so
		// the caller must have called runtime.Goexit.
		nextRes = errPromiseGoexitResult[NextT]{}
	} else if nextResP != nil {
		// the callback returned normally.
		// if a next Result is set, return it.
		// this happens for callbacks that support returning a new [Result].
		nextRes = *nextResP
	} else {
		nextRes = getEffectiveNextRes[NextT](prevRes)
	}

	// resolve the provided Promise to the new Result value.
	nextProm.resolveToRes(nextRes)
}

func getEffectiveNextRes[NextT, PrevT any](
	prevRes Result[PrevT],
) (effRes Result[NextT]) {
	// maintain the nil Result, for calls on nil Result.
	if prevRes == nil {
		return nil
	}

	// otherwise, the previous Result must be of type Result[NextT].
	//
	// NOTE: this is safe because the only way for the previous promise
	// to have a different type is if it was a [Follow] or [FollowCallback]
	// call, and both have their IsTargetState returning true, which means
	// they will never reach this function.
	return any(prevRes).(Result[NextT])
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

	// handle having a single extension call
	handled = handleExtCall(extQ.call, p.res)

	// handle having multiple extension calls
	for _, call := range extQ.extra {
		handled = handleExtCall(call, p.res) || handled
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

	// get a snapshot of the group calls queue to be handled.
	p.group.callsQMu.RLock()
	for call := p.group.callsQ.Front(); call != nil; call = call.Next() {
		handled = handleGroupCall(call.Value, p.res) || handled
		debug(p, doneHandleGroupCall)
	}
	p.group.callsQMu.RUnlock()

	// save the group state and result for calls added later.
	p.group.resMu.Lock()
	groupRes := GroupRes[T]{Result: p.res}

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

// newPromAsync creates a new Promise which is resolved asynchronously,
// via an internal allocated channel.
func newPromAsync[T any](g *Group[T]) *Promise[T] {
	return &Promise[T]{
		group:    g,
		syncChan: make(chan struct{}),
	}
}

// newPromCtx creates a new Promise which is resolved asynchronously,
// via the channel from the passed context.
func newPromCtx[T any](g *Group[T], ctx context.Context) *Promise[T] {
	return &Promise[T]{
		group:    g,
		syncChan: nil, // [Promise.WaitChan] will access it from [Promise.res].
		res:      ctxResult[T]{ctx: ctx},
	}
}

// newPromSync returns a new Promise which is resolved synchronously and
// immediately to the provided [Result] value.
func newPromSync[T any](g *Group[T], res Result[T]) *Promise[T] {
	p := &Promise[T]{
		group:    g,
		syncChan: alreadyClosedSyncChan,
		res:      res,
		// no other fields are needed, since sync promises are resolved directly
		// after created, so any extension call will depend on the syncChan chan.
	}
	p.chainSideEffectsSlow(chainStatusRead, false) // anything other than empty or handled.
	return p
}

// newPromBlocked returns a promise that will never be resolved.
// it's used for promises for Ctx calls with nil context.Context.Done value,
// or for follow calls on such promises.
func newPromBlocked[T any]() *Promise[T] {
	return &Promise[T]{
		syncChan: neverClosedSyncChan,
		// no other fields need to be initialized,
		// since this promise will never be resolved.
	}
}
