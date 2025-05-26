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

import "context"

func followGroup[PrevT, NextT any](prevG *Group[PrevT]) (nextG *Group[NextT]) {
	prevG.init()
	if prevG != nil {
		nextG = &Group[NextT]{core: prevG.core}
	}
	return nextG
}

func Follow[PrevT, NextT any](
	p *Promise[PrevT],
	followCb func(ctx context.Context, res Result[PrevT]) Result[NextT],
) *Promise[NextT] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	nextGroup := followGroup[PrevT, NextT](p.group)
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
	go followHandler(p, nextProm, cb, ctx, cancel)
	return nextProm
}
