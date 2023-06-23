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

	"github.com/asmsh/promise/internal/status"
)

// panic messages
const (
	nilCallbackPanicMsg   = "promise: the provided callback is nil"
	nilResChanPanicMsg    = "promise: the provided resChan is nil"
	multipleSendsPanicMsg = "promise: only one send should be done on the resChan"
)

// newPromInter creates a new GenericPromise which is resolved internally,
// using an internal allocated channel.
func newPromInter[T any](pipeline *pipelineCore, ctx context.Context, flags ...uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{
		pipeline: pipeline,
		ctx:      ctx,
		resChan:  make(chan Result[T]),
	}

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// newPromExter creates a new GenericPromise which is resolved externally,
// using an external allocated channel, the passed resChan.
func newPromExter[T any](pipeline *pipelineCore, ctx context.Context, resChan chan Result[T], flags ...uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{
		pipeline: pipeline,
		ctx:      ctx,
		resChan:  resChan,
		status:   status.PromStatus(status.FlagsIsExternal),
	}

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// newPromFollow creates a new GenericPromise, for one of the follow methods,
// which is resolved internally, using an internal allocated channel.
func newPromFollow[T any](pipeline *pipelineCore, ctx context.Context, prevStatus uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{
		pipeline: pipeline,
		ctx:      ctx,
		resChan:  make(chan Result[T]),
		status:   status.NewFromFlags(prevStatus),
	}

	return p
}

// newPromSync creates a new GenericPromise which is resolved synchronously,
// just after it's created.
func newPromSync[T any](pipeline *pipelineCore, ctx context.Context, flags ...uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{
		pipeline: pipeline,
		ctx:      ctx,
		// not needed, since sync promises are resolved directly after created,
		// and before being used.
		resChan: nil,
	}

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// there are two ways for this method to work in, internal, or external way.
//
// the internal way: the resChan will be an unbuffered chan, which is allocated
// internally.
// the promise is resolved when the resChan is closed, and the res field will
// has the result of the promise.
//
// the external way: the resChan will be bi-directional chan(buffered or not),
// which is allocated externally, by the user.
// the promise is resolved when a value is sent to the resChan chan, or it's
// closed. if a value is sent, that value will be the result of the promise,
// otherwise, the result will be nil.
//
// resolveOnTimeout if true, will set the state to pending(keep it as pending),
// and the fate to resolved, when the wait times-out.
// it should be true in all follow methods, in all implementation.
// it should be true in 'wait' calls only in the 'timed' implementation, and
// when the provided duration exceeds the wanted timeout.
//
// it returns true only if the wait times-out, otherwise it returns false.
func (p *GenericPromise[T]) wait() (timeout bool, s uint32) {
	s = p.status.Load()

	// if the fate is 'Resolved' or 'Handled', don't wait, as they are guaranteed
	// to happen after the result is saved, and after the resChan is closed.
	if status.IsFateResolved(s) || status.IsFateHandled(s) {
		return false, s
	}

	// wait with the appropriate wait procedure
	if status.IsFlagsExternal(s) {
		return p.exterWaitProc(p.ctx)
	} else {
		return p.interWaitProc(p.ctx)
	}
}

// exterWaitProc executes the wait procedure responsible for promises with
// externally allocated resChan.
// this method will be called in only one specific case, when the promise
// is created with the external flag set.
// it returns true only if the provided timerChan is received from, otherwise
// it returns false.
func (p *GenericPromise[T]) exterWaitProc(ctx context.Context) (timeout bool, s uint32) {
	select {
	case res, ok := <-p.resChan: // the chan is closed or a value is received
		if ok {
			// a value is received.
			// set the status to 'Resolving', and handle the result, only if it's
			// been just set, otherwise panic, accordingly.
			ok, s = p.status.SetResolving()
			if ok {
				// this is the first and only result received on this channel,
				// so handle it, accordingly.
				//
				// this will close the chan, regardless it's already closed or
				// not.
				// it might panic if a second result value is sent on it, but
				// since it's required that the chan be used only once for one
				// operation(either a close operation or sending a single value),
				// it's allowed and considered a result for bad usage of the chan.
				// TODO: make sure the value received isn't nil, otherwise reject
				s = resolveToRes(p, res)

				// FIXME: check if this is introducing a race condition
				// only one value should be sent, but more is sent, panic
				if len(p.resChan) != 0 {
					panic(multipleSendsPanicMsg)
				}
			} else {
				// this may happen if two or more values are sent, and the
				// first value is processed, and this is the second value.
				// only one value should be sent, but more is sent, panic.
				panic(multipleSendsPanicMsg)
			}
		} else {
			// the chan is closed, either by us or by the user.
			// set the status to 'Fulfilled' and 'Resolved'.
			//
			// if we closed the chan, then the res and status fields are now set
			// as expected, and attempting to set them again will not succeed,
			// and that's acceptable.
			//
			// if the user closed the chan, which is considered acceptable, only
			// if the user didn't send any values and just closed the chan, the
			// res field will be empty, cause the user didn't send any values,
			// and the status will be empty, so set it to fulfilled and resolve,
			// cause no reason to reject can happen.
			// TODO: in this scenario, the res value will be nil.
			// TODO: maybe, disallow users from closing the chan, or figure a way to detect it first.
			// TODO: this whole 'else' branch could be deleted and we can only rely on checking whether 'res' is 'nil' or not.
			_, s = p.status.SetFulfilledResolved()
		}

		return false, s
	case <-ctx.Done():
		// FIXME: check if this fix itself is not introducing a race condition on reading
		//  the res value, since wait might return now while the promise is still resolving.
		if set, s := p.status.SetResolving(); !set {
			return true, s
		}
		// create an error wrapping the errors that should be reported, by order
		err := newWrapErrs(ErrPromiseTimeout, ctx.Err())
		s = resolveToRejectedRes[T](p, Err[T](err))
		return true, s
	}
}

func (p *GenericPromise[T]) interWaitProc(ctx context.Context) (timeout bool, s uint32) {
	select {
	case <-p.resChan:
		// internally created res chan will always be closed by the previous
		// promise, after setting the res and status fields as expected.
		s = p.status.Load()
		return false, s
	case <-ctx.Done():
		if set, s := p.status.SetResolving(); !set {
			return true, s
		}
		// create an error wrapping the errors that should be reported, by order
		err := newWrapErrs(ErrPromiseTimeout, ctx.Err())
		s = resolveToRejectedRes[T](p, Err[T](err))
		return true, s
	}
}

func handleFollow[PrevResT, NewResT any](
	prevProm *GenericPromise[PrevResT],
	newProm *GenericPromise[NewResT],
	andResolve bool,
) (Result[PrevResT], bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or reject with an erroneous one.
	validHandle, s := prevProm.status.SetHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

	// if this is a valid handle, return the previous promise's result
	if validHandle {
		return prevProm.res, true
	}

	// otherwise, check if it's not request to resolve the new promise,
	// and return the appropriate error.
	if !andResolve {
		return Err[PrevResT](ErrPromiseConsumed), false
	}

	// otherwise, resolve the promise to the appropriate error and return
	resolveToRejectedRes[NewResT](newProm, Err[NewResT](ErrPromiseConsumed))
	return nil, false
}

// this handles invalid follow from then, catch, and recover calls.
// for any promise, this is guaranteed to be called only once, as it's called
// with the newly created promise as a receiver.
func handleInvalidFollow[ResT any](newProm *GenericPromise[ResT], prevRes Result[ResT], prevStatus uint32) {
	// return if the promise is resolved or being resolved by another call
	if set, _ := newProm.status.SetResolving(); !set {
		return
	}

	switch {
	case status.IsStateFulfilled(prevStatus):
		// the previous promise is fulfilled, fulfill with its result
		resolveToFulfilledRes(newProm, prevRes)
	case status.IsStateRejected(prevStatus):
		// the previous promise is rejected, reject with its result
		resolveToRejectedRes(newProm, prevRes)
	case status.IsStatePanicked(prevStatus):
		// the previous promise is panicked, panic with its result
		resolveToPanickedRes(newProm, prevRes)
	default:
		// TODO: investigate whether this might actually happen or not
		panic("promise: internal: unexpected state")
	}
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
func handleReturns[T any](newProm *GenericPromise[T], resP *Result[T]) {
	// make sure that only one call will resolve the promise, or return if
	// the promise is already resolved, so that we don't recover panics when
	// we don't need to.
	if set, _ := newProm.status.SetResolving(); !set {
		return
	}

	// FIXME: double check that this will work with runtime.Goexit(), prevent the goroutine from terminating
	if v := recover(); v == nil {
		// the callback returned normally, or through a call to runtime.Goexit.
		if resP == nil {
			// return from a callback that doesn't support Result returning.
			resolveToFulfilledRes[T](newProm, Empty[T]())
		} else {
			// return from a callback that requires Result returning,
			resolveToRes(newProm, *resP)
		}
	} else {
		// a panic happened, resolve to panicked with the panic value.
		resolveToPanickedRes[T](newProm, Err[T](newUncaughtPanic(v)))
	}
}

// resolveToRes resolves the promise when the computation has finished normally,
// without panic nor timeout.
// the promise is either rejected or fulfilled.
//
// if called from exterWaitProc or handleReturns, then it will be called once
// on the same promise, as it's protected by the Resolving fate setter.
func resolveToRes[T any](newProm *GenericPromise[T], res Result[T]) (s uint32) {
	if res == nil {
		return resolveToRejectedRes[T](newProm, Err[T](ErrPromiseNilResult))
	} else if err := res.Err(); err != nil {
		return resolveToRejectedRes(newProm, res)
	} else {
		return resolveToFulfilledRes(newProm, res)
	}
}

// if called from handleInvalidFollow, then it will be called once on the
// same promise, by design.
// if called from handleReturns, then it will be called once on the same
// promise, as it's called on return, and that can happen once.
func resolveToPanickedRes[ResT any](newProm *GenericPromise[ResT], res Result[ResT]) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	newProm.res = res
	_, s = newProm.status.SetPanickedResolved()
	close(newProm.resChan)

	// if the promise is panicked, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught panic to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		newProm.uncaughtPanicHandler()
	}

	return
}

func resolveToRejectedRes[ResT any](newProm *GenericPromise[ResT], res Result[ResT]) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	newProm.res = res
	_, s = newProm.status.SetRejectedResolved()
	close(newProm.resChan)

	// if the promise is rejected, and the chain is empty (no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught error to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		newProm.uncaughtErrorHandler()
	}

	return
}

