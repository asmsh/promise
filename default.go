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

// defPipelineCore is used for overriding the value passed to all constructors
// below, for the purpose of testing.
var defPipelineCore *pipelineCore

// Chan returns a GoPromise that's created using the provided resChan.
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
func Chan[T any](resChan chan Result[T]) Promise[T] {
	return chanCall[T](defPipelineCore, resChan)
}

func Ctx(ctx context.Context) Promise[any] {
	return ctxCall[any](defPipelineCore, ctx)
}

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
func Go(fun func()) Promise[any] {
	return goCall[any](defPipelineCore, fun)
}

func GoErr(fun func() error) Promise[any] {
	return goErrCall[any](defPipelineCore, fun)
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
func GoRes[T any](fun func(ctx context.Context) Result[T]) Promise[T] {
	return goResCall[T](defPipelineCore, fun)
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
func Delay[T any](res Result[T], d time.Duration, cond ...DelayCond) Promise[T] {
	return delayCall[T](defPipelineCore, res, d, cond...)
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
func Wrap[T any](res Result[T]) Promise[T] {
	return wrapCall[T](defPipelineCore, res)
}

// Panic returns a GoPromise that's resolved to panicked, synchronously, and
// whose result is the passed value, v.
//
// The passed value, v, is only accessible from a Recover callback.
//
// All subsequent promises in any promise chain derived from the returned
// promise needs to call Recover, before the end of each promise's chain,
// otherwise all these promise will re-panic(with the passed value, v).
func Panic[T any](v any) Promise[T] {
	return panicCall[T](defPipelineCore, v)
}
