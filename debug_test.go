// Copyright 2024 Ahmad Sameh(asmsh)
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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

func (de debugEvent) String() string {
	switch de {
	case resolve:
		return "resolve"
	case resolveSuccess:
		return "resolveSuccess"
	case resolveError:
		return "resolveError"
	case resolvePanic:
		return "resolvePanic"
	case startHandleExtCalls:
		return "startHandleExtCalls"
	case endHandleExtCalls:
		return "endHandleExtCalls"
	case foundExtChan:
		return "foundExtChan"
	case missingExtChan:
		return "missingExtChan"
	case foundExtQueue:
		return "foundExtQueue"
	case emptyExtQueue:
		return "emptyExtQueue"
	case doneHandleExtCall:
		return "doneHandleExtCall"
	case startHandleGroupCalls:
		return "startHandleGroupCalls"
	case endHandleGroupCalls:
		return "endHandleGroupCalls"
	case doneHandleGroupCall:
		return "doneHandleGroupCall"
	case doneSaveGroupResult:
		return "doneSaveGroupResult"
	case startUnhandledPanicLogic:
		return "startUnhandledPanicLogic"
	case callUnhandledPanicCallback:
		return "callUnhandledPanicCallback"
	case startUnhandledErrorLogic:
		return "startUnhandledErrorLogic"
	case callUnhandledErrorCallback:
		return "callUnhandledErrorCallback"
	case startHandler:
		return "startHandler"
	case endHandler:
		return "endHandler"
	case startGroupHandler:
		return "startGroupHandler"
	case startGroupChanHandler:
		return "startGroupChanHandler"
	case startGroupCtxHandler:
		return "startGroupCtxHandler"
	case startGroupGoHandler:
		return "startGoHandler"
	case startGroupDelayHandler:
		return "startGroupDelayHandler"
	case endGroupHandler:
		return "endGroupHandler"
	case endGroupChanHandler:
		return "endGroupChanHandler"
	case endGroupCtxHandler:
		return "endGroupCtxHandler"
	case endGroupGoHandler:
		return "endGroupGoHandler"
	case endGroupDelayHandler:
		return "endGroupDelayHandler"
	case startPromiseHandler:
		return "startPromiseHandler"
	case startPromiseDelayHandler:
		return "startPromiseDelayHandler"
	case startPromiseCallbackHandler:
		return "startPromiseCallbackHandler"
	case startPromiseFollowHandler:
		return "startPromiseFollowHandler"
	case startPromiseFollowCallbackHandler:
		return "startPromiseFollowCallbackHandler"
	case startPromiseThenHandler:
		return "startPromiseThenHandler"
	case startPromiseCatchHandler:
		return "startPromiseCatchHandler"
	case startPromiseRecoverHandler:
		return "startPromiseRecoverHandler"
	case startPromiseFinallyHandler:
		return "startPromiseFinallyHandler"
	case endPromiseHandler:
		return "endPromiseHandler"
	case endPromiseDelayHandler:
		return "endPromiseDelayHandler"
	case endPromiseCallbackHandler:
		return "endPromiseCallbackHandler"
	case endPromiseFollowHandler:
		return "endPromiseFollowHandler"
	case endPromiseFollowCallbackHandler:
		return "endPromiseFollowCallbackHandler"
	case endPromiseThenHandler:
		return "endPromiseThenHandler"
	case endPromiseCatchHandler:
		return "endPromiseCatchHandler"
	case endPromiseRecoverHandler:
		return "endPromiseRecoverHandler"
	case endPromiseFinallyHandler:
		return "endPromiseFinallyHandler"
	default:
		return "unknown"
	}
}

type debugMetricsResult struct {
	mu sync.RWMutex
	em map[debugEvent]*atomic.Int64
}

func (dmr *debugMetricsResult) clearAll() {
	dmr.mu.Lock()
	clear(dmr.em)
	dmr.mu.Unlock()
}

func (dmr *debugMetricsResult) get(dt debugEvent) int64 {
	dmr.mu.RLock()
	c, ok := dmr.em[dt]
	dmr.mu.RUnlock()
	if !ok {
		return 0
	}
	return c.Load()
}

func (dmr *debugMetricsResult) String() string {
	s := strings.Builder{}
	dmr.mu.RLock()
	s.WriteString(fmt.Sprintf("debug metrics for %d events\n", len(dmr.em)))
	for de, c := range dmr.em {
		s.WriteString(fmt.Sprintf("%s: %d\n", de, c.Load()))
	}
	dmr.mu.RUnlock()
	return s.String()
}

func createDebugMetricsCB() (*debugMetricsResult, func([]debugEvent)) {
	dm := &debugMetricsResult{em: make(map[debugEvent]*atomic.Int64)}
	return dm, func(des []debugEvent) {
		dm.mu.Lock()
		for _, de := range des {
			if de == 0 {
				continue
			}

			c, ok := dm.em[de]
			if !ok {
				c = new(atomic.Int64)
				dm.em[de] = c
			}
			c.Add(1)
		}
		dm.mu.Unlock()
	}
}
