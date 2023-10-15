// Copyright 2020 Ahmad Sameh(asmsh)
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

	"github.com/asmsh/promise/internal/status"
)

// genericPromise is the default implementation of the Promise interface
//
// The zero value will block forever on any calls.
type genericPromise[T any] struct {
	pipeline *pipelineCore

	// holds the result of the promise.
	// written once, before the resChan channel is closed.
	//
	// don't read it unless the resChan is known to be closed.
	res Result[T]

	// closed or received from when this promise is resolved.
	//
	// when this chan is created internally, it will be an unbuffered chan,
	// in that case the result will be written to the res field, then it
	// will be closed.
	//
	// when this chan is created externally and passed, it will be either a
	// buffered chan, or an unbuffered chan, but it must be a bi-directional
	// chan(so that it can be closed internally).
	// in that case, to resolve the promise, the chan creator either sends
	// the result to it, for only one time, OR, just close it.
	// when the promise is resolved by closing the chan, it will always be
	// fulfilled with a nil result, otherwise, it will be either fulfilled
	// or rejected, depending on whether the sent result contains a non-nil
	// error as its last element or not.
	// the result of the promise will be the sent result.
	//
	// note: not following the expected behavior, either when passing the chan
	// or when resolving the promise, will result in a run-time panic, either
	// internally or in the creator side, and may introduce a data race on this
	// channel.
	resChan chan Result[T]

	// hold the status of the different states, fates, flags, and other data
	// about the promise.
	// refer to the docs of the promStatus type for more info.
	//
	// the res field is guaranteed to be immutable, after the fate value is
	// Resolved or Handled, so don't read it before then.
	status status.PromStatus
}

func (p *genericPromise[T]) Status() Status {
	s := p.status.Load()
	return Status(s)
}

func (p *genericPromise[T]) Wait() {
	p.status.RegWait()
	_ = p.waitCall()
}

func (p *genericPromise[T]) WaitChan() chan struct{} {
	c := make(chan struct{})

	go func(c chan struct{}) {
		p.Wait()
		close(c)
	}(c)

	return c
}

func (p *genericPromise[T]) waitCall() (s uint32) {
	// wait the promise to be resolved
	s = p.wait()

	if !status.IsChainAtLeastRead(s) && !status.IsFateHandled(s) {
		if status.IsStatePanicked(s) {
			// the promise is panicked, not handled, and there are no chained
			// calls to handle it, run the uncaught panic handler, and continue.
			p.uncaughtPanicHandler()
		}

		if status.IsStateRejected(s) {
			// the promise is rejected, not handled, and there are no chained
			// calls to handle it, run the uncaught error handler, and continue.
			p.uncaughtErrorHandler()
		}
	}

	return
}

func (p *genericPromise[T]) Res() Result[T] {
	p.status.RegRead()
	return p.resCall()
}

func (p *genericPromise[T]) resCall() Result[T] {
	// wait the promise to be resolved
	p.wait()

	// if it's a call to handle the result, set the 'Handled' flag.
	// also, keep track of whether this handle was valid(first) or not,
	// to decide whether we should return the actual result of the promise
	// or an erroneous one.
	validHandle, s := p.status.SetHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

	// if the promise result has been used, return the expected error
	if !validHandle {
		return errPromiseConsumedResult[T]{}
	}

	// the promise result can be accessed multiple times...
	// if no result was set, then it's implicitly the empty result
	if p.res == nil {
		return emptyResult[T]{}
	}

	return p.res
}

func (p *genericPromise[T]) Delay(
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	_, s := p.status.RegFollow()
	flags := getDelayFlags(cond)
	p.pipeline.reserveGoroutine()
	nextProm := newPromFollow[T](p.pipeline, s)
	go delayFollowCall(p, nextProm, d, flags)
	return nextProm
}

