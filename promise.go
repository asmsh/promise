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

// GenericPromise is the default implementation of the Promise interface
//
// The zero value will block forever on any calls.
type GenericPromise[T any] struct {
	pipeline *pipelineCore

	ctx context.Context

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

func (p *GenericPromise[T]) Status() Status {
	s := p.status.Load()
	return Status(s)
}

func (p *GenericPromise[T]) Wait() {
	p.status.RegWait()
	p.waitCall(false)
}

func (p *GenericPromise[T]) WaitChan() chan struct{} {
	c := make(chan struct{})

	go func(c chan struct{}) {
		p.Wait()
		close(c)
	}(c)

	return c
}

func (p *GenericPromise[T]) Res() Result[T] {
	p.status.RegRead()
	return p.waitCall(true)
}

func (p *GenericPromise[T]) waitCall(andHandle bool) Result[T] {
	// wait the promise to be resolved, or until its context is Done.
	_, s := p.wait()

	// we only need to do the following special checks if it's not a 'Handle' call,
	// since a 'Handle' call will always prevent the execution of the different
	// uncaught handlers, regardless of the promise chain or state.
	if !andHandle {
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

		// return without the result, since it's not needed(not a 'Handle' call)
		return nil
	}

	// if it's a call to handle the result, set the 'Handled' flag.
	// also, keep track of whether this handle was valid(first) or not,
	// to decide whether we should return the actual result of the promise
	// or an erroneous one.
	validHandle, s := p.status.SetHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

	// return the actual result of the promise, or an erroneous one
	if validHandle {
		return p.res
	} else {
		return Err[T](ErrPromiseConsumed)
	}
}

func (p *GenericPromise[T]) Delay(
	d time.Duration,
	onSuccess bool,
	onFailure bool,
) Promise[T] {
	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	newProm := newPromFollow[T](p.pipeline, context.Background(), s)
	go delayFollowCall(p, newProm, d, onSuccess, onFailure)
	return newProm
}

// delay creates Promise values with the same type
func delayFollowCall[ResT any](
	prevProm *GenericPromise[ResT],
	newProm *GenericPromise[ResT],
	dd time.Duration,
	onSuccess bool,
	onFailure bool,
) {
	// make sure we free this goroutine reservation
	defer newProm.pipeline.freeGoroutine()

	// wait the previous promise to be resolved
	_, s := prevProm.wait()

	// mark prevProm as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be used.
	res, ok := handleFollow(prevProm, newProm, false)
	if !ok {
		// it's not a valid handle. it's considered a failure.
		if onFailure {
			time.Sleep(dd)
		}
		resolveToRejectedRes(newProm, res)
		return
	}

	switch {
	case status.IsStateFulfilled(s):
		// a fulfilled state is considered a success
		if onSuccess {
			time.Sleep(dd)
		}
		resolveToFulfilledRes(newProm, res)
	case status.IsStateRejected(s):
		// a rejected state is considered a failure
		if onFailure {
			time.Sleep(dd)
		}
		resolveToRejectedRes(newProm, res)
	case status.IsStatePanicked(s):
		// a panicked state is considered a failure
		if onFailure {
			time.Sleep(dd)
		}
		resolveToPanickedRes(newProm, res)
	default:
		// TODO: investigate whether this might actually happen or not
		panic("promise: internal: unexpected state")
	}
}

func (p *GenericPromise[T]) Then(
	ctx context.Context,
	thenCb func(ctx context.Context, val T) Result[T],
) Promise[T] {
	if thenCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	newProm := newPromFollow[T](p.pipeline, ctx, s)
	go newProm.thenCall(p, thenCb)
	return newProm
}

func (p *GenericPromise[T]) thenCall(
	prev *GenericPromise[T],
	cb thenCallback[T, T],
) {
	// wait the previous promise to be resolved
	_, s := prev.wait()

	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// 'Then' can handle only the 'Fulfilled' state, so return otherwise
	if !status.IsStateFulfilled(s) {
		handleInvalidFollow(p, prev.res, s)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prev, p, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback[T, T](p, cb, true, res, s, false)
}

func (p *GenericPromise[T]) Catch(
	ctx context.Context,
	catchCb func(ctx context.Context, val T, err error) Result[T],
) Promise[T] {
	if catchCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	newProm := newPromFollow[T](p.pipeline, ctx, s)
	go newProm.catchCall(p, catchCb)
	return newProm
}

func (p *GenericPromise[T]) catchCall(
	prev *GenericPromise[T],
	cb catchCallback[T, T],
) {
	// wait the previous promise to be resolved
	_, s := prev.wait()

	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// 'Catch' can handle only the 'Rejected' state, so return otherwise
	if !status.IsStateRejected(s) {
		handleInvalidFollow(p, prev.res, s)
		return
	}

	// mark prev as 'Handled'.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, _ := handleFollow(prev, p, false)

	// run the callback with the actual promise result
	runCallback[T, T](p, cb, true, res, s, false)
}

func (p *GenericPromise[T]) Recover(
	ctx context.Context,
	recoverCb func(ctx context.Context, v any) Result[T],
) Promise[T] {
	if recoverCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, s := p.status.RegFollow()
	p.pipeline.reserveGoroutine()
	newProm := newPromFollow[T](p.pipeline, ctx, s)
	go newProm.recoverCall(p, ctx, recoverCb)
	return newProm
}

func (p *GenericPromise[T]) recoverCall(
	prev *GenericPromise[T],
	ctx context.Context,
	cb recoverCallback[T, T],
) {
	// wait the previous promise to be resolved
	_, s := prev.wait()

	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// 'Recover' can handle only the 'Panicked' state, so return otherwise
	if !status.IsStatePanicked(s) {
		handleInvalidFollow(p, prev.res, s)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prev, p, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback[T, T](p, cb, true, res, s, false)
}

func (p *GenericPromise[T]) Finally(
	ctx context.Context,
	finallyCb func(ctx context.Context, s Status) Result[T],
) Promise[T] {
	if finallyCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, s := p.status.RegWait()
	p.pipeline.reserveGoroutine()
	newProm := newPromFollow[T](p.pipeline, ctx, s)
	go newProm.finallyCall(p, ctx, finallyCb)
	return newProm
}

// finallyCall is like an asyncReadCall, except that it can't set the 'Handled'
// flag(handle the promise), and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a fulfilled promise,
// and the panic will be dismissed implicitly, which is something we don't want.
func (p *GenericPromise[T]) finallyCall(
	prev *GenericPromise[T],
	ctx context.Context,
	cb finallyCallback[T, T],
) {
	// wait the previous promise to be resolved
	_, s := prev.wait()

	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// run the callback with the actual promise result
	runCallback[T, T](p, cb, true, prev.res, s, false)
}
