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

import "time"

// Go runs the provided function, fun, in a separate goroutine, and returns
// a GoPromise whose result is a nil Res value.
//
// The returned promise will be resolved to panicked, or fulfilled, depending
// on whether fun caused a panic, or returned normally, respectively.
//
// If the callback called runtime.Goexit, the returned promise will be fulfilled
// to 'nil'.
//
// If the returned promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
// It will panic if a nil function is passed.
func Go(fun func()) *GoPromise {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(false)
	go goCall(prom, fun)
	return prom
}

func goCall(p *GoPromise, fun func()) {
	// defer the return handler to handle panics and runtime.Goexit calls.
	defer p.handleReturns(nil)
	// call the go func, then fulfill and return
	fun()
}

// GoRes runs the provided function, fun, in a separate goroutine, and returns
// a GoPromise whose result will be the Res value returned from fun.
//
// The returned promise will be resolved to panicked, rejected, or fulfilled,
// depending on whether fun caused a panic, fun returned a non-nil error as the
// last element in the returned Res value, or fun returned a Res value which
// doesn't has a non-nil error as its last element, respectively.
//
// If the callback called runtime.Goexit, the returned promise will be fulfilled
// to 'nil'.
//
// If the returned promise is rejected, and the error isn't caught(by a Catch
// call) before the end of the promise's chain, a panic will happen with an error
// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
//
// If the returned promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
// It will panic if a nil function is passed.
func GoRes(fun func() Res) *GoPromise {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(false)
	go goResCall(prom, fun)
	return prom
}

func goResCall(p *GoPromise, fun func() Res) {
	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Res)
	defer p.handleReturns(resP)
	// call the go func and get its result
	*resP = fun()
}

// New returns a GoPromise that's created using the provided resChan.
//
// The resChan must be a bi-directional chan(buffered or unbuffered), which is
// used from the caller(creator) side to do only one of the following, either
// send a Res value on it for one time, or just close it.
//
// When a Res value is sent on it, the returned promise will be resolved(and the
// resChan is closed) to either rejected, or fulfilled, depending on whether the
// last element in that sent Res value is a non-nil error, or not, respectively.
// When the resChan is closed, the returned promise will be fulfilled, and its
// result will be a nil Res value.
//
// If the resChan is not usd as described above, a panic will happen when it's
// used, either on the caller side, or here, internally.
//
// If the returned promise is rejected, and the error isn't caught by the end
// of the promise's chain(by a Catch call), a panic will happen with an error
// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
func New(resChan chan Res) *GoPromise {
	prom := newGoPromExter(resChan, false)
	return prom
}

// Resolver provides a JavaScript-like promise creation. It runs the provided
// resolverCb function in a separate goroutine, and returns a new GoPromise,
// which will be resolved using the functions passed to the resolverCb.
//
// The returned promise will be fulfilled when the fulfill function is called,
// for the first time, and before the reject function, then the promise's result
// will be a Res value that contains the passed values, vals.
// Or, when calling the reject function, but with a nil error, and in that case
// the promise's result will be a Res value that contains the passed values, vals,
// and a nil, after them at the end.
//
// The returned promise will be rejected when the reject function is called
// with a non nil error, for the first time, and before the fulfill function,
// then the promise's result will be a Res value that contains, both, the passed
// values, vals(if present), and the provided err(after vals, always at the end).
//
// The returned promise will be panicked when the resolverCb causes a panic,
// before any calls to fulfill or reject is made.
//
// If the callback called runtime.Goexit, before any calls to fulfill or reject
// is made, the returned promise will be fulfilled to 'nil'.
//
// If the resolverCb causes a panic after calling fulfill or reject, the panic
// will not be recovered by the promise.
//
// The fulfill and reject functions should not be called asynchronously, or more
// specifically, they should be called and returned before the resolverCb return.
//
// The provided vals, if passed from a slice/array, the slice/array shouldn't
// be modified after this call.
//
// If the returned promise is rejected, and the error isn't caught(by a Catch
// call) before the end of the promise's chain, a panic will happen with an error
// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
//
// If the returned promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
// It will panic if a nil function is passed.
func Resolver(resolverCb func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{}))) *GoPromise {
	if resolverCb == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(false)
	go resolverCall(prom, resolverCb)
	return prom
}

