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

// CallbackFunc is a type constraint representing the different signatures
// for supported callback functions for the different functions and methods.
type CallbackFunc[NextT, PrevT any] interface {
	// no type approximation is used (~), hence no user defined types allowed.
	// might change if this happens: https://github.com/golang/go/issues/45380

	func() |
		func() error |
		func() NextT |
		func() (NextT, error) |
		func() Result[NextT] |

		func(context.Context) |
		func(context.Context) error |
		func(context.Context) NextT |
		func(context.Context) (NextT, error) |
		func(context.Context) Result[NextT] |

		func(context.Context, Result[PrevT]) |
		func(context.Context, Result[PrevT]) error |
		func(context.Context, Result[PrevT]) NextT |
		func(context.Context, Result[PrevT]) (NextT, error) |
		func(context.Context, Result[PrevT]) Result[NextT]
}

type Callback[NextT, PrevT any] interface {
	// Call executes the actual callback logic.
	// prevRes might be nil, if the backing callback is called
	// in a constructor, and not in a follow method.
	Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT])
}

func runCallbackHandler[
	NextT any,
	PrevT any,
](
	nextProm *Promise[NextT],
	cb Callback[NextT, PrevT],
	prevRes Result[PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	// create the Result pointer, and defer the result handler, to track
	// any result returned, and ensure panic and runtime.Goexit recovery.
	// note: this will only happen for calls that returns a next promise,
	// otherwise, the nextProm is nil.
	var nextResP *Result[NextT]
	if nextProm != nil {
		nextResP = new(Result[NextT])
		defer handleReturns(nextProm, prevRes, nextResP)
	}

	// make sure we close the context once we return from the callback.
	defer cancel()

	// run the callback and extract the result
	nextRes := cb.Call(ctx, prevRes)

	// if the callback doesn't support returning Result, return early,
	// as the rest of the logic isn't relevant anymore.
	if nextResP == nil {
		return
	}

	// set the promise result to the returned value
	*nextResP = nextRes
}

type (
	goFunc[NextT, PrevT any]       func()
	goErrFunc[NextT, PrevT any]    func() error
	goValFunc[NextT, PrevT any]    func() NextT
	goValErrFunc[NextT, PrevT any] func() (NextT, error)
	goResFunc[NextT, PrevT any]    func() Result[NextT]

	ctxFunc[NextT, PrevT any]       func(context.Context)
	ctxErrFunc[NextT, PrevT any]    func(context.Context) error
	ctxValFunc[NextT, PrevT any]    func(context.Context) NextT
	ctxValErrFunc[NextT, PrevT any] func(context.Context) (NextT, error)
	ctxResFunc[NextT, PrevT any]    func(context.Context) Result[NextT]

	followFunc[NextT, PrevT any]       func(context.Context, Result[PrevT])
	followErrFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) error
	followValFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) NextT
	followValErrFunc[NextT, PrevT any] func(context.Context, Result[PrevT]) (NextT, error)
	followResFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) Result[NextT]
)

func (cb goFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb()
	return nil
}
func (cb goErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb()
	return ErrRes[NextT](err)
}
func (cb goValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb()
	return ValRes(next)
}
func (cb goValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb()
	return ValErrRes(next, err)
}
func (cb goResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb()
}
func (cb ctxFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(ctx)
	return nil
}
func (cb ctxErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(ctx)
	return ErrRes[NextT](err)
}
func (cb ctxValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(ctx)
	return ValRes(next)
}
func (cb ctxValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(ctx)
	return ValErrRes(next, err)
}
func (cb ctxResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(ctx)
}
func (cb followFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(ctx, getFinalRes(prevRes))
	return nil
}
func (cb followErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(ctx, getFinalRes(prevRes))
	return ErrRes[NextT](err)
}
func (cb followValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(ctx, getFinalRes(prevRes))
	return ValRes(next)
}
func (cb followValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(ctx, getFinalRes(prevRes))
	return ValErrRes(next, err)
}
func (cb followResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(ctx, getFinalRes(prevRes))
}
