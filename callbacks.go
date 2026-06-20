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

import (
	"context"
	"fmt"
)

// Func is a type constraint representing the different signatures
// for supported callback functions for the different functions and methods.
type Func[NextT, PrevT any] interface {
	// no type approximation is used (~), hence no user defined function types allowed.
	// might change if this happens: https://github.com/golang/go/issues/45380

	func() |
		func() error |
		func() NextT |
		func() (NextT, error) |
		func() Result[NextT] |

		func(PrevT) |
		func(PrevT) error |
		func(PrevT) NextT |
		func(PrevT) (NextT, error) |
		func(PrevT) Result[NextT] |

		func(Result[PrevT]) |
		func(Result[PrevT]) error |
		func(Result[PrevT]) NextT |
		func(Result[PrevT]) (NextT, error) |
		func(Result[PrevT]) Result[NextT] |

		func(context.Context) |
		func(context.Context) error |
		func(context.Context) NextT |
		func(context.Context) (NextT, error) |
		func(context.Context) Result[NextT] |

		func(context.Context, PrevT) |
		func(context.Context, PrevT) error |
		func(context.Context, PrevT) NextT |
		func(context.Context, PrevT) (NextT, error) |
		func(context.Context, PrevT) Result[NextT] |

		func(context.Context, Result[PrevT]) |
		func(context.Context, Result[PrevT]) error |
		func(context.Context, Result[PrevT]) NextT |
		func(context.Context, Result[PrevT]) (NextT, error) |
		func(context.Context, Result[PrevT]) Result[NextT]
}

// callback represents a typed operation that takes a previous result and returns a new result.
type callback[NextT, PrevT any] interface {
	// Call executes the actual callback logic.
	// prevRes might be nil if the backing callback is called
	// in a constructor and not in a follow method.
	Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT])
}

func runCallbackHandler[
	NextT any,
	PrevT any,
](
	nextProm *Promise[NextT],
	cb callback[NextT, PrevT],
	prevRes Result[PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	// validNextResP will be set to *true, only if the callback returns
	// normally, with not panics nor runtime.Goexit calls.
	// this is needed to support the ErrPromiseGoexit error.
	var validNextResP = new(bool)

	// create the Result pointer and defer the result handler to track
	// any result returned and ensure panic and runtime.Goexit recovery.
	// note: this will only happen for calls that return the next promise,
	// otherwise, the nextProm is nil.
	var nextResP *Result[NextT]
	if nextProm != nil {
		nextResP = new(Result[NextT])
		defer handleReturns(nextProm, prevRes, nextResP, validNextResP)
	}

	// make sure we close the context once we return from the callback.
	defer cancel()

	// run the callback and extract the result
	nextRes := cb.Call(ctx, prevRes)
	*validNextResP = true

	// if the callback doesn't support returning Result, return early,
	// as the rest of the logic isn't relevant anymore.
	if nextResP == nil {
		return
	}

	// set the promise result to the returned value
	*nextResP = nextRes
}

// callbackFrom converts any supported callback function signature into
// a strongly-typed callback interface.
func callbackFrom[
	NextT any,
	PrevT any,
	CBFuncT Func[NextT, PrevT],
](
	cbFunc CBFuncT,
) callback[NextT, PrevT] {
	switch cbFuncVal := any(cbFunc).(type) {
	case func():
		return goFunc[NextT, PrevT](cbFuncVal)
	case func() error:
		return goNextErrFunc[NextT, PrevT](cbFuncVal)
	case func() NextT:
		return goNextValFunc[NextT, PrevT](cbFuncVal)
	case func() (NextT, error):
		return goNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func() Result[NextT]:
		return goNextResFunc[NextT, PrevT](cbFuncVal)

	case func(PrevT):
		return prevValFunc[NextT, PrevT](cbFuncVal)
	case func(PrevT) error:
		return prevValNextErrFunc[NextT, PrevT](cbFuncVal)
	case func(PrevT) NextT:
		return prevValNextValFunc[NextT, PrevT](cbFuncVal)
	case func(PrevT) (NextT, error):
		return prevValNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func(PrevT) Result[NextT]:
		return prevValNextResFunc[NextT, PrevT](cbFuncVal)

	case func(Result[PrevT]):
		return prevResFunc[NextT, PrevT](cbFuncVal)
	case func(Result[PrevT]) error:
		return prevResNextErrFunc[NextT, PrevT](cbFuncVal)
	case func(Result[PrevT]) NextT:
		return prevResNextValFunc[NextT, PrevT](cbFuncVal)
	case func(Result[PrevT]) (NextT, error):
		return prevResNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func(Result[PrevT]) Result[NextT]:
		return prevResNextResFunc[NextT, PrevT](cbFuncVal)

	case func(context.Context):
		return ctxFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) error:
		return ctxNextErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) NextT:
		return ctxNextValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) (NextT, error):
		return ctxNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) Result[NextT]:
		return ctxNextResFunc[NextT, PrevT](cbFuncVal)

	case func(context.Context, PrevT):
		return followValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, PrevT) error:
		return followValNextErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, PrevT) NextT:
		return followValNextValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, PrevT) (NextT, error):
		return followValNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, PrevT) Result[NextT]:
		return followValNextResFunc[NextT, PrevT](cbFuncVal)

	case func(context.Context, Result[PrevT]):
		return followResFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) error:
		return followResNextErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) NextT:
		return followResNextValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) (NextT, error):
		return followResNextValErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) Result[NextT]:
		return followResNextResFunc[NextT, PrevT](cbFuncVal)
	default:
		// this will only happen if we added support to a new callback type,
		// in the [CallbackFunc] constraint, without adding a case for it here.
		panic(fmt.Sprintf("promise: internal: received unsupported callback type: %#v", cbFunc))
	}
}