func resolveToFulfilledRes[ResT any](newProm *GenericPromise[ResT], res Result[ResT]) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	newProm.res = res
	_, s = newProm.status.SetFulfilledResolved()
	close(newProm.resChan)

	return
}

func (p *GenericPromise[T]) uncaughtErrorHandler() {
	err := p.res.Err()
	if p.pipeline != nil && p.pipeline.config.UncaughtErrHandler != nil {
		p.pipeline.config.UncaughtErrHandler(err)
	} else {
		defUncaughtErrorHandler(err)
	}
}

func (p *GenericPromise[T]) uncaughtPanicHandler() {
	// TODO: make sure the Err() response is of type UncaughtPanic
	err := p.res.Err()
	v := err.(*UncaughtPanic).v
	if p.pipeline != nil && p.pipeline.config.UncaughtPanicHandler != nil {
		p.pipeline.config.UncaughtPanicHandler(v)
	} else {
		defUncaughtPanicHandler(v)
	}
}

func (p *GenericPromise[T]) resolveToResSync(res Result[T]) (s uint32) {
	if res == nil {
		return p.rejectSync(Err[T](ErrPromiseNilResult))
	} else if err := res.Err(); err != nil {
		return p.rejectSync(res)
	} else {
		return p.fulfillSync(res)
	}
}

