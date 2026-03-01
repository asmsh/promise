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

// Go runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The [Result.State] will either be [Panic], or [Success], based on whether
// cb caused a panic, or returned normally, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func Go(cb func()) *Promise[any] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, goFunc[any, any](cb))
}

// GoErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned nil, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoErr(cb func() error) *Promise[any] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, goNextErrFunc[any, any](cb))
}

// GoValErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoValErr[T any](cb func() (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, goNextValErrFunc[T, T](cb))
}

// GoCtxErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The cb function receives a [context.Context] value that is canceled once
// cb returns.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned nil, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoCtxErr(cb func(ctx context.Context) error) *Promise[any] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, ctxNextErrFunc[any, any](cb))
}

// GoCtxValErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The cb function receives a [context.Context] value that is canceled once
// cb returns.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoCtxValErr[T any](cb func(ctx context.Context) (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, ctxNextValErrFunc[T, T](cb))
}

// GoCtxRes runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoCtxRes[T any](cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, ctxNextResFunc[T, T](cb))
}

// GoFunc runs the provided callback function, cb, in a separate goroutine,
// and returns a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The cb function must satisfy the [CallbackFunc] constraint, which accepts
// any of the supported callback function signatures.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned nil, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return whatever cb
// returned as a value, if any.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoFunc[
	NextT any,
	PrevT any,
	CBFuncT CallbackFunc[NextT, PrevT],
](cb CBFuncT) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, CallbackFrom[NextT, PrevT](cb))
}

// GoCallback runs the [Callback], cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.Res] tracks the execution of cb.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned nil, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return whatever cb
// returned as a value, if any.
//
// If cb called runtime.Goexit, [Result.State] will be [Success] and [Result.Val]
// will return nil.
//
// It will panic if cb is nil.
func GoCallback[NextT, PrevT any](cb Callback[NextT, PrevT]) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(nil, cb)
}

// Delay returns a [Promise] that resolves to res after waiting for at least
// duration d in a separate goroutine, according to how the [State] of res
// matches the provided cond.
func Delay[T any](res Result[T], d time.Duration, cond ...DelayCond) *Promise[T] {
	return (*Group[T]).Delay(nil, res, d, cond...)
}

// Chan returns a [Promise] that wraps the provided [Result] channel, resChan,
// waiting in a separate goroutine for the first [Result] value sent to it,
// which will be used to resolve the returned [Promise].
//
// Only one [Result] value is received from resChan, and any later values
// will not be received (and will block if the channel becomes full).
//
// Closing the resChan will have the same effect as sending nil to it.
//
// Sending nil to resChan will make the [Result.State] return [Success],
// [Result.Err] return nil, and [Result.Val] return nil.
//
// It will panic if resChan is nil.
func Chan[T any](resChan <-chan Result[T]) *Promise[T] {
	return (*Group[T]).Chan(nil, resChan)
}

// Ctx returns a [Promise] that wraps the provided [context.Context] value, ctx,
// that's resolved in a separate goroutine once the ctx is canceled.
// The [Promise.Res] returns a [Result] value that allows knowing the [State]
// of ctx (via [Result.State]), and the error returned from ctx (via [Result.Err]).
//
// Once the [context.Context.Done] channel is closed, the [Result.State] will
// either be [Error], or [Success], based on whether [context.Context.Err]
// returned an error, or nil, respectively.
//
// If the [context.Context.Done] channel is nil or never closed, the returned
// [Promise] value will never resolve, meaning that all its methods will block.
//
// It will panic if ctx is nil.
func Ctx(ctx context.Context) *Promise[any] {
	return (*Group[any]).Ctx(nil, ctx)
}

// Wrap returns a [Promise] that wraps the provided [Result] value, res,
// synchronously, without creating any new goroutines.
// The [Promise.Res] will return the provided res.
func Wrap[T any](res Result[T]) *Promise[T] {
	return (*Group[T]).Wrap(nil, res)
}
