// Copyright 2025 Ahmad Sameh(asmsh)
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

func followGroup[NextT, PrevT any](prevG *Group[PrevT]) (nextG *Group[NextT]) {
	if prevG == nil {
		return nil
	}
	prevG.init()
	return &Group[NextT]{core: prevG.core}
}

func Follow[
	NextT any,
	PrevT any,
	CBFuncT CallbackFunc[NextT, PrevT],
](
	p *Promise[PrevT],
	cb CBFuncT,
) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return FollowCallback(p, CallbackFrom[NextT, PrevT](cb))
}

func FollowCallback[
	NextT any,
	PrevT any,
](
	p *Promise[PrevT],
	cb Callback[NextT, PrevT],
) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	nextGroup := followGroup[NextT, PrevT](p.group)
	if errRes := nextGroup.validateActive(); errRes != nil {
		return newPromSync[NextT](nextGroup, errRes)
	}
	if p.syncChan == neverClosedSyncChan {
		return newPromBlocked[NextT]()
	}
	if !nextGroup.reserveGoroutine(p.regChainRead) {
		return newPromSync[NextT](nextGroup, errGroupBusyResult[NextT]{})
	}

	nextProm := newPromInter[NextT](nextGroup)
	ctx, cancel := callbackCtx(nextGroup, nextProm.syncChan)
	debug(p, startHandler, startPromiseFollowCallbackHandler)
	go followHandler(
		p,
		nextProm,
		cb,
		ctx,
		cancel,
		followOp,
		endHandler,
		endPromiseFollowCallbackHandler,
	)
	return nextProm
}

func CallbackFrom[
	NextT any,
	PrevT any,
	CBFuncT CallbackFunc[NextT, PrevT],
](
	cbFunc CBFuncT,
) Callback[NextT, PrevT] {
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
