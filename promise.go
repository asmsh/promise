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
	"time"

	"github.com/asmsh/promise/internal/status"
)

// GoPromise is the default implementation of the Promise interface
//
// The zero value will block forever on any calls.
type GoPromise struct {
	// holds the result of the promise.
	// written once, before the resChan channel is closed.
	//
	// don't read it unless the resChan is known to be closed, which can't be
	// known if the promise is resolved to 'pending', so don't read it then.
	res Res

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
	// note: not following the expected behaviour, either when passing the chan
	// or when resolving the promise, will result in a run-time panic, either
	// internally or in the creator side, and may introduce a data race on this
	// channel.
	resChan chan Res

	// hold the status of the different states, fates, flags, and other data
	// about the promise.
	// refer to the docs of the promStatus type for more info.
	//
	// the res field is guaranteed to be immutable, after the fate value is
	// Resolved or Handled, so don't read it before then.
	status status.PromStatus
}

// closedResChan is a closed channel that's shared between all instances
// returned from the 'sync' constructor below.
// this can be removed if we made the 'resolved' fate 'happens after' closing
// the resChan, so that we can depend on the value of the status for checking
// whether the promise is resolved or not.
var closedResChan = make(chan Res)

func init() {
	// closed once, as init func runs once
	close(closedResChan)
}

// internal type aliases for callback
type (
	thenCb    = func(res Res, ok bool) Res
	catchCb   = func(err error, res Res, ok bool) Res
	recoverCb = func(v interface{}, ok bool) Res
	finallyCb = func(ok bool) Res
	readCb    = func(res Res, ok bool, args []interface{})
)

// panic messages
const (
	nilCallbackPanicMsg   = "promise: The provided callback is nil"
	nilResChanPanicMsg    = "promise: The provided resChan is nil"
	multipleSendsPanicMsg = "promise: Only one send should be done on the resChan"
)

// newGoPromInter creates a new GoPromise which is resolved internally,
// using an internal allocated channel.
func newGoPromInter(nonSafe bool) *GoPromise {
	p := &GoPromise{
		resChan: make(chan Res),
	}

	// set the flags of the promise, accordingly
	if nonSafe {
		p.status = status.PromStatus(status.FlagsIsNotSafe)
	}
	return p
}

// newGoPromExter creates a new GoPromise which is resolved externally,
// using an external allocated channel, the passed resChan.
func newGoPromExter(resChan chan Res, nonSafe bool) *GoPromise {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	p := &GoPromise{
		resChan: resChan,
	}

	// set the flags of the promise, accordingly
	s := status.FlagsIsExternal
	if nonSafe {
		s |= status.FlagsIsNotSafe
	}
	p.status = status.PromStatus(s)
	return p
}

// newGoPromSync creates a new GoPromise which is resolved synchronously,
// just after it's created.
func newGoPromSync(nonSafe bool) *GoPromise {
	p := &GoPromise{resChan: closedResChan}

	// set the flags of the promise, accordingly
	if nonSafe {
		p.status = status.PromStatus(status.FlagsIsNotSafe)
	}
	return p
}

// Wait waits the promise to be resolved. It returns false, if the promise
// has panicked, otherwise it returns true.
func (p *GoPromise) Wait() (ok bool) {
	return p.waitCall(false, false, 0) // wait infinitely
}

// WaitChan returns a newly created channel, which true will be sent on,
// for only one time, after waiting the promise to be resolved.
//
// If it's called on a resolved promise, true will be sent without waiting.
func (p *GoPromise) WaitChan() chan bool {
	c := make(chan bool, 1) // buffered, so that the goroutine always exit
	go func(c chan bool) {
		ok := p.Wait()
		c <- ok
	}(c)
	return c
}

// WaitUntil waits the promise to be resolved, with max wait time, d.
// It returns false, if the promise has panicked, or the wait timed-out,
// otherwise it returns true.
//
// If the wait duration, d, is 0 or less, it will be like in an Wait call.
func (p *GoPromise) WaitUntil(d time.Duration) (ok bool) {
	// reset to 0 if d is negative, as they are equal in the API,
	// and don't pass negative duration, as it's special, internally
	if d < 0 {
		d = 0
	}
	return p.waitCall(false, false, d) // wait up to duration d
}

