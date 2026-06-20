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
	CBFuncT Func[NextT, PrevT],
](
	p *Promise[PrevT],
	f CBFuncT,
) *Promise[NextT] {
	if f == nil {
		panic(nilCallbackPanicMsg)
	}

	cb := callbackFrom[NextT, PrevT](f)

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

	nextProm := newPromAsync[NextT](nextGroup)
	ctx, cancel := callbackCtx(nextGroup, nextProm.syncChan)
	debug(p, startHandler, startPromiseFollowCallbackHandler)
	go followHandler(
		p,
		nextProm,
		cb,
		ctx,
		cancel,
		true,
		endHandler,
		endPromiseFollowCallbackHandler,
	)
	return nextProm
}
