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

type callbackFunc[PrevResT, NewResT any] interface {
	call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT]
}

type goCallback[PrevResT, NewResT any] func()
type goErrCallback[PrevResT, NewResT any] func() error
type goResCallback[PrevResT, NewResT any] func(ctx context.Context) Result[NewResT]
type thenCallback[PrevResT, NewResT any] func(ctx context.Context, val PrevResT) Result[NewResT]
type catchCallback[PrevResT, NewResT any] func(ctx context.Context, val PrevResT, err error) Result[NewResT]
type recoverCallback[PrevResT, NewResT any] func(ctx context.Context, v any) Result[NewResT]
type finallyCallback[PrevResT, NewResT any] func(ctx context.Context, s Status) Result[NewResT]

func (cb goCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	cb()
	return nil
}
func (cb goErrCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	err := cb()
	return Err[NewResT](err)
}
func (cb goResCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	return cb(ctx)
}
func (cb thenCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	return cb(ctx, res.Val())
}
func (cb catchCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	return cb(ctx, res.Val(), res.Err())
}
func (cb recoverCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	return cb(ctx, res.Err().(*UncaughtPanic).v)
}
func (cb finallyCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT], s uint32) Result[NewResT] {
	return cb(ctx, Status(s))
}

func runCallback[PrevResT, NewResT any](
	p *GenericPromise[NewResT],
	cb callbackFunc[PrevResT, NewResT],
	supportResult bool,
	prevRes Result[PrevResT],
	prevStatus uint32,
	freeAfterDone bool,
) {
	// create the Result pointer, to keep track of any result returned
	var resP *Result[NewResT]
	if supportResult {
		resP = new(Result[NewResT])
	}

	// make sure we free this goroutine reservation if it's required
	if freeAfterDone {
		defer p.pipeline.freeGoroutine()
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	defer handleReturns(p, resP)

	// run the callback and extract the result
	res := cb.call(p.ctx, prevRes, prevStatus)

	// if the callback doesn't support Result returning, return early, as
	// the rest of the logic isn't relevant anymore.
	if !supportResult {
		return
	}

	// if the callback returned invalid result, set the promise result to
	// the appropriate error result, otherwise set it to the value returned.
	if res == nil {
		*resP = Err[NewResT](ErrPromiseNilResult)
	} else {
		*resP = res
	}
}