// GetRes waits the promise to be resolved, and returns its result, res,
// and ok = true, if the promise hasn't panicked, otherwise it returns
// res = nil, and ok = false.
func (p *GoPromise) GetRes() (res Res, ok bool) {
	return p.GetResUntil(0) // wait infinitely
}

// GetResUntil waits the promise to be resolved, with max wait time, d,
// and returns its result, res, and ok = true, if the promise hasn't
// panicked, nor the wait timed-put, otherwise it returns res = nil,
// and ok = false.
func (p *GoPromise) GetResUntil(d time.Duration) (res Res, ok bool) {
	// reset to 0 if d is negative, as they are equal in the API,
	// and don't pass negative duration, as it's special, internally
	if d < 0 {
		d = 0
	}
	ok = p.waitCall(true, false, d) // wait up to duration d
	if !ok {
		return
	}

	// return a copy of the result, to keep the saved one immutable
	res = copyRes(p.res)
	return
}

// ok priority: panics > timeout > onetime
func (p *GoPromise) waitCall(resCall, once bool, d time.Duration) (ok bool) {
	// wait the promise to be resolved, with wait time set to d
	timedout := p.wait(d, false)

	// if the promise has panicked, or the wait timed-out, return false
	s := p.status.Read()
	if status.IsStatePanicked(s) || timedout {
		return false
	}

	// set the flags and update the ok value, accordingly
	if resCall {
		// if the caller need the result, set the handled flag
		ok, _ = p.status.SetHandled()
	} else {
		// otherwise, don't set it, but use its value anyway
		ok = !status.IsFateHandled(s)
	}
	if !once && !ok {
		// ignore the handled flag result if the promise isn't onetime
		ok = true
	}
	return
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
func (p *GoPromise) wait(d time.Duration, resolveOnTimeout bool) (timeout bool) {
	s := p.status.Read()

	// if the fate is Resolved or Handled don't wait, as they are guaranteed
	// to happen after the result is saved, and after the resChan is closed.
	if status.IsFateResolved(s) || status.IsFateHandled(s) {
		// panic if the resChan has any elements, as the promise is Resolved,
		// and the result of the promise is already received.
		// this will be true, only if the resChan is created externally, and
		// more than one value is sent on it, which is considered a bad usage.
		if len(p.resChan) != 0 {
			panic(multipleSendsPanicMsg)
		}

		return false
	}

	// create the timer chan if a duration is passed
	var t *time.Timer
	var tc <-chan time.Time
	if d > 0 {
		t = time.NewTimer(d)
		tc = t.C
	}

	// wait with the appropriate wait procedure
	if status.IsFlagsExternal(s) {
		timeout = p.exterWaitProc(tc, resolveOnTimeout)
	} else {
		timeout = p.interWaitProc(tc, resolveOnTimeout)
	}

	// if a timer is used, stop it to free its resources
	if t != nil {
		t.Stop()
	}
	return
}

// exterWaitProc executes the wait procedure responsible for promises with
// externally allocated resChan.
// this method will be called in only one specific case, when the promise
// is created with the external flag set.
// it returns true only if the provided timerChan is received from, otherwise
// it returns false.
func (p *GoPromise) exterWaitProc(timerChan <-chan time.Time, resolveOnTimeout bool) (timeout bool) {
	select {
	case res, ok := <-p.resChan: // the chan is closed or a value is received
		if ok {
			// set the status to Resolving, and handle the result only if
			// it's just set, otherwise panic, accordingly.
			set, s := p.status.SetResolving()
			if set {
				// handle the (only one) result received, accordingly.
				//
				// this will close the chan, regardless it's already closed or
				// not, but as it's required that the chan be used to do only
				// one operation on it, either a single send or a close, and as
				// it's closed when a send happen, and only one send will ever
				// result in reaching this point, panicking for a second close
				// call, either internally or externally(in the user's code) is
				// allowed, and considered a result for a bad usage of the chan.
				p.resolveToRes(res)
			} else if !(status.IsFateResolved(s) && status.IsStatePending(s)) {
				// the only allowed reason for not setting the fate to Resolving
				// here, is that the promise is resolved to pending, because of
				// a timeout, otherwise..
				//
				// this may happen if two or more values are sent, and the
				// first value is processed and this is the second value.
				// only one value should be sent, but more is sent, panic.
				panic(multipleSendsPanicMsg)
			}
		} else {
			// the chan is closed, either by us or by the user, set the status
			// to fulfilled and resolved.
			//
			// if the chan is closed by us, then the res and status fields are
			// now set as expected, and attempting to set them again will not
			// succeed, and that's acceptable.
			// if it's closed by the user, which is considered acceptable, only
			// if the user didn't send any values and just closed the chan, the
			// res field will be empty, cause the user didn't send any values,
			// and the status will be empty, so set it to fulfilled and resolve,
			// cause no reason to reject can happen.
			p.status.SetFulfilledResolved()
		}

		return false
	case <-timerChan:
		if resolveOnTimeout {
			// set the status to resolved here, without closing the channel,
			// as it's used externally and a value maybe sent on it later.
			p.status.SetPendingResolved()
		}
		return true
	}
}

// interWaitProc executes the wait procedure responsible for promises with
// internally allocated resChan.
// it returns true only if the provided timerChan is received from, otherwise
// it returns false.
func (p *GoPromise) interWaitProc(timerChan <-chan time.Time, resolveOnTimeout bool) (timeout bool) {
	select {
	case <-p.resChan:
		// internally created res chan will always be closed by the previous
		// promise, after setting the res and status fields as expected.
		return false
	case <-timerChan:
		if resolveOnTimeout {
			// resolve to pending, only if it's required
			p.resolveToPending()
		}
		return true
	}
}

// asyncRead is used internally to implement promise extension functions.
//
// as it's registered as a 'read' call, it can prevent UnCaughtErr panics if
// the promise is about to be rejected, but it can't prevent a panicked promise
// from re-broadcasting the panic.
//
// the cb will be called with ok = true, only if the promise is fulfilled
// or rejected.
func (p *GoPromise) asyncRead(cb func(res Res, ok bool, args []interface{}), args ...interface{}) {
	// register this as a 'read' call, and return if no callback is passed
	p.status.RegRead()
	if cb == nil {
		return
	}

	// asynchronously, wait the promise to be resolved and call cb, accordingly
	go p.asyncReadCall(cb, args, false, 0)
}

// asyncReadCall will call the cb with argument that corresponds to the result
// of a GetRes call.
//
// if the promise panics without having a 'follow' call, then the program will
// crash, so if the promise' state is found to be panicked, here, this means
// that, either it's already handled, or there are one or more follow calls
// in the chain that might handle it, and if the panic reached the end of the
// chain without finding a Recover call, the promise will re-broadcast the
// panic, so there's no need to care about panics here.
func (p *GoPromise) asyncReadCall(cb readCb, args []interface{}, once bool, d time.Duration) {
	// wait the promise to be resolved
	p.wait(d, false)

	// run the callback, regardless the previous promise is pending, fulfilled,
	// rejected, or panicked.
	if s := p.status.Read(); status.IsStatePending(s) || status.IsStatePanicked(s) {
		// the previous promise has timedout, or panicked, run the callback
		// with a nil result and ok = false, as if it timedout, the promise
		// was timed, and if it's panicked, await can't handle panics.
		cb(nil, false, args)
	} else {
		// the previous promise is fulfilled, or rejected, but both of them
		// can be handled through await, so set the handled flag, as the
		// promise is about to be handled, and update ok accordingly.
		ok, _ := p.status.SetHandled()
		if !once && !ok {
			// ignore the handled flag result if the promise isn't onetime
			ok = true
		}
		// run the callback, accordingly
		if !ok {
			// the promise is a onetime promise and has already been handled
			cb(nil, false, args)
		} else {
			// pass the result of promise without coping it, but don't forget
			// to copy it if the result will be passed to any user-facing calls.
			cb(p.res, ok, args)
		}
	}
}

// asyncFollow is used internally to implement promise extension functions.
//
// as it's registered as a 'follow' call, it can prevent UnCaughtErr panics if
// the promise is about to be rejected, and also prevent a panicked promise from
// re-broadcasting the panic.
//
// the cb will be called with ok = true, only if the promise is fulfilled
// or rejected.
func (p *GoPromise) asyncFollow(cb func(res Res, ok bool, args []interface{}), args ...interface{}) {
	// register this as a 'follow' call, and return if no callback is passed
	p.status.RegFollow()
	if cb == nil {
		return
	}
}

// Then waits the promise to be resolved, and calls the thenCb function, if
// the promise is resolved to fulfilled or pending.
//
// It returns a GoPromise value whose result will be the Res value returned
// from the thenCb.
//
// The Res value returned from the callback must not be modified after return.
//
// The thenCb is passed with two arguments, this promise's result, res, and
// a boolean, ok, which will always be true.
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GoPromise) Then(thenCb func(res Res, ok bool) Res) Promise {
	if thenCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newGoPromInter(status.IsFlagsNotSafe(s))
	go newProm.thenCall(p, thenCb, false, 0)
	return newProm
}

