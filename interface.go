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
)

// Promise represents some asynchronous work. It offer ways to get the eventual
// result of that work, and/or build a computation pipeline assuming that result
// is known.
//
// The default implementation for this interface is the GoPromise type.
//
// It's a private interface, which can only be implemented by embedding any
// type that implement it from this module.
type Promise[T any] interface {
	// Result is the result of this Promise, once it's resolved.
	// Accessing any of its methods will block until the Promise is resolved.
	// Having the Promise implementing the Result means that it can be returned
	// from a callback.
	Result[T]

	// Wait waits the promise to be resolved. It returns false, if the promise
	// has panicked, otherwise it follows the rules of the underlying Promise
	// implementation.
	//
	// If the underlying Promise implementation is the genericPromise, and it's not
	// panicked, then ok will always = true.
	Wait()

	// WaitChan returns a newly created channel, which a boolean value will be
	// sent on, for only one time, after waiting the promise to be resolved.
	// The sent value correspond to the return of the Wait method.
	//
	// If it's called on a resolved promise, the value will be sent without
	// waiting.
	WaitChan() chan struct{}

	// Res waits the promise to be resolved, and returns its result, res,
	// and a boolean, ok, which will be true only if res is valid.
	//
	// The res value will be invalid(ok = false), if the promise has panicked,
	// otherwise it follows the rules of the underlying Promise implementation.
	//
	// If the underlying Promise implementation is the genericPromise, and it's not
	// panicked, then the res value will always be valid(ok = true).
	Res() Result[T]

	// TODO: ??
	// ResChan() <-chan Result[T]

	Callback(cb func(ctx context.Context, res Result[T]))

	// Delay returns a Promise value which will be resolved to this Promise(
	// by adopting its Res value, state, and fate), after a delay of at least
	// duration d. The delay starts after this Promise is resolved, accordingly.
	//
	// If this promise is resolved to fulfilled or pending, resolving the returned
	// promise will be delayed, only if onSuccess = true.
	//
	// If this promise is resolved to rejected or panicked, resolving the returned
	// promise will be delayed, only if onError = true.
	//
	// If the promise is running in the safe mode(the default), the returned
	// Promise is a rejected promise, and the error is not caught(by a Catch call)
	// before the end of that promise's chain, a panic will happen with an error
	// value of type *UncaughtError, which has that uncaught error 'wrapped' inside it.
	//
	// If the returned promise is a panicked promise, and it's not recovered(by a
	// Recover call) before the end of the promise's chain, or before calling Finally,
	// it will re-panic(with the same value passed to the original 'panic' call).
	Delay(d time.Duration, cond ...DelayCond) Promise[T]

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
	// Promise implementation is the genericPromise.
	//
	// The ok parameter's value(and the validity of other parameter) will
	// follow the rules of each of the other Promise implementations(other
	// than genericPromise), which are described at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Then(thenCb func(ctx context.Context, val T) Result[T]) Promise[T]

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
	// the underlying Promise implementation is the genericPromise.
	//
	// The ok parameter's value(and the validity of other parameters) will
	// follow the rules of each of the other Promise implementations(other
	// than genericPromise), which are described at their methods.
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
	Catch(catchCb func(ctx context.Context, val T, err error) Result[T]) Promise[T]

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
	// Promise implementation is the genericPromise.
	//
	// The ok parameter's value(and the validity of other parameter) will
	// follow the rules of each of the other Promise implementations(other
	// than genericPromise), which are described at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Recover(recoverCb func(ctx context.Context, v any) Result[T]) Promise[T]

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
	// always be true, if the underlying Promise implementation is the genericPromise,
	// and the promise hasn't panicked.
	// But, if the underlying Promise implementation is the genericPromise, and the
	// promise has panicked, ok will be false.
	//
	// The ok parameter's value will follow the rules of each of the other
	// Promise implementations(other than genericPromise), which are described
	// at their methods.
	//
	// It will panic if a nil callback is passed.
	//
	// For more details, see 'Callback Notes' in the package comment.
	Finally(finallyCb func(ctx context.Context)) Promise[T]

	// this is a private interface that's specific to the different types and
	// functions in this module, and knows about them.
	privateImplementation()

	impl() *genericPromise[T]
}

type State int

const (
	// the order here matter
	unknown State = iota
	Fulfilled
	Rejected
	Panicked
)

func (s State) String() string {
	switch s {
	case Fulfilled:
		return "fulfilled"
	case Rejected:
		return "rejected"
	case Panicked:
		return "panicked"
	default:
		return "<unknown>"
	}
}
