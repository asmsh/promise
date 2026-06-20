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

var neverClosedSyncChan = make(chan struct{})
var alreadyClosedSyncChan = make(chan struct{})

func init() {
	close(alreadyClosedSyncChan)
}

func newSyncCtxWithCancel() (context.Context, context.CancelFunc) {
	sc := make(chan struct{})
	return syncCtx{syncChan: sc}, func() { close(sc) }
}

// syncCtx is a synchronous context.Context value, which doesn't
// require a separate goroutine.
// It's similar to context.cancelCtx, without any connection
// or knowledge between it and any possible children.
// It is used as the callbacks' Context value if the Promise's
// Group has no ctx value.
// If it's used for callbacks, its syncChan is closed once
// the callback returns.
type syncCtx struct{ syncChan <-chan struct{} }

func (sc syncCtx) Deadline() (deadline time.Time, ok bool) { return }
func (sc syncCtx) Done() <-chan struct{}                   { return sc.syncChan }
func (sc syncCtx) Err() error                              { return nil }
func (sc syncCtx) Value(any) any                           { return nil }
func (sc syncCtx) String() string                          { return "syncCtx" }

func noopCancelFunc() {
	// do nothing
}

// callbackCtx returns the effective Context value for a callback along
// with its CancelFunc.
// it takes the syncChan of the next Promise, or nil if no value exists.
// if a syncChan value is provided, it will be used as the backing Done
// channel of the returned Context.
// if a nil syncChan is provided, a new Context value is created.
func callbackCtx[T any](
	g *Group[T],
	sc <-chan struct{},
) (context.Context, context.CancelFunc) {
	// default scenario, either no Group or a Group with default behavior.
	// we return a Context wrapping the syncChan with noop cancellation,
	// if a syncChan is provided,
	// otherwise we return a new Context with cancellation.
	// note: g.core.ctx will be set when either of [GroupConfig.FailuresCancelCBCtx]
	// or [GroupConfig.FailuresCancelGroup] is set.
	if g == nil || (g.core.ctx == nil && !g.core.options.IsNeverCancelCBCtx()) {
		// the syncChan will be nil only when there's no next promise,
		// which only happens from [Promise.Callback] calls for now.
		if sc == nil {
			return newSyncCtxWithCancel()
		}

		// no cancellation needs to be arranged in this case, as the syncChan
		// will be already canceled (closed) once the promise is resolved.
		return syncCtx{syncChan: sc}, noopCancelFunc
	}

	// there's a Group, if it's requested to never cancel callback Context,
	// then we return early with Background and no cancellation.
	if g.core.options.IsNeverCancelCBCtx() {
		return context.Background(), noopCancelFunc
	}

	// there's a Group with a Context, so create the Context to be
	// returned only from the group's Context.
	// note: the ctx returned will be canceled after the execution of
	// the [Callback.Call] in [runCallbackHandler], by the same goroutine
	// resolving Promise and closing the syncChan.
	// in contrast to returning a syncChan, which gets canceled in [handleReturns].
	return context.WithCancel(g.core.ctx)
}