func (p *GoPromise) thenCall(prev *GoPromise, cb thenCb, once bool, d time.Duration) {
	// wait the previous promise to be resolved, with max time d
	prev.wait(d, true)

	// ok is initially true and will be updated if the state is pending,
	// which mean that the promise has timedout(either now or before).
	ok := true

	// then can handle 'fulfilled' & 'pending' states, so return otherwise
	if s := prev.status.Read(); status.IsStatePending(s) {
		ok = false
	} else if !status.IsStateFulfilled(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// if the previous promise hasn't timedout and is fulfilled, set the handled
	// flag, as the promise is about to be handled, and update ok accordingly.
	if ok {
		ok, _ = prev.status.SetHandled()
		if !once && !ok {
			// ignore the handled flag result if the promise isn't onetime
			ok = true
		}
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Res)
	defer p.handleReturns(resP)

	// run the callback, accordingly
	if !ok {
		// the wait has timedout, or the promise is a onetime promise
		// and has already been handled.
		*resP = cb(nil, false)
	} else {
		// pass a copy of the previous result, to keep it immutable
		prevRes := copyRes(prev.res)
		*resP = cb(prevRes, true)
	}
}

// this handles invalid follow from then, catch, and recover calls.
// for any promise, this is guaranteed to be called only once, as it's called
// with the newly created promise as a receiver.
func (p *GoPromise) handleInvalidFollow(prevRes Res, prevStatus uint32) {
	if status.IsStatePending(prevStatus) {
		// the previous promise is pending, resolve to timeout
		p.resolveToPending()
	} else if status.IsStateFulfilled(prevStatus) {
		// the previous promise is fulfilled, fulfill with its result
		p.fulfill(prevRes)
	} else if status.IsStateRejected(prevStatus) {
		// the previous promise is rejected, reject with its result
		p.reject(prevRes)
	} else if status.IsStatePanicked(prevStatus) {
		// the previous promise is panicked, panic with its result
		p.resolveToPanic(prevRes)
	}
}

// handleReturns must be deferred.
// the callback function is called after a deferred call to this method.
// no internal call that may cause a panic should be called after this method.
func (p *GoPromise) handleReturns(resP *Res) {
	// make sure that only one call will resolve the promise, or return if
	// the promise is already resolved, so that we don't recover panics when
	// we don't need to.
	set, _ := p.status.SetResolving()
	if !set {
		return
	}

	v := recover()
	if v == nil {
		// no panic, resolve to the Res value pointed to, as the callback is
		// either returned normally, or through a call to runtime.Goexit call,
		// and in either case the Res value pointed to will not change here.
		var res Res
		if resP != nil {
			res = *resP
		}
		p.resolveToRes(res)
	} else {
		// a panic happened, resolve to panicked, with the panic value(inside
		// a Res value).
		p.resolveToPanic(Res{v})
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
// once once on the same promise, by design.
func (p *GoPromise) resolveToRes(res Res) {
	if res.IsErrRes() {
		p.reject(res)
	} else {
		p.fulfill(res)
	}
}

// if called from handleInvalidFollow, then it will be called once on the
// same promise, by design.
// if called from handleReturns, then it will be called once on the same
// promise, as it's called on return, and that can happen once.
func (p *GoPromise) resolveToPanic(res Res) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	_, s := p.status.SetPanickedResolved()
	close(p.resChan)

	// re-broadcast the panic if the promise is not followed
	if !status.IsChainModeFollow(s) {
		panic(res[0])
	}
}

func (p *GoPromise) panicSync(res Res) {
	p.res = res
	p.status.SetPanickedResolvedSync()
}

func (p *GoPromise) reject(res Res) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	_, s := p.status.SetRejectedResolved()
	close(p.resChan)

	// panic if the safe mode is enabled(the default), and the promise
	// is not followed, nor has any calls that can prevent such panics.
	if !status.IsFlagsNotSafe(s) && status.IsChainEmpty(s) {
		err := res.GetErr()
		panic(newUnCaughtErr(err))
	}
}

func (p *GoPromise) rejectSync(res Res) {
	p.res = res
	p.status.SetRejectedResolvedSync()
}

func (p *GoPromise) fulfill(res Res) {
	// save the result, update the status, and close the resChan to unblock
	// all waiting calls.
	p.res = res
	p.status.SetFulfilledResolved()
	close(p.resChan)
}

func (p *GoPromise) fulfillSync(res Res) {
	p.res = res
	p.status.SetFulfilledResolvedSync()
}

// resolveToPending sets the state to Pending, and the fate to Resolved, then
// close the resChan, only if the promise isn't Resolved.
//
// it is needed for the TimedPromise, and maybe called more than one time on
// the same promise, after the promise's wait times-out.
func (p *GoPromise) resolveToPending() {
	set, _ := p.status.SetPendingResolved()

	// return if the promise is already resolved, to keep the following code
	// executed once.
	if !set {
		return
	}

	// close the resChan to unblock all waiting calls
	close(p.resChan)
}

// Catch waits the promise to be resolved, and calls the catchCb function,
// if the promise is resolved to rejected.
//
// It returns a GoPromise value whose result will be the Res value returned
// from the catchCb.
//
// The Res value returned from the callback must not be modified after return.
//
// The catchCb is passed with three arguments, the error that caused this
// promise to be rejected, err, this promise's result, res(including the
// error at the end), and a boolean, ok, which will always be true.
//
// The result is passed here with the error, because, in Go, errors are
// just values, so returning them is not always considered an unwanted
// behaviour(example: io.EOF error), so some logic may depend on both.
//
// Note that if the catchCb returned the res as-is, the returned promise
// will be rejected also(because there's a non-nil error at its end).
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GoPromise) Catch(catchCb func(err error, res Res, ok bool) Res) Promise {
	if catchCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newGoPromInter(status.IsFlagsNotSafe(s))
	go newProm.catchCall(p, catchCb, false, 0)
	return newProm
}

