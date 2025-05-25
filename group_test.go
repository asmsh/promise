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
	"reflect"
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

type waitTriggerTestMode int

const (
	noWaitTrigger waitTriggerTestMode = iota
	groupWaitTrigger
	promiseWaitTrigger
)

type promiseTestCase[T any] struct {
	delay time.Duration
	res   Result[T]

	// promise is optional, and if not set, one will be
	// generated from the above scenario.
	promise *Promise[T]
}

type groupWaitTestCall struct {
	// callDelay is the sleep duration before calling
	// the function in question.
	callDelay time.Duration

	waitMode       waitTriggerTestMode
	waitPromiseIdx int

	// lowerDuration and upperDuration define the duration
	// range which the function in question is allowed to take,
	// which includes the time consumed by any wait trigger as well.
	lowerDuration time.Duration
	upperDuration time.Duration
}

type groupResTestCall[GroupResT any] struct {
	// callDelay is the sleep duration before calling
	// the function in question.
	callDelay time.Duration

	waitMode       waitTriggerTestMode
	waitPromiseIdx int

	// lowerDuration and upperDuration define the duration
	// range which the function in question is allowed to take.
	lowerDuration time.Duration
	upperDuration time.Duration

	wantState  State
	wantResVal GroupResT
}

type groupWaitTestCase[PromiseResT any] struct {
	name     string
	config   *GroupConfig
	promises []*promiseTestCase[PromiseResT]
	calls    []groupWaitTestCall
}

type groupResTestCase[PromiseResT, GroupResT any] struct {
	name     string
	config   *GroupConfig
	promises []*promiseTestCase[PromiseResT]
	calls    []groupResTestCall[GroupResT]
}

func groupWaitTestHelper[PromiseResT any](
	t *testing.T,
	tc groupWaitTestCase[PromiseResT],
	fn func(g *Group[PromiseResT], triggers ...func()),
	fnName string,
) {
	t.Parallel()

	var g *Group[PromiseResT]
	if tc.config != nil {
		g = NewGroup[PromiseResT](ApplyConfig(*tc.config))
	} else {
		g = NewGroup[PromiseResT]()
	}

	for _, pp := range tc.promises {
		if pp.promise != nil {
			continue
		}

		pp.promise = g.GoRes(func(ctx context.Context) Result[PromiseResT] {
			time.Sleep(pp.delay)
			return pp.res
		})
	}

	for tccIdx, tcc := range tc.calls {
		if tcc.callDelay != 0 {
			time.Sleep(tcc.callDelay)
		}

		start := time.Now()
		switch tcc.waitMode {
		default: // also == 'noWaitTrigger'
			fn(g)
		case groupWaitTrigger:
			fn(g, func() {
				g.Wait()
			})
		case promiseWaitTrigger:
			fn(g, func() {
				tc.promises[tcc.waitPromiseIdx].promise.Wait()
			})
		}
		elapsed := time.Since(start)

		if elapsed < tcc.lowerDuration {
			t.Errorf(
				"Call[%d] %s() returned early. expected: %s, got: %s",
				tccIdx,
				fnName,
				tcc.lowerDuration,
				elapsed,
			)
		}

		if elapsed > tcc.upperDuration {
			t.Errorf(
				"Call[%d] %s() waited so long. expected: %s, got: %s",
				tccIdx,
				fnName,
				tcc.upperDuration,
				elapsed,
			)
		}
	}
}

