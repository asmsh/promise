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

	"github.com/asmsh/promise/internal/status"
)

// panic messages
const (
	nilCallbackPanicMsg = "promise: the provided callback is nil"
	nilResChanPanicMsg  = "promise: the provided resChan is nil"
)

// wait waits for the promise p to be resolved, by either blocking on receiving
// from the syncChan, or utilizing the fate value of the promise status.
//
// the syncChan will be an unbuffered chan, which is allocated internally.
// the promise is resolved when the syncChan is closed, and the res field will
// have the result of the promise.
func (p *genericPromise[T]) wait() (s uint32) {
	s = p.status.Load()

	// if the fate is 'Resolved' or 'Handled', don't wait, as they are guaranteed
	// to happen after the result is saved, and after the syncChan is closed.
	if status.IsFateResolved(s) || status.IsFateHandled(s) {
		return s
	}

	// wait until the promise is resolved.
	// the chan will always be closed by the previous promise,
	// after setting the res and status fields as expected.
	<-p.syncChan

	// return the up-to-date status value
	return p.status.Load()
}

func handleFollow[PrevT, NextT any](
	prevProm *genericPromise[PrevT],
	nextProm *genericPromise[NextT],
	resolveOnErr bool,
) (resToBeHandled Result[PrevT], valid bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or reject with an erroneous one.
	validHandle, s := prevProm.status.SetHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

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
) (s uint32) {
	// save the result, update the status, and close the syncChan to unblock
	// all waiting calls.
	p.res = res
	_, s = p.status.SetPanickedResolved()
	close(p.syncChan)

	handleExtCalls(p)

	// if the promise is panicked, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught panic to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		p.uncaughtPanicHandler()
	}

	return
}

func resolveToRejectedRes[T any](
	p *genericPromise[T],
	res Result[T],
) (s uint32) {
	// save the result, update the status, and close the syncChan to unblock
	// all waiting calls.
	p.res = res
	_, s = p.status.SetRejectedResolved()
	close(p.syncChan)

	handleExtCalls(p)

	// if the promise is rejected, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught error to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		p.uncaughtErrorHandler()
	}

	return
}

func resolveToFulfilledRes[T any](
	p *genericPromise[T],
	res Result[T],
) (s uint32) {
	p.res = res
	_, s = p.status.SetFulfilledResolved()
	close(p.syncChan)

	handleExtCalls(p)
	return
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
	handled = handleExtCall(extQ.call, res) || handled

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

func (p *genericPromise[T]) resolveToResSync(res Result[T]) (s uint32) {
	if res != nil && res.Err() != nil {
		return p.rejectSync(res)
	} else {
		return p.fulfillSync(res)
	}
}

func (p *genericPromise[T]) panicSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetPanickedResolvedSync()
}

func (p *genericPromise[T]) rejectSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetRejectedResolvedSync()
}

func (p *genericPromise[T]) fulfillSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetFulfilledResolvedSync()
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

// newPromFollow creates a new genericPromise, for one of the follow methods,
// which is resolved internally, using an internal allocated channel.
func newPromFollow[T any](pipeline *pipelineCore, prevStatus uint32) *genericPromise[T] {
	p := newPromInter[T](pipeline)
	p.status = status.NewFrom(prevStatus)
	return p
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
		// none of these fields needs to be initialized, since this promise
		// will never be resolved.
		pipeline: nil,
		syncChan: nil,
		extsChan: nil,
	}
}