func resolverCall(p *GoPromise, cb func(fulfill func(...interface{}), reject func(error, ...interface{}))) {
	// defer the return handler to handle panics and runtime.Goexit calls
	defer p.handleReturns(nil)

	// create the resolver functions, and pass them to the callback
	fulfill := func(vals ...interface{}) {
		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call(from fulfill or reject) will reach this point
		p.resolveToRes(vals)
	}
	reject := func(err error, vals ...interface{}) {
		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call(from fulfill or reject) will reach this point
		p.resolveToRes(append(vals, err))
	}
	cb(fulfill, reject)
}

// Delay returns a GoPromise that's resolved to the passed Res value, res,
// after waiting for, at least, the passed duration, d, accordingly.
//
// The returned promise is resolved to rejected, or fulfilled, depending on
// whether the last element in res is, respectively, a non-nil error value,
// or any other value.
//
// If the promise is about to be fulfilled, resolving the promise will only
// be delayed, if onSucceed = true.
// If the promise is about to be rejected, resolving the promise will only
// be delayed, if onFail = true.
//
// The provided res value shouldn't be modified after this call.
//
// If the returned promise is rejected, and the error isn't caught by the end
// of the promise's chain(by a Catch call), a panic will happen with an error
// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
func Delay(res Res, d time.Duration, onSucceed, onFail bool) *GoPromise {
	prom := newGoPromInter(false)
	go delayCall(prom, res, d, onSucceed, onFail)
	return prom
}

// handles rejection and fulfillment only
func delayCall(p *GoPromise, res Res, d time.Duration, onSucceed, onFail bool) {
	if res.IsErrRes() {
		// a rejected state is considered a failure
		if onFail {
			time.Sleep(d)
		}
		p.resolveToReject(res, false)
	} else {
		// a fulfilled state is considered a success
		if onSucceed {
			time.Sleep(d)
		}
		p.resolveToFulfill(res, false)
	}
}

// Wrap returns a GoPromise that's resolved, synchronously, to the provided
// Res value, res.
//
// The returned promise is resolved to rejected, or fulfilled, depending on
// whether the last element in res is, respectively, a non-nil error value,
// or any other value.
//
// The provided res value shouldn't be modified after this call.
//
// If the returned promise is rejected, it will not panic with an *UnCaughtErr
// error, but all subsequent promises in any promise chain derived from it will,
// until the error is caught on each of these chains(by a Catch call).
func Wrap(res Res) *GoPromise {
	prom := newGoPromSync(false)
	wrapCall(prom, res)
	return prom
}

func wrapCall(p *GoPromise, res Res) {
	if res.IsErrRes() {
		p.rejectSync(res)
	} else {
		p.fulfillSync(res)
	}
}

// Fulfill returns a GoPromise that's fulfilled, synchronously, and whose result
// is a Res value that contains the passed values, vals(if present).
//
// The provided vals, if passed from a slice/array, the slice/array shouldn't be
// modified after this call.
func Fulfill(vals ...interface{}) *GoPromise {
	prom := newGoPromSync(false)
	prom.fulfillSync(vals)
	return prom
}

// Reject returns a GoPromise that's rejected, synchronously, and whose result
// is a Res value that contains, both, the passed values, vals(if present), and
// the provided error, err(after vals, at the end), only if err is a non-nil
// error value.
//
// If err is a nil error value, it returns a GoPromise that's fulfilled,
// synchronously, and whose result is a Res value that contains vals.
//
// The provided vals, if passed from a slice/array, the slice/array shouldn't
// be modified after this call.
//
// The returned promise, if rejected, will not panic with an *UnCaughtErr error,
// but all subsequent promises in any promise chain derived from it will, until
// the error is caught on each of these chains(by a Catch call).
func Reject(err error, vals ...interface{}) *GoPromise {
	prom := newGoPromSync(false)
	// fulfill if err is nil, so that Catch is never called with a nil error
	if err == nil {
		prom.fulfillSync(append(vals, err))
		return prom
	}
	prom.rejectSync(append(vals, err))
	return prom
}

// Panic returns a GoPromise that's resolved to panicked, synchronously, and
// whose result is the passed value, v.
//
// The passed value, v, is only accessible from a Recover callback.
//
// All subsequent promises in any promise chain derived from the returned
// promise needs to call Recover, before the end of each promise's chain,
// otherwise all these promise will re-panic(with the passed value, v).
func Panic(v interface{}) *GoPromise {
	prom := newGoPromSync(false)
	prom.panicSync(Res{v})
	return prom
}
