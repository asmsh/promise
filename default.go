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
// a [Promise] value that tracks the execution of cb.
// The [Promise.Res] returns a [Result] value that allows knowing the [State]
// of cb (via [Result.State]).
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

	nextProm := newPromInter[any](nil)
	ctx, cancel := callbackCtx[any](nil, nextProm.syncCtx)
	go goHandler(nextProm, cb, ctx, cancel)
	return nextProm
}

// GoCtxRes runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value that tracks the execution of cb.
// The [Promise.Res] returns a [Result] value that allows knowing the [State]
// of cb (via [Result.State]), the error returned from cb (via [Result.Err]),
// and the value returned from cb (via [Result.Val]).
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

	nextProm := newPromInter[T](nil)
	ctx, cancel := callbackCtx[T](nil, nextProm.syncCtx)
	go goCtxResHandler(nextProm, cb, ctx, cancel)
	return nextProm
}

// TODO: maybe add 'GoCtxErr', as a quick constructor that accepts a Context but returns just an error.

func GoAny[
	NextT any,
	PrevT any,
	CBFuncT CallbackFunc[NextT, PrevT],
](cb CBFuncT) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	nextProm := newPromInter[NextT](nil)
	ctx, cancel := callbackCtx[NextT](nil, nextProm.syncCtx)
	go goCallbackHandler(nextProm, CallbackFrom[NextT, PrevT](cb), ctx, cancel)
	return nextProm
}

// GoCallback runs the [Callback], cb, in a separate goroutine, and returns
// a [Promise] value that tracks the execution of cb.
// The [Promise.Res] returns a [Result] value that allows knowing the [State]
// of cb (via [Result.State]), and the error returned from cb (via [Result.Err]).
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned a non-nil error, or returned a nil error,
// respectively.
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

	nextProm := newPromInter[NextT](nil)
	ctx, cancel := callbackCtx[NextT](nil, nextProm.syncCtx)
	go goCallbackHandler(nextProm, cb, ctx, cancel)
	return nextProm
}

// Delay returns a [Promise] whose [Promise.Res] returns the provided [Result]
// value, res, after waiting for at least duration d, according to how the [State]
// of res matches the provided [DelayCond], cond.
func Delay[T any](res Result[T], d time.Duration, cond ...DelayCond) *Promise[T] {
	// TODO: can we use a central scheduler instead of creating new goroutines?
	// the scheduler can even be per-Group, and a single one for the default Group.
	nextProm := newPromInter[T](nil)
	flags := getDelayFlags(cond)
	go delayHandler(nextProm, res, d, flags)
	return nextProm
}

// Chan returns a [Promise] that wraps the provided [Result] channel, resChan,
// waiting for the first [Result] value sent to it, which will be returned from
// [Promise.Res].
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
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	nextProm := newPromInter[T](nil)
	go chanHandler(nextProm, resChan)
	return nextProm
}

// Ctx returns a [Promise] that wraps the provided [context.Context] value, ctx,
// that's resolved once the ctx is canceled.
// The [Promise.Res] returns a [Result] value that allows knowing the [State]
// of ctx (via [Result.State]), and the error returned from ctx (via [Result.Err]).
//
// Once the [context.Context.Done] channel is closed, the [Result.State] will
// either be [Error], or [Success], based on whether ctx returned a non-nil
// error, or a nil error, respectively.
//
// If the [context.Context.Done] channel is nil or never closed, the returned
// [Promise] value will never resolve, meaning that all its methods will block.
//
// It will panic if ctx is nil.
func Ctx(ctx context.Context) *Promise[any] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}

	// handle contexts with nil done channel, as a nil channel is never closed,
	// and if used with this package it will block follow calls on it forever.
	if ctx.Done() == nil {
		// since this ctx value will never be closed, the equivalent
		// outcome would be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded
		// resources.
		return newPromBlocked[any]()
	}

	nextProm := newPromCtx[any](nil, ctx)
	go ctxHandler(nextProm)
	return nextProm
}

// Wrap returns a [Promise] that wraps the provided [Result] value, res,
// synchronously, without creating any new goroutines.
// The [Promise.Res] will return the provided res.
func Wrap[T any](res Result[T]) *Promise[T] {
	return newPromSync[T](nil, res)
}
