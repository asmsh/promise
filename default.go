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
)

var (
	defUncaughtPanicHandler = func(v any) {
		panic(newUncaughtPanic(v).Error())
	}
	defUncaughtErrorHandler = func(err error) {
		panic(newUncaughtError(err).Error())
	}

	defaultPipelineConfig = PipelineConfig{
		UncaughtPanicHandler: defUncaughtPanicHandler,
		UncaughtErrHandler:   defUncaughtErrorHandler,
	}

	defaultPipeline = NewPipeline[any](defaultPipelineConfig)
)

// AnyPromise is the default type
type AnyPromise = Promise[any]

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
func Go(fun func()) AnyPromise {
	return defaultPipeline.Go(fun)
}

func GoErr(fun func() error) AnyPromise {
	return defaultPipeline.GoErr(fun)
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
// If the returned promise is rejected, and the error is not caught(by a Catch
// call) before the end of the promise's chain, or the promise result is not
// read(by a Res call), a panic will happen with an error value of type
// *UncaughtError, which has that uncaught error 'wrapped' inside it.
//
// If the returned promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
// It will panic if a nil function is passed.
func GoRes(fun func() Result[any]) AnyPromise {
	return defaultPipeline.GoRes(fun)
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
// If the returned promise is rejected, and the error is not caught(by a Catch
// call) before the end of the promise's chain, or the promise result is not
// read(by a Res call), a panic will happen with an error value of type
// *UncaughtError, which has that uncaught error 'wrapped' inside it.
func New(resChan chan Result[any]) AnyPromise {
	return defaultPipeline.New(resChan)
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
// FIXME: we should lift the blow restriction
// The fulfill and reject functions should not be called asynchronously, or more
// specifically, they should be called and returned before the resolverCb return.
//
// The provided vals, if passed from a slice/array, the slice/array shouldn't
// be modified after this call.
//
// If the returned promise is rejected, and the error is not caught(by a Catch
// call) before the end of the promise's chain, or the promise result is not
// read(by a Res call), a panic will happen with an error value of type
// *UncaughtError, which has that uncaught error 'wrapped' inside it.
//
// If the returned promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
// It will panic if a nil function is passed.
func Resolver(resolverCb func(
	fulfill func(val ...any),
	reject func(err error, val ...any),
)) AnyPromise {
	return defaultPipeline.Resolver(resolverCb)
}

// Delay returns a GoPromise that's resolved to the passed Res value, res,
// after waiting for at least duration d, accordingly.
//
// The returned promise is resolved to rejected, or fulfilled, depending on
// whether the last element in res is, respectively, a non-nil error value,
// or any other value.
//
// If the promise is about to be fulfilled, resolving the promise will be
// delayed, only if onSucceed = true.
//
// If the promise is about to be rejected, resolving the promise will be
// delayed, only if onFail = true.
//
// The provided res value shouldn't be modified after this call.
//
// If the returned promise is rejected, and the error is not caught(by a Catch
// call) before the end of the promise's chain, or the promise result is not
// read(by a Res call), a panic will happen with an error value of type
// *UncaughtError, which has that uncaught error 'wrapped' inside it.
func Delay(res Result[any], d time.Duration, onSucceed, onFail bool) AnyPromise {
	return defaultPipeline.Delay(res, d, onSucceed, onFail)
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
// If the returned promise is rejected, it will not panic with an *UncaughtError
// error, but all subsequent promises in any promise chain derived from it will,
// until the error is caught on each of these chains(by a Catch call), or the
// promise result is read(by a Res call).
func Wrap(res Result[any]) AnyPromise {
	return defaultPipeline.Wrap(res)
}

// Panic returns a GoPromise that's resolved to panicked, synchronously, and
// whose result is the passed value, v.
//
// The passed value, v, is only accessible from a Recover callback.
//
// All subsequent promises in any promise chain derived from the returned
// promise needs to call Recover, before the end of each promise's chain,
// otherwise all these promise will re-panic(with the passed value, v).
func Panic(v any) AnyPromise {
	return defaultPipeline.Panic(v)
}
