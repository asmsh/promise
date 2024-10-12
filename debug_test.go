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

func (d debugEvent) String() string {
	switch d {
	case resolve:
		return "resolve"
	case resolveFulfilled:
		return "resolveFulfilled"
	case resolveRejected:
		return "resolveRejected"
	case resolvePanicked:
		return "resolvePanicked"
	case startExtCall:
		return "startExtCall"
	case endExtCall:
		return "endExtCall"
	case foundExtChan:
		return "foundExtChan"
	case missingExtChan:
		return "missingExtChan"
	case foundExtQueue:
		return "foundExtQueue"
	case startUncaughtPanicHandler:
		return "startUncaughtPanicHandler"
	case callUncaughtPanicHandler:
		return "callUncaughtPanicHandler"
	case startUncaughtErrorHandler:
		return "startUncaughtErrorHandler"
	case callUncaughtErrorHandler:
		return "callUncaughtErrorHandler"
	case startHandler:
		return "startHandler"
	case endHandler:
		return "endHandler"
	case startConstrHandler:
		return "startConstrHandler"
	case startConstrChanHandler:
		return "startConstrChanHandler"
	case startConstrGoHandler:
		return "startConstrGoHandler"
	case startConstrGoErrHandler:
		return "startConstrGoErrHandler"
	case startConstrGoResHandler:
		return "startConstrGoResHandler"
	case startConstrDelayHandler:
		return "startConstrDelayHandler"
	case endConstrHandler:
		return "endConstrHandler"
	case endConstrChanHandler:
		return "endConstrChanHandler"
	case endConstrGoHandler:
		return "endConstrGoHandler"
	case endConstrGoErrHandler:
		return "endConstrGoErrHandler"
	case endConstrGoResHandler:
		return "endConstrGoResHandler"
	case endConstrDelayHandler:
		return "endConstrDelayHandler"
	case startFollowHandler:
		return "startFollowHandler"
	case startCallbackFollowHandler:
		return "startCallbackFollowHandler"
	case startDelayFollowHandler:
		return "startDelayFollowHandler"
	case startThenFollowHandler:
		return "startThenFollowHandler"
	case startCatchFollowHandler:
		return "startCatchFollowHandler"
	case startRecoverFollowHandler:
		return "startRecoverFollowHandler"
	case startFinallyFollowHandler:
		return "startFinallyFollowHandler"
	case endFollowHandler:
		return "endFollowHandler"
	case endCallbackFollowHandler:
		return "endCallbackFollowHandler"
	case endDelayFollowHandler:
		return "endDelayFollowHandler"
	case endThenFollowHandler:
		return "endThenFollowHandler"
	case endCatchFollowHandler:
		return "endCatchFollowHandler"
	case endRecoverFollowHandler:
		return "endRecoverFollowHandler"
	case endFinallyFollowHandler:
		return "endFinallyFollowHandler"
	default:
		return "unknown"
	}
}

type debugMetricsResult struct {
	mu sync.RWMutex
	em map[debugEvent]*atomic.Int64
}

func (d *debugMetricsResult) clearAll() {
	d.mu.Lock()
	clear(d.em)
	d.mu.Unlock()
}

func (d *debugMetricsResult) get(dt debugEvent) int64 {
	d.mu.RLock()
	m, ok := d.em[dt]
	d.mu.RUnlock()
	if !ok {
		return 0
	}
	return m.Load()
}

func (d *debugMetricsResult) String() string {
	s := strings.Builder{}
	d.mu.RLock()
	s.WriteString(fmt.Sprintf("debug metrics for %d events\n", len(d.em)))
	for dt, count := range d.em {
		s.WriteString(fmt.Sprintf("%s: %d\n", dt, count.Load()))
	}
	d.mu.RUnlock()
	return s.String()
}

func createDebugMetricsCB() (*debugMetricsResult, func([]debugEvent)) {
	dm := &debugMetricsResult{em: make(map[debugEvent]*atomic.Int64)}
	return dm, func(dt []debugEvent) {
		dm.mu.Lock()
		for _, dt := range dt {
			d, ok := dm.em[dt]
			if !ok {
				d = new(atomic.Int64)
				dm.em[dt] = d
			}
			d.Add(1)
		}
		dm.mu.Unlock()
	}
}
