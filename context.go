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
	"context"
	"time"
)

var closedChan = make(chan struct{})

func init() {
	close(closedChan)
}

func newSyncCtx() context.Context {
	return syncCtx{syncChan: make(chan struct{})}
}

func cancelSyncCtx(ctx context.Context) {
	sc := ctx.(syncCtx)
	// Note: no need to arrange for this close call to be executed
	// only once, as, from current usage, this cancelSyncCtx func
	// is only used from one goroutine, the runCallbackHandler, so
	// that's taken care of.
	close(sc.syncChan)
}

func newClosedSyncCtx() context.Context {
	return syncCtx{syncChan: closedChan}
}

// syncCtx is a sync context.Context value, which doesn't
// require a separate goroutine.
// It's similar to context.cancelCtx, without any connection
// or knowledge between it and any possible children.
// It is used to manage the Promise's resolve logic,
// or only to be passed to callbacks if the Promise has no
// syncCtx value.
// If it's used for a Promise, its syncChan is closed once
// the Promise is resolved.
// If it's used for callbacks, its syncChan is closed once
// the callback returns.
type syncCtx struct{ syncChan chan struct{} }

func (sc syncCtx) Deadline() (deadline time.Time, ok bool) { return }
func (sc syncCtx) Done() <-chan struct{}                   { return sc.syncChan }
func (sc syncCtx) Err() error                              { return nil }
func (sc syncCtx) Value(any) any                           { return nil }
func (sc syncCtx) String() string                          { return "syncCtx" }
