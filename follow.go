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
	prevG.init()
	if prevG != nil {
		nextG = &Group[NextT]{core: prevG.core}
	}
	return nextG
}

func Follow[
	NextT any,
	PrevT any,
	CT CallbackFunc[NextT, PrevT],
](
	p *Promise[PrevT],
	cb CT,
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
	if nextGroup.isWaiting() {
		return newPromSync[NextT](nextGroup, errPromiseGroupDoneResult[NextT]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return newPromBlocked[NextT]()
	}
	if !nextGroup.reserveGoroutine(p.regChainRead) {
		return newPromSync[NextT](nextGroup, errPromiseGroupBusyResult[NextT]{})
	}

	nextProm := newPromInter[NextT](nextGroup)
	ctx, cancel := callbackCtx(nextGroup, nextProm.syncCtx)
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
	CFuncT CallbackFunc[NextT, PrevT],
](
	cbFunc CFuncT,
) Callback[NextT, PrevT] {
	switch cbFuncVal := any(cbFunc).(type) {
	case func():
		return goFunc[NextT, PrevT](cbFuncVal)
	case func() error:
		return goErrFunc[NextT, PrevT](cbFuncVal)
	case func() NextT:
		return goValFunc[NextT, PrevT](cbFuncVal)
	case func() (NextT, error):
		return goValErrFunc[NextT, PrevT](cbFuncVal)
	case func() Result[NextT]:
		return goResFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context):
		return ctxFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) error:
		return ctxErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) NextT:
		return ctxValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) (NextT, error):
		return ctxValErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context) Result[NextT]:
		return ctxResFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]):
		return followFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) error:
		return followErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) NextT:
		return followValFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) (NextT, error):
		return followValErrFunc[NextT, PrevT](cbFuncVal)
	case func(context.Context, Result[PrevT]) Result[NextT]:
		return followResFunc[NextT, PrevT](cbFuncVal)
	default:
		// this will only happen if we added support to a new callback type,
		// in the [CallbackFunc] constraint, without adding a case for it here.
		panic(fmt.Sprintf("promise: internal: received unsupported callback type: %#v", cbFunc))
	}
}