// delay creates Promise values with the same type
func delayFollowCall[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	dd time.Duration,
	flags delayFlags,
) {
	// make sure we free this goroutine reservation
	defer prevProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	s := prevProm.wait()

	// mark prevProm as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be used.
	res, ok := handleFollow(prevProm, nextProm, false)
	if !ok {
		// it's not a valid handle. it's considered a failure.
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToRejectedRes(nextProm, res)
		return
	}

	switch {
	case status.IsStateFulfilled(s):
		// a fulfilled state is considered a success
		if flags.onSuccess {
			time.Sleep(dd)
		}
		resolveToFulfilledRes(nextProm, res)
	case status.IsStateRejected(s):
		// a rejected state is considered a failure
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToRejectedRes(nextProm, res)
	case status.IsStatePanicked(s):
		// a panicked state is considered a failure
		if flags.onPanic {
			time.Sleep(dd)
		}
		resolveToPanickedRes(nextProm, res)
	default:
		// TODO: investigate whether this might actually happen or not
		panic("promise: internal: unexpected state")
	}
}

func (p *genericPromise[T]) Then(
	thenCb func(ctx context.Context, val T) Result[T],
) Promise[T] {
	if thenCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	nextProm := newPromFollow[T](p.pipeline, s)
	go thenFollowCall(p, nextProm, thenCb)
	return nextProm
}

func thenFollowCall[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	cb thenCallback[T, T],
) {
	// make sure we free this goroutine reservation
	defer prevProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	s := prevProm.wait()

	// 'Then' can handle only the 'Fulfilled' state, so return otherwise
	if !status.IsStateFulfilled(s) {
		handleInvalidFollow(prevProm, nextProm, s)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prevProm, nextProm, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback[T, T](nextProm, cb, res, true, false)
}

func (p *genericPromise[T]) Catch(
	catchCb func(ctx context.Context, val T, err error) Result[T],
) Promise[T] {
	if catchCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	nextProm := newPromFollow[T](p.pipeline, s)
	go catchFollowCall(p, nextProm, catchCb)
	return nextProm
}

func catchFollowCall[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	cb catchCallback[T, T],
) {
	// make sure we free this goroutine reservation
	defer prevProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	s := prevProm.wait()

	// 'Catch' can handle only the 'Rejected' state, so return otherwise
	if !status.IsStateRejected(s) {
		handleInvalidFollow(prevProm, nextProm, s)
		return
	}

	// mark prev as 'Handled'.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, _ := handleFollow(prevProm, nextProm, false)

	// run the callback with the actual promise result
	runCallback[T, T](nextProm, cb, res, true, false)
}

func (p *genericPromise[T]) Recover(
	recoverCb func(ctx context.Context, v any) Result[T],
) Promise[T] {
	if recoverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	nextProm := newPromFollow[T](p.pipeline, s)
	go recoverFollowCall(p, nextProm, recoverCb)
	return nextProm
}

func recoverFollowCall[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	cb recoverCallback[T, T],
) {
	// make sure we free this goroutine reservation
	defer prevProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	s := prevProm.wait()

	// 'Recover' can handle only the 'Panicked' state, so return otherwise
	if !status.IsStatePanicked(s) {
		handleInvalidFollow(prevProm, nextProm, s)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prevProm, nextProm, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback[T, T](nextProm, cb, res, true, false)
}

func (p *genericPromise[T]) Finally(
	finallyCb func(ctx context.Context) Result[T],
) Promise[T] {
	if finallyCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegWait()
	p.pipeline.reserveGoroutine()
	nextProm := newPromFollow[T](p.pipeline, s)
	go finallyFollowCall(p, nextProm, finallyCb)
	return nextProm
}

// finallyFollowCall is like an asyncReadCall, except that it can't set the 'Handled'
// flag(handle the promise), and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a fulfilled promise,
// and the panic will be dismissed implicitly, which is something we don't want.
func finallyFollowCall[T any](
	prevProm *genericPromise[T],
	nextProm *genericPromise[T],
	cb finallyCallback[T, T],
) {
	// make sure we free this goroutine reservation
	defer prevProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	prevProm.wait()

	// run the callback with the actual promise result
	runCallback[T, T](nextProm, cb, prevProm.res, true, false)
}
