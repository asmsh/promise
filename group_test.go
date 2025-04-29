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

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 50 * time.Millisecond // max(1, 2, 50)
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a buffer duration of 5 Milliseconds.

		start := time.Now()
		g.Wait()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("Wait() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("Wait() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}
	})
}

func TestGroup_SelectRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(20 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 1 * time.Millisecond // promise1 sleep.
		upperDuration := 6 * time.Millisecond // promise1 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		res := g.SelectRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("SelectRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("SelectRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Error {
			t.Errorf("SelectRes().State() is %s, want Error", res.State())
		}
	})
}

func TestGroup_AllRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(10 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(100 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(500 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 10 * time.Millisecond // promise1 sleep.
		upperDuration := 15 * time.Millisecond // promise1 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		// it shouldn't wait for other promises once one is Error.
		res := g.AllRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AllRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AllRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Error {
			t.Errorf("AllRes().State() is %s, want Error", res.State())
		}
	})

	t.Run("panic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(10 * time.Millisecond)
			return PanicRes[string]("promise1")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(100 * time.Millisecond)
			return ErrRes[string](testStrError("promise2"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(500 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 100 * time.Millisecond // promise2 sleep.
		upperDuration := 105 * time.Millisecond // promise2 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		// it shouldn't wait for other promises once one is Error,
		// unless that one is Panic.
		res := g.AllRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AllRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AllRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Panic {
			t.Errorf("AllRes().State() is %s, want Panic", res.State())
		}
	})
}

func TestGroup_AllWaitRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 50 * time.Millisecond // max(1, 2, 50)
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a buffer duration of 5 Milliseconds.

		start := time.Now()
		res := g.AllWaitRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AllWaitRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AllWaitRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Error {
			t.Errorf("AllWaitRes().State() is %s, want Error", res.State())
		}
	})
}

func TestGroup_AnyRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(10 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(100 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(500 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 100 * time.Millisecond // promise2 sleep.
		upperDuration := 105 * time.Millisecond // promise2 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		// it shouldn't wait for other promises once one is Success.
		res := g.AnyRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AnyRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AnyRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Success {
			t.Errorf("AnyRes().State() is %s, want Success", res.State())
		}
	})

	t.Run("panic flow - early", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(10 * time.Millisecond)
			return PanicRes[string]("promise1")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(100 * time.Millisecond)
			return ErrRes[string](testStrError("promise2"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(500 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 500 * time.Millisecond // promise3 sleep.
		upperDuration := 505 * time.Millisecond // promise3 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		// it should wait until one is Success, but if there was one that
		// is Panic, it should resolve to Panic.
		res := g.AnyRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AnyRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AnyRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Panic {
			t.Errorf("AnyRes().State() is %s, want Panic", res.State())
		}
	})

	t.Run("panic flow - late", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(10 * time.Millisecond)
			return ValRes("promise1")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(100 * time.Millisecond)
			return ErrRes[string](testStrError("promise2"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(500 * time.Millisecond)
			return PanicRes[string]("promise3")
		})

		lowerDuration := 10 * time.Millisecond // promise1 sleep.
		upperDuration := 15 * time.Millisecond // promise1 sleep + a buffer duration of 5 Milliseconds.

		start := time.Now()
		// it shouldn't wait for other promises once one is Success.
		res := g.AnyRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AllRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AllRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Success {
			t.Errorf("AllRes().State() is %s, want Success", res.State())
		}
	})
}

func TestGroup_AnyWaitRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 50 * time.Millisecond // max(1, 2, 50)
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a buffer duration of 5 Milliseconds.

		start := time.Now()
		res := g.AnyWaitRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AnyWaitRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AnyWaitRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Success {
			t.Errorf("AnyWaitRes().State() is %s, want Success", res.State())
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
			return ValRes("promise2")
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			return ValRes("promise3")
		})

		lowerDuration := 50 * time.Millisecond // max(1, 2, 50)
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a buffer duration of 5 Milliseconds.

		start := time.Now()
		res := g.AnyWaitRes(func() {
			g.Wait()
		})
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("AnyWaitRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("AnyWaitRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Panic {
			t.Errorf("AnyWaitRes().State() is %s, want Panic", res.State())
		}
	})
}

func TestGroup_JoinRes(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		g := NewGroup[string]()

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(1 * time.Millisecond)
			return ErrRes[string](testStrError("promise1"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(2 * time.Millisecond)
			panic(testStrError("promise2"))
		})

		g.GoRes(func(ctx context.Context) Result[string] {
			time.Sleep(50 * time.Millisecond)
			panic(testStrError("promise3"))
		})

		lowerDuration := 50 * time.Millisecond // max(1, 2, 50)
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a buffer duration of 5 Milliseconds.

		start := time.Now()
		res := g.JoinRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("JoinRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("JoinRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if res.State() != Success {
			t.Errorf("JoinRes().State() is %s, want Success", res.State())
		}
	})
}

