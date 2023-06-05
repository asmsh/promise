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
	"github.com/asmsh/promise/result"
)

type GoPromise = GenericPromise[result.AnyRes]
type AnyPromise = GenericPromise[any]

// GenericPromise is the default implementation of the Promise interface
//
// The zero value will block forever on any calls.
type GenericPromise[T any] struct {
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

// panic messages
const (
	nilCallbackPanicMsg   = "promise: the provided callback is nil"
	nilResChanPanicMsg    = "promise: the provided resChan is nil"
	multipleSendsPanicMsg = "promise: only one send should be done on the resChan"
)

// newPromInter creates a new GenericPromise which is resolved internally,
// using an internal allocated channel.
func newPromInter[T any](flags ...uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{resChan: make(chan Result[T])}

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// newPromExter creates a new GenericPromise which is resolved externally,
// using an external allocated channel, the passed resChan.
func newPromExter[T any](resChan chan Result[T], flags ...uint32) *GenericPromise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	p := &GenericPromise[T]{resChan: resChan}

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// newPromSync creates a new GenericPromise which is resolved synchronously,
// just after it's created.
func newPromSync[T any](flags ...uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{resChan: make(chan Result[T])}
	close(p.resChan)

	// set the flags of the promise, accordingly
	for f := range flags {
		p.status = p.status | status.PromStatus(f)
	}

	return p
}

// newPromFollow creates a new GenericPromise, for one of the follow methods,
// which is resolved internally, using an internal allocated channel.
func newPromFollow[T any](ps uint32) *GenericPromise[T] {
	p := &GenericPromise[T]{
		resChan: make(chan Result[T]),
		status:  status.NewFromFlags(ps),
	}

	return p
}

func (p *GenericPromise[T]) Status() Status {
	s := p.status.Load()
	return Status(s)
}

func (p *GenericPromise[T]) Wait() {
	p.status.RegWait()
	p.waitCall(context.Background(), false)
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
	return p.waitCall(context.Background(), true)
}

func (p *GenericPromise[T]) waitCall(ctx context.Context, andHandle bool) Result[T] {
	// wait the promise to be resolved, or until its context is Done,
	// and then read its status.
	_, s := p.wait(ctx)

	// if it's a call to handle the result, set the 'Handled' flag.
	// also, keep track of whether this handle was valid(first) or not,
	// to decide whether we should return the actual result of the promise
	// or an erroneous one.
	var validHandle bool
	if andHandle {
		validHandle, s = p.status.SetHandled()
	}

	if status.IsStatePanicked(s) {
		// the promise is panicked, it's not handled, and there are no
		// chained calls to handle it, execute the uncaught panic handler.
		if !status.IsChainAtLeastRead(s) && !status.IsFateHandled(s) {
			p.uncaughtPanicHandler()
		}

		// move forward to return the actual result of the promise, if needed
		if andHandle {
			return p.res
		} else {
			return nil
		}
	}

	if status.IsStateRejected(s) {
		// the promise is rejected, it's not handled, and there are no
		// chained calls to handle it, execute the uncaught error handler.
		if !status.IsChainAtLeastRead(s) && !status.IsFateHandled(s) {
			p.uncaughtErrorHandler()
		}

		// move forward to return the actual result of the promise, if needed
		if andHandle {
			return p.res
		} else {
			return nil
		}
	}

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

	if andHandle {
		if validHandle {
			// it's a valid handle call, return the actual result of the promise
			return p.res
		} else {
			// otherwise, return an erroneous result
			return result.Err[T](ErrPromiseConsumed)
		}
	}

	// don't return the result if it's not needed
	return nil
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
func (p *GenericPromise[T]) wait(ctx context.Context) (timeout bool, s uint32) {
	s = p.status.Load()

	// if the fate is 'Resolved' or 'Handled', don't wait, as they are guaranteed
	// to happen after the result is saved, and after the resChan is closed.
	if status.IsFateResolved(s) || status.IsFateHandled(s) {
		return false, s
	}

	// wait with the appropriate wait procedure
	if status.IsFlagsExternal(s) {
		return p.exterWaitProc(ctx)
	} else {
		return p.interWaitProc(ctx)
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
				s = p.resolveToRes(res)

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
		s = p.resolveToRejectedWithErr(ErrPromiseTimeout, false)
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
		s = p.resolveToRejectedWithErr(ErrPromiseTimeout, false)
		return true, s
	}
}

func (p *GenericPromise[T]) Delay(
	d time.Duration,
	onSucceed bool,
	onFail bool,
) Promise[T] {
	_, s := p.status.RegFollow()
	newProm := newPromFollow[T](s)
	go newProm.delayCall(context.Background(), p, d, onSucceed, onFail)
	return newProm
}

func (p *GenericPromise[T]) delayCall(
	ctx context.Context,
	prev *GenericPromise[T],
	dd time.Duration,
	onSucceed bool,
	onFail bool,
) {
	// wait the previous promise to be resolved, or until ctx is closed
	_, s := prev.wait(ctx)

	// mark the promise as 'Handled', and check whether we should continue or not.
	// this will reject immediately if the promise was already handled.
	res, ok := p.handleFollow(prev.res, true)
	if !ok {
		return
	}

	switch {
	case status.IsStateFulfilled(s):
		// a fulfilled state is considered a success
		if onSucceed {
			time.Sleep(dd)
		}
		p.resolveToFulfilledRes(res, false)
	case status.IsStateRejected(s):
		// a rejected state is considered a failure
		if onFail {
			time.Sleep(dd)
		}
		p.resolveToRejectedRes(res, false)
	case status.IsStatePanicked(s):
		// a panicked state is considered a failure
		if onFail {
			time.Sleep(dd)
		}
		p.resolveToPanickedRes(res, false)
	default:
		// TODO: investigate whether this might actually happen or not
		panic("promise: internal: unexpected state")
	}
}

func (p *GenericPromise[T]) Then(thenCb func(val T) Result[T]) Promise[T] {
	if thenCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newPromFollow[T](s)
	go newProm.thenCall(context.Background(), p, thenCb)
	return newProm
}

func (p *GenericPromise[T]) thenCall(
	ctx context.Context,
	prev *GenericPromise[T],
	cb thenCallback[T],
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	_, s := prev.wait(ctx)

	// 'Then' can handle only the 'Fulfilled' state, so return otherwise
	if !status.IsStateFulfilled(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// mark the promise as 'Handled', and check whether we should continue or not.
	// the result value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := p.handleFollow(prev.res, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback(p, res, s, cb)
}

func (p *GenericPromise[T]) handleFollow(prevRes Result[T], andResolve bool) (res Result[T], ok bool) {
	// set the 'Handled' flag, and keep track of whether this handle is
	// valid(first) or not, to decide whether we should move forward and
	// use the actual result of the promise or reject with an erroneous one.
	validHandle, s := p.status.SetHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !status.IsFlagsOnce(s) {
		validHandle = true
	}

	// and if this handle call isn't valid, return and/or reject appropriate error
	if !validHandle {
		res = result.Err[T](ErrPromiseConsumed)
		if andResolve {
			p.resolveToRejectedRes(res, false)
		}
	} else {
		res = prevRes
	}

	return res, validHandle
}

// this handles invalid follow from then, catch, and recover calls.
// for any promise, this is guaranteed to be called only once, as it's called
// with the newly created promise as a receiver.
func (p *GenericPromise[T]) handleInvalidFollow(prevRes Result[T], prevStatus uint32) {
	switch {
	case status.IsStateFulfilled(prevStatus):
		// the previous promise is fulfilled, fulfill with its result
		p.resolveToFulfilledRes(prevRes, false)
	case status.IsStateRejected(prevStatus):
		// the previous promise is rejected, reject with its result
		p.resolveToRejectedRes(prevRes, false)
	case status.IsStatePanicked(prevStatus):
		// the previous promise is panicked, panic with its result
		p.resolveToPanickedRes(prevRes, false)
	default:
		// TODO: investigate whether this might actually happen or not
		panic("promise: internal: unexpected state")
	}
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
func (p *GenericPromise[T]) handleReturns(resP *Result[T]) {
	// make sure that only one call will resolve the promise, or return if
	// the promise is already resolved, so that we don't recover panics when
	// we don't need to.
	set, _ := p.status.SetResolving()
	if !set {
		return
	}

	// FIXME: double check that this will work with runtime.Goexit()
	v := recover()
	if v == nil {
		// no panic, resolve to the AnyRes value pointed to, as the callback is
		// either returned normally, or through a call to runtime.Goexit call,
		// and in either case the AnyRes value pointed to will not change here.
		p.resolveToRes(*resP)
	} else {
		// a panic happened, resolve to panicked, with the panic value.
		p.resolveToPanickedRes(result.Err[T](newUncaughtPanic(v)), false)
	}
}

// resolveToRes resolves the promise when the computation has finished normally,
// without panic nor timeout.
// the promise is either rejected or fulfilled, depending on whether the last
// element of res is a non-nil error value or not, respectively.
//
// if called from exterWaitProc, then it will be called once on the same
// promise, as it's protected by the Resolving fate setter.
// if called from handleReturns, then it will be called once on the same
// promise, as it's protected by the Resolving fate setter.
// if called from finallyCall(before handleReturns), then it will be called
// once on the same promise, by design.
// if called from the resolverCb, then it will be called once on the same
// promise, as it's protected by the Resolving fate setter.
func (p *GenericPromise[T]) resolveToRes(res Result[T]) (s uint32) {
	if err := res.Err(); err != nil {
		return p.resolveToRejectedRes(res, false)
	} else {
		return p.resolveToFulfilledRes(res, false)
	}
}

func (p *GenericPromise[T]) uncaughtPanicHandler() {
	// TODO: make sure the Err() response is of type UncaughtPanic
	err := p.res.Err()
	defUncaughtPanicHandler(err.(*UncaughtPanic))
}

func defUncaughtPanicHandler(e *UncaughtPanic) {
	panic(e.Error())
}

// if called from handleInvalidFollow, then it will be called once on the
// same promise, by design.
// if called from handleReturns, then it will be called once on the same
// promise, as it's called on return, and that can happen once.
func (p *GenericPromise[T]) resolveToPanickedRes(res Result[T], andHandle bool) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	_, s = p.status.SetPanickedResolved()
	if andHandle {
		_, s = p.status.SetHandled() // set to Handled, if requested
	}
	close(p.resChan)

	// if the promise is panicked, and the chain is empty(no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught panic to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		p.uncaughtPanicHandler()
	}

	return
}

func (p *GenericPromise[T]) resolveToRejectedWithErr(err error, andHandle bool) (s uint32) {
	res := result.Err[T](err)
	return p.resolveToRejectedRes(res, andHandle)
}

func (p *GenericPromise[T]) uncaughtErrorHandler() {
	defUncaughtErrorHandler(p.res.Err())
}

func defUncaughtErrorHandler(err error) {
	panic(newUncaughtError(err).Error())
}

func (p *GenericPromise[T]) resolveToRejectedRes(res Result[T], andHandle bool) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	_, s = p.status.SetRejectedResolved()
	if andHandle {
		_, s = p.status.SetHandled() // set to Handled, if requested
	}
	close(p.resChan)

	// if the promise is rejected, and the chain is empty(no follow, read
	// or wait calls), execute the default uncaught panic handling logic.
	// otherwise, delay the handling of the uncaught error to the last call
	// in the chain.
	if status.IsChainEmpty(s) {
		p.uncaughtErrorHandler()
	}

	return
}

func (p *GenericPromise[T]) resolveToFulfilledRes(res Result[T], andHandle bool) (s uint32) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	_, s = p.status.SetFulfilledResolved()
	if andHandle {
		_, s = p.status.SetHandled() // set to Handled, if requested
	}
	close(p.resChan)

	return
}

func (p *GenericPromise[T]) panicSync(res Result[T]) {
	p.res = res
	p.status.SetPanickedResolvedSync()
}

func (p *GenericPromise[T]) rejectSync(res Result[T]) {
	p.res = res
	p.status.SetRejectedResolvedSync()
}

func (p *GenericPromise[T]) fulfillSync(res Result[T]) {
	p.res = res
	p.status.SetFulfilledResolvedSync()
}

// Catch waits the promise to be resolved, and calls the catchCb function,
// if the promise is resolved to rejected.
//
// It returns a GenericPromise value whose result will be the AnyRes value returned
// from the catchCb.
//
// The AnyRes value returned from the callback must not be modified after return.
//
// The catchCb is passed with three arguments, the error that caused this
// promise to be rejected, err, this promise's result, res(including the
// error at the end), and a boolean, ok, which will always be true.
//
// The result is passed here with the error, because, in Go, errors are
// just values, so returning them is not always considered an unwanted
// behavior(example: io.EOF error), so some logic may depend on both.
//
// Note that if the catchCb returned the res as-is, the returned promise
// will be rejected also(because there's a non-nil error at its end).
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GenericPromise[T]) Catch(catchCb func(val T, err error) Result[T]) Promise[T] {
	if catchCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newPromFollow[T](s)
	go newProm.catchCall(context.Background(), p, catchCb)
	return newProm
}