func groupResTestHelper[PromiseResT, GroupResT any](
	t *testing.T,
	tc groupResTestCase[PromiseResT, GroupResT],
	fn func(g *Group[PromiseResT], triggers ...func()) Result[GroupResT],
	fnName string,
) {
	t.Parallel()

	var g *Group[PromiseResT]
	if tc.config != nil {
		g = NewGroup[PromiseResT](ApplyConfig(*tc.config))
	} else {
		g = NewGroup[PromiseResT]()
	}

	for _, pp := range tc.promises {
		if pp.promise != nil {
			continue
		}

		pp.promise = g.GoRes(func(ctx context.Context) Result[PromiseResT] {
			time.Sleep(pp.delay)
			return pp.res
		})
	}

	for tccIdx, tcc := range tc.calls {
		if tcc.callDelay != 0 {
			time.Sleep(tcc.callDelay)
		}

		start := time.Now()
		var res Result[GroupResT]
		switch tcc.waitMode {
		default: // also == 'noWaitTrigger'
			res = fn(g)
		case groupWaitTrigger:
			res = fn(g, func() {
				g.Wait()
			})
		case promiseWaitTrigger:
			res = fn(g, func() {
				tc.promises[tcc.waitPromiseIdx].promise.Wait()
			})
		}
		elapsed := time.Since(start)

		if elapsed < tcc.lowerDuration {
			t.Errorf(
				"Call[%d] %s() returned early. expected: %s, got: %s",
				tccIdx,
				fnName,
				tcc.lowerDuration,
				elapsed,
			)
		}

		if elapsed > tcc.upperDuration {
			t.Errorf(
				"Call[%d] %s() waited so long. expected: %s, got: %s",
				tccIdx,
				fnName,
				tcc.upperDuration,
				elapsed,
			)
		}

		if res.State() != tcc.wantState {
			t.Errorf(
				"Call[%d] %s().State() is %s, want %s",
				tccIdx,
				fnName,
				res.State(),
				tcc.wantState,
			)
		}

		if !reflect.DeepEqual(res.Val(), tcc.wantResVal) {
			t.Errorf(
				"Call[%d] %s().Val() is %v, want %#v",
				tccIdx,
				fnName,
				res.Val(),
				tcc.wantResVal,
			)
		}
	}
}

func TestGroup_Wait(t *testing.T) {
	tests := []groupWaitTestCase[string]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupWaitTestCall{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 1 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 2 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 50 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupWaitTestCall{
				{
					lowerDuration: 50 * time.Millisecond, // max duration value.
					upperDuration: 55 * time.Millisecond, // max duration value + a 5ms buffer duration.
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupWaitTestHelper(t, tc, (*Group[string]).Wait, "Wait")
		})
	}
}

func TestGroup_SelectRes(t *testing.T) {
	tests := []groupResTestCase[string, GroupRes[string]]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupResTestCall[GroupRes[string]]{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
					wantState:     Success,
					wantResVal:    GroupRes[string]{},
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
			},
		},
		{
			name: "second call consistency",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
				{
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:      Panic,
					wantResVal:     GroupRes[string]{Result: PanicRes[string]("promise1")},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise1")},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise1")},
				},
			},
		},
		{
			name: "all result - no wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
			},
		},
		{
			name: "all result - group wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
			},
		},
		{
			name: "all result - promise wait trigger 1",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:      Panic,
					wantResVal:     GroupRes[string]{Result: PanicRes[string]("promise1")},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal:    GroupRes[string]{Result: PanicRes[string]("promise1")},
				},
			},
		},
		{
			name: "all result - promise wait trigger 2",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise1")},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise1")},
				},
			},
		},
		{
			name: "last result - no wait trigger",
			config: &GroupConfig{
				SaveLastSingleGroupResult: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{Result: ErrRes[string](testStrError("promise1"))},
				},
				{
					callDelay:     100 * time.Millisecond, // after promise2.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise2")},
				},
				{
					callDelay:     500 * time.Millisecond, // after promise3.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise3")},
				},
			},
		},
		{
			name: "last result - group wait trigger",
			config: &GroupConfig{
				SaveLastSingleGroupResult: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise3")},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise3")},
				},
			},
		},
		{
			name: "last result - promise wait trigger",
			config: &GroupConfig{
				SaveLastSingleGroupResult: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:      Panic,
					wantResVal:     GroupRes[string]{Result: PanicRes[string]("promise1")},
				},
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  85 * time.Millisecond, // promise2 - promise1 with 5ms buffer duration.
					upperDuration:  95 * time.Millisecond, // promise2 - promise1 + a 5ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise2")},
				},
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					lowerDuration:  395 * time.Millisecond, // promise3 - promise2 with 5ms buffer duration.
					upperDuration:  405 * time.Millisecond, // promise3 - promise2 + a 5ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise3")},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupResTestHelper(t, tc, (*Group[string]).SelectRes, "SelectRes")
		})
	}
}

