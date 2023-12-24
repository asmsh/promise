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
	call(ctx context.Context, res Result[PrevResT]) Result[NewResT]
}

type goCallback[PrevResT, NewResT any] func()
type goErrCallback[PrevResT, NewResT any] func() error
type goResCallback[PrevResT, NewResT any] func(ctx context.Context) Result[NewResT]
type thenCallback[PrevResT, NewResT any] func(ctx context.Context, val PrevResT) Result[NewResT]
type catchCallback[PrevResT, NewResT any] func(ctx context.Context, val PrevResT, err error) Result[NewResT]
type recoverCallback[PrevResT, NewResT any] func(ctx context.Context, v any) Result[NewResT]
type finallyCallback[PrevResT, NewResT any] func(ctx context.Context)
type callbackCallback[PrevResT, NewResT any] func(ctx context.Context, val PrevResT)

func (cb goCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	cb()
	return nil
}
func (cb goErrCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	err := cb()
	return Err[NewResT](err)
}
func (cb goResCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	return cb(ctx)
}
func (cb thenCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	return cb(ctx, res.Val())
}
func (cb catchCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	return cb(ctx, res.Val(), res.Err())
}
func (cb recoverCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	return cb(ctx, res.Err().(UncaughtPanic).v)
}
func (cb finallyCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	cb(ctx)
	return nil
}
func (cb callbackCallback[PrevResT, NewResT]) call(ctx context.Context, res Result[PrevResT]) Result[NewResT] {
	cb(ctx, res.Val())
	return nil
}

func runCallback[PrevResT, NewResT any](
	p *genericPromise[NewResT],
	cb callbackFunc[PrevResT, NewResT],
	prevRes Result[PrevResT],
	supportNewResult bool,
	freeAfterDone bool,
	supportHandleReturns bool,
	ctx context.Context,
	cancel context.CancelFunc,
) {
	// create the Result pointer, to keep track of any result returned
	var newResP *Result[NewResT]
	if supportNewResult {
		newResP = new(Result[NewResT])
	}

	// make sure we free this goroutine reservation if it's required
	if freeAfterDone {
		defer p.pipeline.freeGoroutine()
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	if supportHandleReturns {
		defer handleReturns(p, prevRes, newResP)
	}

	// make sure we close the context once we return from the callback
	defer cancel()

	// run the callback and extract the result
	newRes := cb.call(ctx, getFinalRes(prevRes))

	// if the callback doesn't support Result returning, return early, as
	// the rest of the logic isn't relevant anymore.
	if !supportNewResult {
		return
	}

	// set the promise result to the returned value
	*newResP = newRes
}