func (p *GoPromise) catchCall(prev *GoPromise, cb catchCb, once bool, d time.Duration) {
	// wait the previous promise to be resolved, with max time d
	prev.wait(d, true)

	// catch can handle only 'rejected' state, so if the previous promise
	// has timedout, the state can't be 'rejected', so handle it accordingly.
	if s := prev.status.Read(); !status.IsStateRejected(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// the previous promise hasn't timedout but is rejected, set the handled
	// flag, as the promise is about to be handled, and update ok accordingly.
	ok, _ := prev.status.SetHandled()
	if !once {
		// ignore the handled flag result if the promise isn't onetime
		ok = true
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Res)
	defer p.handleReturns(resP)

	// run the callback, accordingly
	if !ok {
		// the promise is a onetime promise and has already been handled
		*resP = cb(nil, nil, false)
	} else {
		// get the error responsible for rejecting the previous promise
		err := prev.res.GetErr()

		// pass a copy of the previous result, to keep it immutable
		prevRes := copyRes(prev.res)
		*resP = cb(err, prevRes, true)
	}
}

// Recover waits the promise to be resolved, and calls the recoverCb function,
// if the promise is resolved to panicked.
//
// It returns a GoPromise value whose result will be the Res value returned
// from the recoverCb.
//
// The Res value returned from the callback must not be modified after return.
//
// The recoverCb is passed with two arguments, the value that was passed
// to the 'panic' call which caused this promise to be panicked, v, and a
// boolean, ok, which will always be true.
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GoPromise) Recover(recoverCb func(v interface{}, ok bool) Res) Promise {
	if recoverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegFollow()
	newProm := newGoPromInter(status.IsFlagsNotSafe(s))
	go newProm.recoverCall(p, recoverCb, false, 0)
	return newProm
}