type (
	goFunc[NextT, PrevT any]           func()
	goNextErrFunc[NextT, PrevT any]    func() error
	goNextValFunc[NextT, PrevT any]    func() NextT
	goNextValErrFunc[NextT, PrevT any] func() (NextT, error)
	goNextResFunc[NextT, PrevT any]    func() Result[NextT]

	prevValFunc[NextT, PrevT any]           func(PrevT)
	prevValNextErrFunc[NextT, PrevT any]    func(PrevT) error
	prevValNextValFunc[NextT, PrevT any]    func(PrevT) NextT
	prevValNextValErrFunc[NextT, PrevT any] func(PrevT) (NextT, error)
	prevValNextResFunc[NextT, PrevT any]    func(PrevT) Result[NextT]

	prevResFunc[NextT, PrevT any]           func(Result[PrevT])
	prevResNextErrFunc[NextT, PrevT any]    func(Result[PrevT]) error
	prevResNextValFunc[NextT, PrevT any]    func(Result[PrevT]) NextT
	prevResNextValErrFunc[NextT, PrevT any] func(Result[PrevT]) (NextT, error)
	prevResNextResFunc[NextT, PrevT any]    func(Result[PrevT]) Result[NextT]

	ctxFunc[NextT, PrevT any]           func(context.Context)
	ctxNextErrFunc[NextT, PrevT any]    func(context.Context) error
	ctxNextValFunc[NextT, PrevT any]    func(context.Context) NextT
	ctxNextValErrFunc[NextT, PrevT any] func(context.Context) (NextT, error)
	ctxNextResFunc[NextT, PrevT any]    func(context.Context) Result[NextT]

	followValFunc[NextT, PrevT any]           func(context.Context, PrevT)
	followValNextErrFunc[NextT, PrevT any]    func(context.Context, PrevT) error
	followValNextValFunc[NextT, PrevT any]    func(context.Context, PrevT) NextT
	followValNextValErrFunc[NextT, PrevT any] func(context.Context, PrevT) (NextT, error)
	followValNextResFunc[NextT, PrevT any]    func(context.Context, PrevT) Result[NextT]

	followResFunc[NextT, PrevT any]           func(context.Context, Result[PrevT])
	followResNextErrFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) error
	followResNextValFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) NextT
	followResNextValErrFunc[NextT, PrevT any] func(context.Context, Result[PrevT]) (NextT, error)
	followResNextResFunc[NextT, PrevT any]    func(context.Context, Result[PrevT]) Result[NextT]
)

func (cb goFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb()
	return nil
}
func (cb goNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb()
	return ErrRes[NextT](err)
}
func (cb goNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb()
	return ValRes(next)
}
func (cb goNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb()
	return ValErrRes(next, err)
}
func (cb goNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb()
}

func (cb prevValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(getFinalRes(prevRes).Val())
	return
}
func (cb prevValNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(getFinalRes(prevRes).Val())
	return ErrRes[NextT](err)
}
func (cb prevValNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(getFinalRes(prevRes).Val())
	return ValRes(next)
}
func (cb prevValNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(getFinalRes(prevRes).Val())
	return ValErrRes(next, err)
}
func (cb prevValNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(getFinalRes(prevRes).Val())
}

func (cb prevResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(getFinalRes(prevRes))
	return
}
func (cb prevResNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(getFinalRes(prevRes))
	return ErrRes[NextT](err)
}
func (cb prevResNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(getFinalRes(prevRes))
	return ValRes(next)
}
func (cb prevResNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(getFinalRes(prevRes))
	return ValErrRes(next, err)
}
func (cb prevResNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(getFinalRes(prevRes))
}

func (cb ctxFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(ctx)
	return nil
}
func (cb ctxNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(ctx)
	return ErrRes[NextT](err)
}
func (cb ctxNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(ctx)
	return ValRes(next)
}
func (cb ctxNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(ctx)
	return ValErrRes(next, err)
}
func (cb ctxNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(ctx)
}

func (cb followValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(ctx, getFinalRes(prevRes).Val())
	return nil
}
func (cb followValNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(ctx, getFinalRes(prevRes).Val())
	return ErrRes[NextT](err)
}
func (cb followValNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(ctx, getFinalRes(prevRes).Val())
	return ValRes(next)
}
func (cb followValNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(ctx, getFinalRes(prevRes).Val())
	return ValErrRes(next, err)
}
func (cb followValNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(ctx, getFinalRes(prevRes).Val())
}

func (cb followResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	cb(ctx, getFinalRes(prevRes))
	return nil
}
func (cb followResNextErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	err := cb(ctx, getFinalRes(prevRes))
	return ErrRes[NextT](err)
}
func (cb followResNextValFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next := cb(ctx, getFinalRes(prevRes))
	return ValRes(next)
}
func (cb followResNextValErrFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	next, err := cb(ctx, getFinalRes(prevRes))
	return ValErrRes(next, err)
}
func (cb followResNextResFunc[NextT, PrevT]) Call(ctx context.Context, prevRes Result[PrevT]) (nextRes Result[NextT]) {
	return cb(ctx, getFinalRes(prevRes))
}