func (p *GenericPromise[T]) catchCall(
	ctx context.Context,
	prev *GenericPromise[T],
	cb catchCallback[T],
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	_, s := prev.wait(ctx)

	// 'Catch' can handle only the 'Rejected' state, so return otherwise
	if !status.IsStateRejected(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// mark the promise as 'Handled'.
	// the result value returned will hold the correct value that should be handled
	// by the callback.
	res, _ := p.handleFollow(prev.res, false)

	// run the callback with the actual promise result
	runCallback(p, res, s, cb)
}

// Recover waits the promise to be resolved, and calls the recoverCb function,
// if the promise is resolved to panicked.
//
// It returns a GenericPromise value whose result will be the AnyRes value returned
// from the recoverCb.
//
// The AnyRes value returned from the callback must not be modified after return.
//
// The recoverCb is passed with two arguments, the value that was passed
// to the 'panic' call which caused this promise to be panicked, v, and a
// boolean, ok, which will always be true.
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GenericPromise[T]) Recover(recoverCb func(v any) Result[T]) Promise[T] {
	if recoverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newPromFollow[T](s)
	go newProm.recoverCall(context.Background(), p, recoverCb)
	return newProm
}

func (p *GenericPromise[T]) recoverCall(
	ctx context.Context,
	prev *GenericPromise[T],
	cb recoverCallback[T],
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	_, s := prev.wait(ctx)

	// 'Recover' can handle only the 'Panicked' state, so return otherwise
	if !status.IsStatePanicked(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// mark the promise as 'Handled', and check whether we should continue or not.
	// the result value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := p.handleFollow(prev.res, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallback(p, res, s, cb)
}

// Finally waits the promise to be resolved, and calls the finallyCb function,
// regardless the promise is rejected, panicked, or neither.
//
// If the promise has panicked, it must be handled(either sequentially, or
// in-parallel) before calling this method, otherwise the promise will re-panic
// before this call.
//
// It returns a GenericPromise value, which will be resolved to this promise's result,
// if the finallyCb returned a nil AnyRes value.
// If the finallyCb returned a non-nil AnyRes value, the returned promise will be
// resolved to that returned AnyRes value.
//
// The AnyRes value returned from the callback must not be modified after return.
//
// The finallyCb is passed with one arguments, a boolean, ok, which will always
// be true.
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GenericPromise[T]) Finally(finallyCb func(s Status) Result[T]) Promise[T] {
	if finallyCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegWait()
	newProm := newPromFollow[T](s)
	go newProm.finallyCall(context.Background(), p, finallyCb)
	return newProm
}

// finallyCall is like an asyncReadCall, except that it can't set the 'Handled'
// flag(handle the promise), and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a fulfilled promise,
// and the panic will be dismissed implicitly, which is something we don't want.
func (p *GenericPromise[T]) finallyCall(
	ctx context.Context,
	prev *GenericPromise[T],
	cb finallyCallback[T],
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	_, s := prev.wait(ctx)

	// run the callback with the actual promise result
	runCallback(p, prev.res, s, cb)
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
	go p.asyncReadCall(context.Background(), cb, args)
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
	ctx context.Context,
	cb func(res Result[T], args []any),
	args []any,
) {
	// wait the previous promise to be resolved, as long as ctx is not done
	p.wait(ctx)

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