func (p *GenericPromise[T]) panicSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetPanickedResolvedSync()
}

func (p *GenericPromise[T]) rejectSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetRejectedResolvedSync()
}

func (p *GenericPromise[T]) fulfillSync(res Result[T]) (s uint32) {
	p.res = res
	return p.status.SetFulfilledResolvedSync()
}

func (p *GenericPromise[T]) privateImplementation() {}

// asyncRead is used internally to implement promise extension functions.
//
// as it's registered as a 'read' call, it can prevent UncaughtError panics if
// the promise is about to be rejected, but it can't prevent a panicked promise
// from re-broadcasting the panic.
//
// the cb will be called with ok = true, only if the promise is fulfilled
// or rejected.
func (p *GenericPromise[T]) asyncRead(cb func(res Result[T], args []any), args ...any) {
	// register this as a 'read' call, and return if no callback is passed
	p.status.RegRead()
	if cb == nil {
		return
	}

	// asynchronously, wait the promise to be resolved and call cb, accordingly
	go p.asyncReadCall(cb, args)
}

// asyncReadCall will call the cb with an argument that corresponds to the result
// of a GetRes call.
//
// if the promise panics without having a 'follow' call, then the program will
// crash, so if the promise' state is found to be panicked, here, this means
// that, either it's already handled, or there are one or more follow calls
// in the chain that might handle it, and if the panic reached the end of the
// chain without finding a Recover call, the promise will re-broadcast the
// panic, so there's no need to care about panics here.
func (p *GenericPromise[T]) asyncReadCall(
	cb func(res Result[T], args []any),
	args []any,
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	p.wait()

	// run the callback
	cb(p.res, args)
}

// asyncFollow is used internally to implement promise extension functions.
//
// as it's registered as a 'follow' call, it can prevent UncaughtError panics if
// the promise is about to be rejected, and also prevent a panicked promise from
// re-broadcasting the panic.
//
// the cb will be called with ok = true, only if the promise is fulfilled
// or rejected.
func (p *GenericPromise[T]) asyncFollow(cb func(res Result[T], args []any), args ...interface{}) {
	// register this as a 'follow' call, and return if no callback is passed
	p.status.RegFollow()
	if cb == nil {
		return
	}
}
