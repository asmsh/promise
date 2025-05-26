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

type callbackFunc[PrevT, NextT any] interface {
	call(ctx context.Context, res Result[PrevT]) Result[NextT]
}

type goCallback[PrevT, NextT any] func()
type goErrCallback[PrevT, NextT any] func() error
type goResCallback[PrevT, NextT any] func(ctx context.Context) Result[NextT]
type followCallback[PrevT, NextT any] func(ctx context.Context, res Result[PrevT]) Result[NextT]
type finallyCallback[PrevT, NextT any] func(ctx context.Context)
type callbackCallback[PrevT, NextT any] func(ctx context.Context, res Result[PrevT])

func (cb goCallback[PrevT, NextT]) call(context.Context, Result[PrevT]) Result[NextT] {
	cb()
	return nil
}
func (cb goErrCallback[PrevT, NextT]) call(context.Context, Result[PrevT]) Result[NextT] {
	if err := cb(); err != nil {
		return ErrRes[NextT](err)
	}
	return nil
}
func (cb goResCallback[PrevT, NextT]) call(ctx context.Context, _ Result[PrevT]) Result[NextT] {
	return cb(ctx)
}
func (cb followCallback[PrevT, NextT]) call(ctx context.Context, res Result[PrevT]) Result[NextT] {
	return cb(ctx, getFinalRes(res))
}
func (cb finallyCallback[PrevT, NextT]) call(ctx context.Context, _ Result[PrevT]) Result[NextT] {
	cb(ctx)
	return nil
}
func (cb callbackCallback[PrevT, NextT]) call(ctx context.Context, res Result[PrevT]) Result[NextT] {
	cb(ctx, getFinalRes(res))
	return nil
}

func runCallbackHandler[PrevT, NextT any](
	nextProm *Promise[NextT],
	cb callbackFunc[PrevT, NextT],
	prevRes Result[PrevT],
	supportNewResult bool,
	supportHandleReturns bool,
	ctx context.Context,
	cancel context.CancelFunc,
) {
	// create the Result pointer, to keep track of any result returned
	var nextResP *Result[NextT]
	if supportNewResult {
		nextResP = new(Result[NextT])
	}

	// defer the return handler to handle panics and runtime.Goexit calls
	if supportHandleReturns {
		defer handleReturns(nextProm, prevRes, nextResP)
	}

	// make sure we close the context once we return from the callback.
	// if cancel is nil, then the Context is a syncCtx created specifically
	// for this callback handler.
	if cancel != nil {
		defer cancel()
	} else {
		defer closeSyncCtx(ctx)
	}

	// run the callback and extract the result
	nextRes := cb.call(ctx, prevRes)

	// if the callback doesn't support Result returning, return early, as
	// the rest of the logic isn't relevant anymore.
	if !supportNewResult {
		return
	}

	// set the promise result to the returned value
	*nextResP = nextRes
}
