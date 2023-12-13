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
	"fmt"

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

	// wait with the appropriate wait procedure
	p.interWaitProc()

	// return the up-to-date status value
	return p.status.Load()
}

func (p *genericPromise[T]) interWaitProc() {
	// the chan will always be closed by the previous promise,
	// after setting the res and status fields as expected.
	<-p.syncChan
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
	return prevProm.getRes(), true
}

// this handles invalid follow from then, catch, and recover calls.
// for any promise, this is guaranteed to be called only once, as it's called
// with the newly created promise as a receiver.
func handleInvalidFollow[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	prevStatus uint32,
) {
	// return if the promise is resolved or being resolved by another call
	if set, _ := nextProm.status.SetResolving(); !set {
		return
	}

	switch {
	case status.IsStateFulfilled(prevStatus):
		// the previous promise is fulfilled, fulfill with its result
		resolveToFulfilledRes(nextProm, prevProm.res)
	case status.IsStateRejected(prevStatus):
		// the previous promise is rejected, reject with its result
		resolveToRejectedRes(nextProm, prevProm.res)
	case status.IsStatePanicked(prevStatus):
		// the previous promise is panicked, panic with its result
		resolveToPanickedRes(nextProm, prevProm.res)
	default:
		// TODO: investigate whether this might actually happen or not
		panic(fmt.Sprintf("promise: internal: unexpected state: '%b'", prevStatus))
	}
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
// TODO: pass a new value, panicked (similar to valid from the sync.OnceFunc implementation),
// and make the handleReturns function uses this value to tell whether the nil value is valid or not.
func handleReturns[T any](prom *genericPromise[T], resP *Result[T]) {
	// make sure that only one call will resolve the promise, or return if
	// the promise is already resolved, so that we don't recover panics when
	// we don't need to.
	if set, _ := prom.status.SetResolving(); !set {
		return
	}

	// FIXME: double check that this will work with runtime.Goexit(), prevent the goroutine from terminating
	if v := recover(); v == nil {
		// the callback returned normally, through a call to runtime.Goexit,
		// or with a nil panic value.
		if resP != nil {
			// return from a callback that requires Result returning
			resolveToRes[T](prom, *resP)
		} else {
			// return from a callback that doesn't support Result returning.
			// this is equivalent to setting the result to Empty[T] explicitly.
			resolveToFulfilledRes[T](prom, nil)
		}
	} else {
		// a panic happened, resolve to panicked with the panic value.
		resolveToPanickedRes[T](prom, errPromisePanickedResult[T]{v: v})
	}
}

// resolveToRes resolves the promise when the computation has finished normally,
// without panic nor timeout.
// the promise is either rejected or fulfilled.
//
// if called from exterWaitProc or handleReturns, then it will be called once
// on the same promise, as it's protected by the Resolving fate setter.
func resolveToRes[T any](prom *genericPromise[T], res Result[T]) (s uint32) {
	if res != nil && res.Err() != nil {
		return resolveToRejectedRes(prom, res)
	} else {
		return resolveToFulfilledRes(prom, res)
	}
}

// if called from handleInvalidFollow, then it will be called once on the
// same promise, by design.
// if called from handleReturns, then it will be called once on the same
// promise, as it's called on return, and that can happen once.
func resolveToPanickedRes[T any](
	prom *genericPromise[T],
	res Result[T],
) (s uint32) {
	// save the result, update the status, and close the syncChan to unblock
	// all waiting calls.
	prom.res = res
	_, s = prom.status.SetPanickedResolved()
	close(prom.syncChan)

	handleExtCalls(prom, Panicked)

	// if the promise is panicked, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught panic to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		prom.uncaughtPanicHandler()
	}

	return
}

func resolveToRejectedRes[T any](
	prom *genericPromise[T],
	res Result[T],
) (s uint32) {
	// save the result, update the status, and close the syncChan to unblock
	// all waiting calls.
	prom.res = res
	_, s = prom.status.SetRejectedResolved()
	close(prom.syncChan)

	handleExtCalls(prom, Rejected)

	// if the promise is rejected, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught error to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		prom.uncaughtErrorHandler()
	}

	return
}

func resolveToFulfilledRes[T any](
	prom *genericPromise[T],
	res Result[T],
) (s uint32) {
	prom.res = res
	_, s = prom.status.SetFulfilledResolved()
	close(prom.syncChan)

	handleExtCalls(prom, Fulfilled)
	return
}

func handleExtCalls[T any](prom *genericPromise[T], state State) (handled bool) {
	extQ := <-prom.extsChan

	// handle not having any extension calls
	if !extQ.valid {
		return false
	}

	// get the final and ready-to-use result
	res := prom.getRes()

	// handle having a single extension call
	handled = handleExtCall(extQ.call, res, state) || handled

	// handle having multiple extension calls
	for _, call := range extQ.extra {
		handled = handleExtCall(call, res, state) || handled
	}

	return handled
}

func handleExtCall[T any](call extCall[T], res Result[T], state State) bool {
	select {
	case call.resChan <- IdxRes[T]{
		Idx:   call.idx,
		Res:   res,
		State: state,
	}:
		return true
	case <-call.syncChan:
		return false
	}
}

func getPanicV[T any](res Result[T]) any {
	err := res.(errPromisePanickedResult[T])
	return err.v
}

func (p *genericPromise[T]) uncaughtPanicHandler() {
	v := UncaughtPanic{v: getPanicV(p.res)}
	if p.pipeline == nil {
		defUncaughtPanicHandler(v)
	} else {
		// if there's a pipeline cancel function, call it before the handler
		if p.pipeline.cancel != nil {
			p.pipeline.cancel()
		}

		// still, resolve to using the default handler if none is defined
		if p.pipeline.uncaughtPanicHandler != nil {
			p.pipeline.uncaughtPanicHandler(v)
		} else {
			defUncaughtPanicHandler(v)
		}
	}
}

func (p *genericPromise[T]) uncaughtErrorHandler() {
	err := p.res.Err()
	v := UncaughtError{err: err}
	if p.pipeline == nil {
		defUncaughtErrorHandler(v)
	} else {
		// if there's a pipeline cancel function, call it before the handler
		if p.pipeline.cancel != nil {
			p.pipeline.cancel()
		}

		// still, resolve to using the default handler if none is defined
		if p.pipeline.uncaughtErrorHandler != nil {
			p.pipeline.uncaughtErrorHandler(v)
		} else {
			defUncaughtErrorHandler(v)
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
