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
	"errors"
	"fmt"
	"time"
)

// panic messages
const (
	nilCallbackPanicMsg = "promise: the provided callback is nil"
	nilResChanPanicMsg  = "promise: the provided resChan is nil"
)

const (
	chainStatusEmpty = iota
	chainStatusWait
	chainStatusRead
	chainStatusHandled
)

func (p *genericPromise[T]) regChainWait() {
	// this will do nothing if the chain was already wait, read or handled.
	p.chainStatus.CompareAndSwap(chainStatusEmpty, chainStatusWait)
}

func (p *genericPromise[T]) regChainRead() {
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

func (p *genericPromise[T]) setChainHandled() (validHandle bool) {
	// this will return true, only if the chain wasn't already handled.
	oldChain := p.chainStatus.Swap(chainStatusHandled)
	return oldChain != chainStatusHandled
}

// wait waits for the promise p to be resolved, by either blocking on receiving
// from the syncChan, or by using the resolveState value of the promise.
func (p *genericPromise[T]) wait() State {
	// if the resolve status is non-zero, then no need to wait, as
	// this is guaranteed to happen before closing the sync channel.
	s := p.resolveState.Load()
	if s != 0 {
		return State(s)
	}

	// wait until the promise is resolved.
	// the chan will always be closed by the previous promise,
	// after setting the res and status fields as expected.
	<-p.syncChan

	s = p.resolveState.Load()
	return State(s)
}

func handleFollow[PrevT, NextT any](
	prevProm *genericPromise[PrevT],
	nextProm *genericPromise[NextT],
	resolveOnErr bool,
) (resToBeHandled Result[PrevT], valid bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or reject with an erroneous one.
	validHandle := prevProm.setChainHandled()

	// TODO: support one-time promise, based on the pipeline options.
	// if the promise isn't a one-time promise, all handle calls will be valid
	validHandle = true

	// if the promise result has been used, either return or resolve with the expected error
	if !validHandle {
		if resolveOnErr {
			resolveToRejectedRes[NextT](nextProm, errPromiseConsumedResult[NextT]{})
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
	p *genericPromise[NewResT],
	prevRes Result[PrevResT],
	newResP *Result[NewResT],
) {
	// get the new Result value based on the state of the callback
	var newRes Result[NewResT]
	if v := recover(); v != nil {
		// the callback panicked, create the appropriate Result value.
		newRes = promisePanickedResult[NewResT]{v: v}
	} else {
		// the callback returned normally, called runtime.Goexit, or
		// called panic with nil value.
		newRes = getEffectiveNewRes(prevRes, newResP)
	}

	// resolve the provided Promise to the new Result value.
	resolveToRes[NewResT](p, newRes)
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
	return Err[NewResT](errors.New("TODO: unexpected"))
}

func resolveToRes[T any](p *genericPromise[T], res Result[T]) {
	// res will be nil after a Finally callback on a Promise with nil Result,
	// after a callback that doesn't support returning Result, or when it's
	// explicitly returned from a callback that supports returning Result.
	if res == nil {
		resolveToFulfilledRes(p, nil)
		return
	}

	// resolve the provided Promise to the provided Result, accordingly.
	// Note: if res is a Promise value, the State call will block until that
	// Promise is resolved.
	switch s := res.State(); s {
	case Panicked:
		resolveToPanickedRes(p, res)
	case Rejected:
		resolveToRejectedRes(p, res)
	case Fulfilled:
		resolveToFulfilledRes(p, res)
	default:
		panic(fmt.Sprintf("promise: internal: unexpected Result state: '%s'", s))
	}
}

func resolveToResWithDelay[T any](
	p *genericPromise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	if res == nil {
		if flags.onSuccess {
			time.Sleep(dd)
		}
		resolveToFulfilledRes(p, nil)
		return
	}

	switch s := res.State(); s {
	case Panicked:
		// a rejected state is considered a failure
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToPanickedRes(p, res)
	case Rejected:
		// a rejected state is considered a failure
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToRejectedRes(p, res)
	case Fulfilled:
		// a fulfilled state is considered a success
		if flags.onSuccess {
			time.Sleep(dd)
		}
		resolveToFulfilledRes(p, res)
	default:
		panic(fmt.Sprintf("promise: internal: unexpected Result state: '%s'", s))
	}
}

// if called from handleInvalidFollow, then it will be called once on the
// same promise, by design.
// if called from handleReturns, then it will be called once on the same
// promise, as it's called on return, and that can happen once.
func resolveToPanickedRes[T any](
	p *genericPromise[T],
	res Result[T],
) {
	// save the result, set the resolve status, and close the syncChan to unblock
	// all waiting calls.
	p.res = res
	p.resolveState.Store(uint32(Panicked))
	close(p.syncChan)

	handleExtCalls(p)

	// if the promise is panicked, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught panic to the last call
	// in the chain.
	if p.chainStatus.Load() == chainStatusEmpty {
		p.uncaughtPanicHandler()
	}
}

func resolveToRejectedRes[T any](
	p *genericPromise[T],
	res Result[T],
) {
	p.res = res
	p.resolveState.Store(uint32(Rejected))
	close(p.syncChan)

	handleExtCalls(p)

	// if the promise is rejected, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught error to the last call
	// in the chain.
	if p.chainStatus.Load() == chainStatusEmpty {
		p.uncaughtErrorHandler()
	}
}

func resolveToFulfilledRes[T any](
	p *genericPromise[T],
	res Result[T],
) {
	p.res = res
	p.resolveState.Store(uint32(Fulfilled))
	close(p.syncChan)

	handleExtCalls(p)
}

func handleExtCalls[T any](p *genericPromise[T]) (handled bool) {
	extQ := <-p.extsChan

	// handle not having any extension calls
	if !extQ.valid {
		return false
	}

	// get the final and ready-to-use result
	res := getFinalRes(p.res)

	// handle having a single extension call
	handled = handleExtCall(extQ.call, res)

	// handle having multiple extension calls
	for _, call := range extQ.extra {
		handled = handleExtCall(call, res) || handled
	}

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

func (p *genericPromise[T]) uncaughtPanicHandler() {
	if p.pipeline != nil {
		// if it's request to cancel all the promises of the pipeline on
		// uncaught panics, call the cancel function before the handler.
		if p.pipeline.cancel != nil {
			p.pipeline.cancel()
		}

		// call the handler if one is provided
		if p.pipeline.uncaughtPanicHandler != nil {
			p.pipeline.uncaughtPanicHandler(p.res.(panicResult).getPanicV())
		}
	}
}

func (p *genericPromise[T]) uncaughtErrorHandler() {
	if p.pipeline != nil {
		// if it's request to cancel all the promises of the pipeline on
		// uncaught errors, call the cancel function before the handler.
		if p.pipeline.cancel != nil {
			p.pipeline.cancel()
		}

		// call the handler if one is provided
		if p.pipeline.uncaughtErrorHandler != nil {
			p.pipeline.uncaughtErrorHandler(p.res.Err())
		}
	}
}

func (p *genericPromise[T]) resolveToResSync(res Result[T]) {
	if res != nil && res.Err() != nil {
		p.rejectSync(res)
	} else {
		p.fulfillSync(res)
	}
}

func (p *genericPromise[T]) panicSync(res Result[T]) {
	p.res = res
	p.resolveState.Store(uint32(Panicked))
}

func (p *genericPromise[T]) rejectSync(res Result[T]) {
	p.res = res
	p.resolveState.Store(uint32(Rejected))
}

func (p *genericPromise[T]) fulfillSync(res Result[T]) {
	p.res = res
	p.resolveState.Store(uint32(Fulfilled))
}

func (p *genericPromise[T]) privateImplementation() {}

func (p *genericPromise[T]) impl() *genericPromise[T] { return p }

// newPromInter creates a new genericPromise which is resolved internally,
// using an internal allocated channel.
func newPromInter[T any](pipeline *pipelineCore) *genericPromise[T] {
	extsChan := make(chan extQueue[T], 1)
	extsChan <- extQueue[T]{}

	return &genericPromise[T]{
		pipeline: pipeline,
		syncChan: make(chan struct{}),
		extsChan: extsChan,
	}
}

var closedChan = make(chan struct{})

func init() {
	close(closedChan)
}

// newPromSync creates a new genericPromise which is resolved synchronously,
// just after it's created.
func newPromSync[T any](pipeline *pipelineCore) *genericPromise[T] {
	return &genericPromise[T]{
		pipeline: pipeline,
		syncChan: closedChan,
		// not needed, since sync promises are resolved directly(synchronously)
		// after created, so any extension call will depend on the syncChan.
		extsChan: nil,
	}
}

// newPromBlocking returns a promise that will never be resolved.
// it's used for promises for Ctx calls with empty context.Context value,
// or for follow calls on such promises.
func newPromBlocking[T any]() *genericPromise[T] {
	return &genericPromise[T]{
		// no fields need to be initialized, since this promise will never be resolved.
	}
}
