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

import "context"

type callbackFunc[T any] interface {
	call(ctx context.Context, res Result[T], s uint32) Result[T]
}

type goCallback[T any] func()
type goErrCallback[T any] func() error
type goResCallback[T any] func(ctx context.Context) Result[T]
type thenCallback[T any] func(ctx context.Context, val T) Result[T]
type catchCallback[T any] func(ctx context.Context, val T, err error) Result[T]
type recoverCallback[T any] func(ctx context.Context, v any) Result[T]
type finallyCallback[T any] func(ctx context.Context, s Status) Result[T]

func (cb goCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	cb()
	return nil
}
func (cb goErrCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	err := cb()
	return Err[T](err)
}
func (cb goResCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	return cb(ctx)
}
func (cb thenCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	return cb(ctx, res.Val())
}
func (cb catchCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	return cb(ctx, res.Val(), res.Err())
}
func (cb recoverCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	return cb(ctx, res.Err().(*UncaughtPanic).v)
}
func (cb finallyCallback[T]) call(ctx context.Context, res Result[T], s uint32) Result[T] {
	return cb(ctx, Status(s))
}

func runCallback[T any](
	ctx context.Context,
	p *GenericPromise[T],
	cb callbackFunc[T],
	supportResult bool,
	prevRes Result[T],
	prevStatus uint32,
	freeAfterDone bool,
) {
	// create the Result pointer, to keep track of any result returned
	var resP *Result[T]
	if supportResult {
		resP = new(Result[T])
	}

	// make sure we free this goroutine reservation if it's required
	if freeAfterDone {
		defer p.pipeline.freeGoroutine()
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	defer p.handleReturns(resP)

	// run the callback and extract the result
	res := cb.call(ctx, prevRes, prevStatus)

	// if the callback doesn't support Result returning, return early, as
	// the rest of the logic isn't relevant anymore.
	if !supportResult {
		return
	}

	// if the callback returned invalid result, set the promise result to
	// the appropriate error result, otherwise set it to the value returned.
	if res == nil {
		*resP = Err[T](ErrPromiseNilResult)
	} else {
		*resP = res
	}
}