func TestGroup_AllRes(t *testing.T) {
	tests := []groupResTestCase[string, []GroupRes[string]]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
					wantState:     Success,
					wantResVal:    nil,
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error,
					// unless that one is Panic.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string]("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error,
					// unless that one is Panic.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
			},
		},
		{
			name: "success flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises to return, as all
					// of them are Success and it only breaks on [Error],
					// and return all of their results as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after all promises have
					// returned, and use the group history to return the correct
					// result state, as if it was running from the start.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Panic promise
					// return, but yet return its state (because it's the
					// highest among the above promises), and it should
					// return on the Error promise, without waiting for
					// the Success one.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:      Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Error promise
					// return, but since there was a Panic promise, before it,
					// it will ignore the Error promise.
					// so, it returns the Panic result (highest [State])
					// and the Success result (observed by this call).
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:      Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - no wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Error.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Error,                 // the only one observed.
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
			},
		},
		{
			name: "all result - group wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after all promises have returned,
					// but since we saved all results, it should return them.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 1",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					// it should start waiting after the first [Success] promise
					// have returned, but sine it's an [All] call, it returns
					// after [Error], and yet we should get all 3 results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 2",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					// it should start waiting after second promise have returned,
					// but sine it's an [All] call, it would return after Error, and
					// yet we should get all 3 results.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
				{
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - second call consistency",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					// it should start waiting after second promise have returned,
					// but sine it's an [All] call, it would return after Error, and
					// yet we should get all 3 results.
					lowerDuration: 500 * time.Millisecond, // promise2 sleep.
					upperDuration: 505 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupResTestHelper(t, tc, (*Group[string]).AllRes, "AllRes")
		})
	}
}

func TestGroup_AllWaitRes(t *testing.T) {
	tests := []groupResTestCase[string, []GroupRes[string]]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
					wantState:     Success,
					wantResVal:    nil,
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, but yet return Error.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, but return Panic, even
					// though an Error happened, because Panic is higher than Error.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with only the [Panic] result.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string]("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, return their results,
					// but as [Panic].
					lowerDuration: 500 * time.Millisecond, // promise1 sleep.
					upperDuration: 505 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: PanicRes[string]("promise3")},
					},
				},
				{
					// it should return immediately with only the [Panic] result.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise3")},
					},
				},
			},
		},
		{
			name: "success flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises to return, as all
					// of them are Success and it only breaks on [Error],
					// and return all of their results as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode: groupWaitTrigger,
					// it should only start waiting after all promises have
					// returned, and use the group history to return the correct
					// result state, as if it was running from the start.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					// it should only start waiting after the Panic promise
					// return, but yet return its state (because it's the
					// highest among the above promises), and it should
					// return after all promises are done.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{ // state's result + observed result.
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					// it should only start waiting after the Error promise
					// return, which means it skips its result, and then
					// return after all promises are done.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{ // state's result + observed result.
						{Result: PanicRes[string]("promise1")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "all result - no wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should return after all promises return with their results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode: groupWaitTrigger,
					// it should start waiting after all promises have returned,
					// but since we saved all results, it should return them.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					// it should start waiting after the first [Success] promise
					// have returned, but sine it's an [AllWait] call, it returns
					// after all the promises return.
					// it should return [Error], as it's the highest [State], with
					// all results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					// it should start waiting after the second [Success] promise
					// have returned, but sine it's an [AllWait] call, it returns
					// after all the promises returns.
					// it should return [Error], as it's the highest [State], with
					// all results, even the first [Success] promise's.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupResTestHelper(t, tc, (*Group[string]).AllWaitRes, "AllWaitRes")
		})
	}
}

