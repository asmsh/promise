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
	"fmt"
	"time"
)

// Promise represents some asynchronous work. It offer ways to get the eventual
// result of that work, and/or build a computation pipeline assuming that result
// is known.
//
// The default implementation for this interface is the GoPromise type.
//
// It's a private interface, which can only be implemented by embedding any
// type that implement it from this module.
type Promise interface {
	// Status returns the status of this promise at this specific moment.
	//
	// The returned value corresponds only to the time it was created at.
	Status() Status

	// Wait waits the promise to be resolved. It returns false, if the promise
	// has panicked, otherwise it follows the rules of the underlying Promise
	// implementation.
	//
	// If the underlying Promise implementation is the GoPromise, and it's not
	// panicked, then ok will always = true.
	Wait() (ok bool)

	// WaitChan returns a newly created channel, which a boolean value will be
	// sent on, for only one time, after waiting the promise to be resolved.
	// The sent value correspond to the return of the Wait method.
	//
	// If it's called on a resolved promise, the value will be sent without
	// waiting.
	WaitChan() chan bool

	// GetRes waits the promise to be resolved, and returns its result, res,
	// and a boolean, ok, which will be true only if res is valid.
	//
	// The res value will be invalid(ok = false), if the promise has panicked,
	// otherwise it follows the rules of the underlying Promise implementation.
	//
	// If the underlying Promise implementation is the GoPromise, and it's not
	// panicked, then the res value will always be valid(ok = true).
	GetRes() (res Res, ok bool)

	// Delay returns a Promise value which will be resolved to this Promise(
	// by adopting its Res value, state, and fate), after a delay of at least
	// duration d. The delay starts after this Promise is resolved, accordingly.
	//
	// If this promise is resolved to fulfilled or pending, resolving the returned
	// promise will be delayed, only if onSucceed = true.
	//
	// If this promise is resolved to rejected or panicked, resolving the returned
	// promise will be delayed, only if onFail = true.
	//
	// If the promise is running in the safe mode(the default), the returned
	// Promise is a rejected promise, and the error is not caught(by a Catch call)
	// before the end of that promise's chain, a panic will happen with an error
	// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
	//
	// If the returned promise is a panicked promise, and it's not recovered(by a
	// Recover call) before the end of the promise's chain, or before calling Finally,
	// it will re-panic(with the same value passed to the original 'panic' call).
	Delay(d time.Duration, onSucceed, onFail bool) Promise

	// Then waits the promise to be resolved, and calls the thenCb function, if
	// the promise is resolved to fulfilled or pending(the promise didn't return
	// an error nor caused a panic).
	//
	// It returns a Promise value, which will be resolved to the Res value
	// returned from the thenCb.
	//
	// The Res value returned from the callback must not be modified after return.
	//
	// The thenCb is passed with two arguments, this promise's result, res, and
	// a boolean, ok, which will be true, only if res is valid.
	//
	// The res parameter will always be valid(and ok = true), if the underlying
	// Promise implementation is the GoPromise.
	//
	// The ok parameter's value(and the validity of other parameter) will
	// follow the rules of each of the other Promise implementations(other
	// than GoPromise), which are described at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Then(thenCb func(res Res, ok bool) Res) Promise

	// Catch waits the promise to be resolved, and calls the catchCb function,
	// if the promise is resolved to rejected(the promise returned an error).
	//
	// It returns a Promise value, which will be resolved to the Res value
	// returned from the catchCb.
	//
	// The Res value returned from the callback must not be modified after return.
	//
	// The catchCb is passed with three arguments, the error that caused this
	// promise to be rejected, err, this promise's result, res(including the
	// error at the end), and a boolean, ok, which will be true, only if err,
	// and res, are valid.
	//
	// The err, and the res parameters will always be valid(and ok = true), if
	// the underlying Promise implementation is the GoPromise.
	//
	// The ok parameter's value(and the validity of other parameters) will
	// follow the rules of each of the other Promise implementations(other
	// than GoPromise), which are described at their methods.
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
	// For more details, see 'Callback Notes' in the package comment.
	Catch(catchCb func(err error, res Res, ok bool) Res) Promise

	// Recover waits the promise to be resolved, and calls the recoverCb function,
	// if the promise is resolved to panicked(the promise caused a panic).
	//
	// It returns a Promise value, which will be resolved to the Res value
	// returned from the recoverCb.
	//
	// The Res value returned from the callback must not be modified after return.
	//
	// The recoverCb is passed with two arguments, the value that was passed
	// to the 'panic' call which caused this promise to be panicked, v, and a
	// boolean, ok, which will be true, only if v is valid.
	//
	// The v parameter will always be valid(and ok = true), if the underlying
	// Promise implementation is the GoPromise.
	//
	// The ok parameter's value(and the validity of other parameter) will
	// follow the rules of each of the other Promise implementations(other
	// than GoPromise), which are described at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Recover(recoverCb func(v interface{}, ok bool) Res) Promise

	// Finally waits the promise to be resolved, and calls the finallyCb function,
	// regardless the promise is rejected, panicked, or neither.
	//
	// If the promise has panicked, it must be handled(either sequentially, or
	// in-parallel) before calling this method, otherwise the promise will
	// re-panic before this call.
	//
	// It returns a Promise value, which will be resolved to this promise's
	// result, if the finallyCb returned a nil Res value.
	// If the finallyCb returned a non-nil Res value, the returned Promise
	// will be resolved to that returned Res value.
	//
	// The Res value returned from the callback must not be modified after return.
	//
	// The finallyCb is passed with one arguments, a boolean, ok, which will
	// always be true, if the underlying Promise implementation is the GoPromise,
	// and the promise hasn't panicked.
	// But, if the underlying Promise implementation is the GoPromise, and the
	// promise has panicked, ok will be false.
	//
	// The ok parameter's value will follow the rules of each of the other
	// Promise implementations(other than GoPromise), which are described
	// at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Finally(finallyCb func(ok bool) Res) Promise

	// this is a private interface that's specific to the different types and
	// functions in this module, and knows about them.
	privateImplementation()

	// the following method are/will be used internally to implement extension
	// functions in this module, which are functions that accepts promises and
	// extends their functionality or provide new functionality.
	asyncRead(cb func(res Res, ok bool, args []interface{}), args ...interface{})
	asyncFollow(cb func(res Res, ok bool, args []interface{}), args ...interface{})
}

// UnCaughtErr wraps an error that happened in a promise chain, but hasn't
// been caught, by the end of that chain.
type UnCaughtErr struct {
	err error
}

func (e *UnCaughtErr) Error() string {
	return fmt.Sprintf("UnCaught error in the promise chain: %v", e.err)
}

func (e *UnCaughtErr) Unwrap() error {
	return e.err
}

func newUnCaughtErr(err error) error {
	return &UnCaughtErr{err: err}
}
