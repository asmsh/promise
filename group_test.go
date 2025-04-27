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
	"testing"
	"time"
)

var testCtx, testCtxCancel = newTestContext()

type testContext struct {
	context.Context
}

func newTestContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return &testContext{ctx}, cancel
}
func (t *testContext) String() string { return "testContext" }

var testsCases_Group_callbackCtx = []struct {
	name             string
	g                *Group[any]
	syncCtx          context.Context
	expectedCtxName  string
	expectsNilCancel bool
}{
	// Callback cases (nil syncCtx is passes)...
	{
		// when no Group is available.
		name:             "nil_group,nil_syncCtx",
		expectedCtxName:  "syncCtx",
		expectsNilCancel: true,
	},
	{
		// when a Group is available with neverCancelCBCtx=false.
		name:             "non-nil_group,nil_group_ctx,cancel-callback-ctx,nil_syncCtx",
		g:                &Group[any]{core: groupCore{}},
		expectedCtxName:  "syncCtx",
		expectsNilCancel: true,
	},
	{
		// when a Group is available with neverCancelCBCtx=true.
		name:            "non-nil_group,nil_group_ctx,never-cancel-callback-ctx,nil_syncCtx",
		g:               &Group[any]{core: groupCore{neverCancelCBCtx: true}},
		expectedCtxName: "context.Background",
	},
	{
		// when a Group is available with the group Context set.
		name:            "non-nil_group,non-nil_group_ctx,cancel-callback-ctx,nil_syncCtx",
		g:               &Group[any]{core: groupCore{ctx: testCtx, cancel: testCtxCancel}},
		expectedCtxName: "testContext.WithCancel",
	},

	// Follow cases (non-nil syncCtx is passes)...
	{
		// when no Group is available.
		name:            "nil_group,non-nil_syncCtx",
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=false.
		name:            "non-nil_group,nil_group_ctx,cancel-callback-ctx,non-nil_syncCtx",
		g:               &Group[any]{core: groupCore{}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=true.
		name:            "non-nil_group,nil_group_ctx,never-cancel-callback-ctx,non-nil_syncCtx",
		g:               &Group[any]{core: groupCore{neverCancelCBCtx: true}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "context.Background",
	},
	{
		// when a Group is available with the group Context set.
		name:            "non-nil_group,non-nil_group_ctx,cancel-callback-ctx,non-nil_syncCtx",
		g:               &Group[any]{core: groupCore{ctx: testCtx, cancel: testCtxCancel}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "testContext.WithCancel",
	},
}

func TestGroup_callbackCtx(t *testing.T) {
	for _, test := range testsCases_Group_callbackCtx {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.g.callbackCtx(test.syncCtx)
			if ctx == nil {
				t.Errorf("nil ctx")
			}
			if cancel == nil && !test.expectsNilCancel {
				t.Errorf("nil cancel")
			}
			ctxName := fmt.Sprintf("%s", ctx)
			if ctxName != test.expectedCtxName {
				t.Errorf("ctxName is %s, want %s", ctxName, test.expectedCtxName)
			}
		})
	}
}

func BenchmarkGroup_callbackCtx(b *testing.B) {
	for _, bm := range testsCases_Group_callbackCtx {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctx, cancel := bm.g.callbackCtx(bm.syncCtx)
				if ctx == nil {
					b.Errorf("nil ctx")
				}
				if cancel == nil && !bm.expectsNilCancel {
					b.Errorf("nil cancel")
				}
			}
		})
	}
}

func TestGroup_Wait(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		targetDuration := 300 * time.Millisecond
		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(targetDuration)
			return Err[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(targetDuration)
			return Val("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(targetDuration)
			return Val("promise3")
		})

		// at least targetDuration have to pass while waiting.
		start := time.Now()
		g.Wait()
		elapsed := time.Since(start)
		if elapsed < targetDuration {
			t.Errorf("Wait() returned early")
		}

		// make sure we don't wait longer than we should.
		// for that, we allow a buffer duration of 5 Millisecond.
		if elapsed > targetDuration+(5*time.Millisecond) {
			t.Errorf("Wait() waited so long. expected: %s, got: %s", targetDuration, elapsed)
		}
	})
}

func TestGroup_SelectRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return Err[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return Val("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(3 * time.Millisecond)
			return Val("promise3")
		})

		res := g.SelectRes()

		if res.State() != Rejected {
			t.Errorf("res.State() is %s, want Rejected", res.State())
		}
	})
}

func TestGroup_AllWaitRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return Err[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return Val("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(5 * time.Millisecond)
			return Val("promise3")
		})

		res := g.AllWaitRes()

		if res.State() != Rejected {
			t.Errorf("res.State() is %s, want Rejected", res.State())
		}
	})
}

func TestGroup_AnyWaitRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return Err[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return Val("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(5 * time.Millisecond)
			return Val("promise3")
		})

		res := g.AnyWaitRes()

		if res.State() != Fulfilled {
			t.Errorf("res.State() is %s, want Fulfilled", res.State())
		}
	})

	t.Run("basic flow with basic wait", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			panic(testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return Val("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(5 * time.Millisecond)
			return Val("promise3")
		})

		res := g.AnyWaitRes(func() {
			g.Wait()
		})

		if res.State() != Panicked {
			t.Errorf("res.State() is %s, want Panicked", res.State())
		}
	})
}

func TestGroup_JoinRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return Err[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			panic(testStrError("promise2"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(5 * time.Millisecond)
			panic(testStrError("promise3"))
		})

		res := g.JoinRes()

		if res.State() != Fulfilled {
			t.Errorf("res.State() is %s, want Fulfilled", res.State())
		}
	})
}