func (p *GoPromise) recoverCall(prev *GoPromise, cb recoverCb, once bool, d time.Duration) {
	// wait the previous promise to be resolved, with max time d
	prev.wait(d, true)

	// recover can handle only 'panicked' state, so if the previous promise
	// has timedout, the state can't be 'panicked', so handle it accordingly.
	if s := prev.status.Read(); !status.IsStatePanicked(s) {
		p.handleInvalidFollow(prev.res, s)
		return
	}

	// the previous promise hasn't timedout and is panicked, set the handled
	// flag, as the promise is about to be handled, and update ok accordingly.
	ok, _ := prev.status.SetHandled()
	if !once && !ok {
		// ignore the handled flag result if the promise isn't onetime
		ok = true
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Res)
	defer p.handleReturns(resP)

	// run the callback, accordingly
	if !ok {
		// the promise is a onetime promise and has already been handled
		*resP = cb(nil, false)
	} else {
		*resP = cb(prev.res[0], true)
	}
}

// Finally waits the promise to be resolved, and calls the finallyCb function,
// regardless the promise is rejected, panicked, or neither.
//
// If the promise has panicked, it must be handled(either sequentially, or
// in-parallel) before calling this method, otherwise the promise will re-panic
// before this call.
//
// It returns a GoPromise value, which will be resolved to this promise's result,
// if the finallyCb returned a nil Res value.
// If the finallyCb returned a non-nil Res value, the returned promise will be
// resolved to that returned Res value.
//
// The Res value returned from the callback must not be modified after return.
//
// The finallyCb is passed with one arguments, a boolean, ok, which will always
// be true.
//
// It will panic if a nil callback is passed.
//
// For more, see 'Callback Notes' in the package comment.
func (p *GoPromise) Finally(finallyCb func(ok bool) Res) Promise {
	if finallyCb == nil {
		panic(nilCallbackPanicMsg)
	}

	_, s := p.status.RegRead()
	newProm := newGoPromInter(status.IsFlagsNotSafe(s))
	go newProm.finallyCall(p, finallyCb, false, 0)
	return newProm
}