func TestGroup_AnyRes(t *testing.T) {
	tests := []groupResTestCase[string, []GroupRes[string]]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
					wantState:     Success,
					wantResVal:    nil,
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Success.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Success,
					// unless that one is Panic.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string]("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises once one is Success,
					// unless that one is Panic.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 15 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
			},
		},
		{
			name: "error flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises to return, as all
					// of them are Error and it only breaks on [Success],
					// and return all of their results as [Error].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode: groupWaitTrigger,
					// it should only start waiting after all promises have
					// returned, and use the group history to return the correct
					// result state, as if it was running from the start.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: PanicRes[string]("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Error promise
					// return, and return on Success, without waiting for
					// the Panic one.
					// it should return a Success result, as it won't observe
					// the Panic promise.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
				},
				{
					// it starts waiting after the Panic promise returns,
					// so it becomes the highest state, and should be returned.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					lowerDuration:  395 * time.Millisecond, // promise3 - promise2 with a 5ms buffer duration.
					upperDuration:  405 * time.Millisecond, // a 5ms buffer duration.
					wantState:      Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise3")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					// it should only start waiting after the Error promise
					// return, ignore its result, but yet return immediately
					// after it, with the first promise, as it's a Success.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
				{
					// it should only return the first Success.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
			},
		},
		{
			name: "all result - no wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should return on the first Success, include the
					// Error result, and have a Success [State].
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
					},
				},
				{
					// since there's no wait duration, it should return the
					// exact result in the previous call, almost instantly.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
					},
				},
				{
					// since we wait after the last promise return, it should
					// be included in the result.
					callDelay:     500 * time.Millisecond, // promise3 - promise2 wait.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - group wait trigger",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after all promises have returned,
					// but since we saved all results, it should return them.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					// since no wait duration, it should be the same as
					// the previous call.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 1",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after the first [Success] promise
					// have returned, so it should return immediately with
					// its result only.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10,                    // promise2 sleep.
					upperDuration:  15 * time.Millisecond, // promise2  sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
				{
					// since it waits after the [Error] promise returns,
					// it should include all results in the returned result,
					// without changing the result [State].
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 2",
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after [Error] promise returns,
					// but since there was a [Success] resolved before, it
					// should return immediately, with the first Success and
					// the Error results, as we're saving all results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100,                    // promise2 sleep.
					upperDuration:  105 * time.Millisecond, // promise2 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
				{
					// it should return the same result as the previous call.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupResTestHelper(t, tc, (*Group[string]).AnyRes, "AnyRes")
		})
	}
}

func TestGroup_AnyWaitRes(t *testing.T) {
	tests := []groupResTestCase[string, []GroupRes[string]]{
		{
			name:     "zero group",
			promises: nil,
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					lowerDuration: 0,
					upperDuration: 1 * time.Millisecond, // an OK delay for the test env.
					wantState:     Success,
					wantResVal:    nil,
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for the last [Success], and return all results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for the last [Success], return all
					// results, but as [Panic].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with only the [Panic] result.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string]("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, return their results,
					// but as [Panic].
					lowerDuration: 500 * time.Millisecond, // promise1 sleep.
					upperDuration: 505 * time.Millisecond, // promise1 sleep + a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: PanicRes[string]("promise3")},
					},
				},
				{
					// it should return immediately with only the [Panic] result.
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise3")},
					},
				},
			},
		},
		{
			name: "error flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises to return, as all
					// of them are Error and it only breaks on [Success],
					// and return all of their results as [Error].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
					},
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// result state, as if it was running from the start.
					// returned, and use the group history to return the correct
					// it should only start waiting after all promises have
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Panic promise
					// return, but yet return its state (because it's the
					// highest among the above promises), and it should
					// return on the Success promise, and include all results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:      Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{ // state's result + observed result.
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{ // state's result.
						{Result: PanicRes[string]("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the seconds [Error]
					// promise return, which means it skips its result and
					// previous ones, and then return after the Success promise
					// is done, with its result only.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - no wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should return after all promises return with their results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after all promises have returned,
					// but since we saved all results, it should return them.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after the first [Success] promise
					// have returned, but sine it's an [AnyWait] call, it returns
					// after all the promises return.
					// it should return [Success], as it's the highest [State], with
					// all results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
		{
			name: "all result - promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			config: &GroupConfig{
				SaveAllGroupResults: true,
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					// it should start waiting after the [Error] promise
					// returns, but yet return all results as [Success],
					// as we're saving all results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 505 * time.Millisecond, // promise3 sleep + a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 5 * time.Millisecond, // a 5ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groupResTestHelper(t, tc, (*Group[string]).AnyWaitRes, "AnyWaitRes")
		})
	}
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
		upperDuration := 55 * time.Millisecond // max(1, 2, 50) + a 5ms buffer duration.

		start := time.Now()
		res := g.JoinRes()
		elapsed := time.Since(start)

		if elapsed < lowerDuration {
			t.Errorf("JoinRes() returned early. expected: %s, got: %s", lowerDuration, elapsed)
		}

		if elapsed > upperDuration {
			t.Errorf("JoinRes() waited so long. expected: %s, got: %s", upperDuration, elapsed)
		}

		if want := Success; res.State() != want {
			t.Errorf("JoinRes().State() is %s, want %s", res.State(), want)
		}
	})
}
