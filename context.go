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

var neverClosedSyncCtx context.Context = syncCtx{}
var closedSyncCtx context.Context

func init() {
	closedChan := make(chan struct{})
	close(closedChan)
	closedSyncCtx = syncCtx{syncChan: closedChan}
}

func newSyncCtx() context.Context {
	return syncCtx{syncChan: make(chan struct{})}
}

func closeSyncCtx(ctx context.Context) {
	sc := ctx.(syncCtx)
	// Note: no need to arrange for this close call to be executed
	// only once, as, from current usage, this closeSyncCtx func
	// is only used from one goroutine, the runCallbackHandler, so
	// that's taken care of.
	close(sc.syncChan)
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

func noopCancelFunc() {
	// do nothing
}

// callbackCtx returns the effective Context for a callback, and its CancelFunc,
// if one is required, given the promise's syncCtx value.
// syncCtx should be a non-closed Context, or nil.
func callbackCtx[T any](
	g *Group[T],
	syncCtx context.Context,
) (context.Context, context.CancelFunc) {
	// default scenario, either no Group or a Group with default behavior.
	// we return the syncCtx with no cancellation, if one is provided,
	// otherwise we return Background with cancellation.
	if g == nil || (g.core.ctx == nil && !g.core.neverCancelCBCtx) {
		if syncCtx == nil {
			return newSyncCtx(), nil
		}
		return syncCtx, noopCancelFunc
	}

	// there's a Group, if it's requested to never cancel callback Context,
	// then we return early with Background and no cancellation.
	if g.core.neverCancelCBCtx {
		return context.Background(), noopCancelFunc
	}

	// there's a Group with a group Context, so create the Context to be returned,
	// and arrange to close it when the promise's syncCtx is closed, if provided.
	if syncCtx == nil {
		return context.WithCancel(g.core.ctx)
	}

	// TODO: these 2 context calls can be replaced by a JoinContext that will be
	//  cancelled when any of them is cancelled.
	ctx, cancel := context.WithCancel(g.core.ctx)
	context.AfterFunc(syncCtx, cancel)
	return ctx, cancel
}