// finallyCall is like an asyncReadCall, except that it can't set the 'Handled'
// flag(handle the promise), and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a fulfilled promise,
// and the panic will be dismissed implicitly, which is something we don't want.
func (p *GoPromise) finallyCall(prev *GoPromise, cb finallyCb, once bool, d time.Duration) {
	// wait the previous promise to be resolved, with max time d
	prev.wait(d, true)

	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Res)
	defer p.handleReturns(resP)

	// run the callback, regardless the previous promise is pending, fulfilled,
	// rejected, or panicked, then resolve according to the returned value.
	if s := prev.status.Read(); status.IsStatePanicked(s) {
		res := cb(false)
		if res == nil {
			// a nil Res value is returned, don't resolve to the previous Res,
			// and resolve fulfill, with nil Res, as the panic must have been
			// already handled(otherwise the program would have crashed and we
			// would never reach this).
			p.fulfill(nil)
		} else {
			// save the new result
			*resP = res
		}
	} else if status.IsStatePending(s) {
		res := cb(false)
		if res == nil {
			// resolve to the previous promise(pending state)
			p.resolveToPending()
		} else {
			// save the new result
			*resP = res
		}
	} else { // fulfilled, or rejected
		// set the 'call' flag of the 'finally' callback(as it's about to
		// be called), and update ok accordingly.
		ok, _ := prev.status.SetCalledFinally()
		if !once && !ok {
			// ignore the handled flag result if the promise isn't onetime
			ok = true
		}

		// run the callback, then resolve according to the returned value
		res := cb(ok)
		if res == nil {
			// resolve to the previous promise(the previous result)
			p.resolveToRes(prev.res)
		} else {
			// save the new result
			*resP = res
		}
	}
}

func (*GoPromise) privateImplementation() {}

func copyRes(in Res) Res {
	n := len(in)
	if n == 0 {
		return in
	}

	out := make(Res, n)
	copy(out, in)
	return out
}
