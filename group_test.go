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
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

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

	wantState   State
	wantResVal  GroupResT
	wantResErrs []error
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

		pp.promise = g.GoCtxRes(func(ctx context.Context) Result[PromiseResT] {
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

		pp.promise = g.GoCtxRes(func(ctx context.Context) Result[PromiseResT] {
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

		if gotResState := res.State(); gotResState != tcc.wantState {
			t.Errorf(
				"Call[%d] %s().State() is %s, want %s",
				tccIdx,
				fnName,
				gotResState,
				tcc.wantState,
			)
		}

		if gotResVal := res.Val(); !reflect.DeepEqual(gotResVal, tcc.wantResVal) {
			t.Errorf(
				"Call[%d] %s().Val() is %#v, want %#v",
				tccIdx,
				fnName,
				gotResVal,
				tcc.wantResVal,
			)
		}

		gotResErr := res.Err()
		if len(tcc.wantResErrs) != 0 {
			for i, wantResErr := range tcc.wantResErrs {
				if !errors.Is(gotResErr, wantResErr) {
					t.Errorf(
						"Call[%d] %s().Err() is %#v, want[%d] %#v",
						tccIdx,
						fnName,
						gotResErr,
						i,
						wantResErr,
					)
				}
			}
		} else {
			if !errors.Is(gotResErr, nil) {
				t.Errorf(
					"Call[%d] %s().Err() is %#v, want %#v",
					tccIdx,
					fnName,
					gotResErr,
					nil,
				)
			}
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
					upperDuration: 70 * time.Millisecond, // max duration value + a 20ms buffer duration.
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
					wantResErrs:   nil,
				},
			},
		},
		{
			name: "basic flow",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValErrRes("promise1", testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
			},
		},
		{
			name: "second call consistency",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: PanicRes[string]("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:      Panic,
					wantResVal:     GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
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
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise1")},
					wantResErrs:    nil,
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise1")},
					wantResErrs:   nil,
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
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
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
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
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
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:      Panic,
					wantResVal:     GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    GroupRes[string]{},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
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
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[GroupRes[string]]{
				{
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal:     GroupRes[string]{Result: ValRes("promise1")},
					wantResErrs:    nil,
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal:    GroupRes[string]{Result: ValRes("promise1")},
					wantResErrs:   nil,
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
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					// the second call should return only the first Error.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises after Panic.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it shouldn't wait for other promises after Error.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: []error{
						testStrError("promise2"),
					},
				},
				{
					// the second call now observes the Panic, and since
					// Panic is higher in priority than Error, it returns
					// only the Panic Result.
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise3"),
						panicResult[string]{testStrError("promise3")},
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
					// of them are Success, and return all as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should return only the first Success.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start after all are done, and return
					// the panic error from the history.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Panic promise
					// return, but yet return its state (because it's the
					// highest among the above promises), and it should
					// return without waiting with the already-resolved
					// Panic promise.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  10 * time.Millisecond, // promise1 sleep.
					upperDuration:  30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:      Panic,                 // the highest state.
					wantResVal:     nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after promise2, but since
					// there was a Panic promise before it, it returns the Panic
					// error from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Panic,
					wantResVal:     nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
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
					// it shouldn't wait for other promises after Error.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Error,                 // the only one observed.
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
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
					// have returned, but since it's an [All] call, it returns
					// after [Error], and yet we should get all 3 results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
					},
				},
				{
					// the second call returns all results, since we saved all results.
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
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
				{delay: 100 * time.Millisecond, res: PanicRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after promise2 is done, and return
					// immediately with the Panic result, and also the Success
					// result from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Panic,                  // since we return only after Panic.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: []error{
						testStrError("promise2"),
						panicResult[string]{testStrError("promise2")},
					},
				},
				{
					// it waits until promise3 is done, and should return immediately
					// with all results from the history.
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise2"),
						panicResult[string]{testStrError("promise2")},
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
					// it should start waiting after second promise have returned,
					// but sine it's an [All] call, it would return after Error,
					// so, we should get all 3 results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise2 sleep.
					upperDuration:  520 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
					},
				},
				{
					// the seconds call should get all results, since we saved them.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
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
					// it should wait for all promises, and return Error with
					// the Error values promise1 and promise3.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					// it should return Error immediately with the error promise1
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
			},
		},
		{
			name: "panic flow - early",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, and return Panic
					// with the single value promise3, and the Panic value
					// promise1, instead of the Error value promise2, as
					// Panic is higher than Error.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					// it should return immediately with only the Panic error
					// promise1 from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "panic flow - late",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: PanicRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should wait for all promises, and return Panic with
					// the single value promise1, and the Panic value promise2.
					lowerDuration: 500 * time.Millisecond, // promise1 sleep.
					upperDuration: 520 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Panic,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
						panicResult[string]{testStrError("promise3")},
					},
				},
				{
					// it should return immediately with only the [Panic] error.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise3"),
						panicResult[string]{testStrError("promise3")},
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
			},
		},
		{
			name: "group wait trigger",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after all promises have
					// returned, and return the Panic, with the panic error
					// promise1 from the history.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 1",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Panic promise
					// return, but yet return its state (because it's the
					// highest among the above promises), and it should
					// return after all promises are done.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
						testStrError("promise2"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should only start waiting after the Error promise
					// return, which means it skips its result, and then
					// return after all promises are done.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Panic,                  // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Panic,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: []error{
						testStrError("promise1"),
						panicResult[string]{testStrError("promise1")},
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
					// have returned, but sine it's an [AllWait] call, it returns
					// after all the promises return.
					// it should return [Error], as it's the highest [State], with
					// all results.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
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
					// it should start waiting after the second [Success] promise
					// have returned, but sine it's an [AllWait] call, it returns
					// after all the promises returns.
					// it should return [Error], as it's the highest [State], with
					// all results, even the first [Success] promise's.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Error,                  // since we return only after Error.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
					},
				},
				{
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,                // same as the first call.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: []error{
						testStrError("promise3"),
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
					// it shouldn't wait for other promises after Success,
					// and should return only the single Success Result.
					lowerDuration: 100 * time.Millisecond, // promise2 sleep.
					upperDuration: 120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should return the single Success Result.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// it shouldn't wait for other promises after Success,
					// and should return only the single Success Result.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should return the single Success Result.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					// it shouldn't wait for other promises after Success,
					// and should return only the single Success Result.
					lowerDuration: 10 * time.Millisecond, // promise1 sleep.
					upperDuration: 30 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should return the single Success Result.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise3"),
					},
				},
				{
					// the seconds call should return only the first Error result
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
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
					// returned, and use the group history to return the single
					// Success result state.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// the seconds call should return only the single Success result
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should start waiting after the Panic promise
					// returns, and still return the single Success result
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					lowerDuration:  380 * time.Millisecond, // promise3 - promise2 with a 20ms buffer duration.
					upperDuration:  420 * time.Millisecond, // a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// it should only start waiting after the Error promise
					// return, and return immediately with the Success before
					// it from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100 * time.Millisecond, // promise2 sleep.
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
				{
					// the second call should return only the first Success from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
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
					upperDuration: 120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// since there's no wait duration, it should return the
					// exact result in the previous call, almost instantly.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// since we wait after the last promise return, it should
					// be included in the result.
					callDelay:     500 * time.Millisecond, // promise3 - promise2 wait.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// since no wait duration, it should be the same as
					// the previous call.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					upperDuration:  30 * time.Millisecond, // promise2  sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
				{
					// since it waits after the [Error] promise returns,
					// it should include all results in the returned result,
					// without changing the result [State].
					callDelay:     500 * time.Millisecond, // after promise3 returns.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// should return immediately, with the first Success.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  100,                    // promise2 sleep.
					upperDuration:  120 * time.Millisecond, // promise2 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
				},
				{
					// it should return the same result as the previous call.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
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
					wantResErrs:   nil,
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
					// it should wait for all, and return only Success results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with promise2 from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// it should wait for all, and return only Success results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with only promise3 from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					// it should wait for all, and return only Success results.
					lowerDuration: 500 * time.Millisecond, // promise1 sleep.
					upperDuration: 520 * time.Millisecond, // promise1 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with only promise3 from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
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
					// it should wait for all, and return all of them as [Error].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
						testStrError("promise2"),
						testStrError("promise3"),
					},
				},
				{
					// it should return immediately with promise1 error from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Error,
					wantResVal:    nil,
					wantResErrs: []error{
						testStrError("promise1"),
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
					// it should start after all are done, and return the only
					// Success result from the history.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the only Success result
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
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
					// it should start after promise1 is done, wait for all,
					// and return the only Success result.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with the only Success result
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
			},
		},
		{
			name: "promise wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ValRes("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start after promise2 is done, wait for all,
					// and return the only Success result from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with the only Success result
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
					},
					wantResErrs: nil,
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
					// it should return after all, and return all Success results.
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with all Success results
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					// it should start waiting after all are done, and return
					// only Success results.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,                // the highest state.
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with all Success results
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
					wantResErrs: nil,
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
					// it should start after promise1 is done, wait for all,
					// and return all Success results, including promise1 from
					// the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with all Success results
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// it should start after promise3 is done, and return all
					// Success results from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
				},
				{
					// it should return immediately with all Success results
					// from the history.
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
					},
					wantResErrs: nil,
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
					// it should wait for all promises, return their results,
					// as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are 'promise1' and 'promise2',
					// as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
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
					// it should wait for all promises, return their results,
					// as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are all of them, as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
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
					// it should wait for all promises, return their results,
					// as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: PanicRes[string]("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are all of them, as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
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
					// it should wait for all promises, return their results,
					// as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which is promise1, as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
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
					// it should start waiting after all promises are done,
					// and return the first unique [Result] values from the
					// history, which are all of them, as [Success]..
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are all of them, as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "group wait trigger 2",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: PanicRes[string]("promise1")},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after all promises are done,
					// and return the first unique [Result] values from the
					// history, which are promise1 and promise2, as [Success]..
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are promise1 and promise2,
					// as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
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
					// it should start waiting only after promise1 returns,
					// and yet return all results as [Success], including
					// promise1 (from the history).
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are all of them, as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string]("promise1")},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
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
					// it should start waiting only after promise2 returns,
					// and yet return only promise1 and promise3 results,
					// as [Success], with promise1 coming from the history,
					// and no promise2 as it's duplicate state of promise1.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 1,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are promise1 and promise3,
					// as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise3")},
					},
				},
			},
		},
		{
			name: "promise wait trigger 3",
			promises: []*promiseTestCase[string]{
				{delay: 10 * time.Millisecond, res: ErrRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ErrRes[string](testStrError("promise2"))},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting only after promise2 returns,
					// and return all results, as [Success], with promise1
					// coming from the history.
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ErrRes[string](testStrError("promise2"))},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with the first unique [Result]
					// values from the history, which are promise1 and promise3,
					// as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
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
					// it should wait for all promises, return their results,
					// as [Success].
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: ErrRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with all results as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
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
				{delay: 10 * time.Millisecond, res: PanicRes[string](testStrError("promise1"))},
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ValRes("promise3")},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after all promises have returned,
					// but since we saved all results, it should return them.
					waitMode:      groupWaitTrigger,
					lowerDuration: 500 * time.Millisecond, // promise3 sleep.
					upperDuration: 520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:     Success,
					wantResVal: []GroupRes[string]{
						{Result: PanicRes[string](testStrError("promise1"))},
						{Result: ValRes("promise2")},
						{Result: ValRes("promise3")},
					},
				},
				{
					// it should return immediately with all results as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
					wantState:     Success,
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
					// have returned, wait all the promises to return, and return
					// all results as [Success].
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 0,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					// it should return immediately with all results as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
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
				{delay: 100 * time.Millisecond, res: ValRes("promise2")},
				{delay: 500 * time.Millisecond, res: ErrRes[string](testStrError("promise3"))},
			},
			calls: []groupResTestCall[[]GroupRes[string]]{
				{
					// it should start waiting after the second [Success] promise
					// have returned, wait all the promises to return, and return
					// all results as [Success].
					waitMode:       promiseWaitTrigger,
					waitPromiseIdx: 2,
					lowerDuration:  500 * time.Millisecond, // promise3 sleep.
					upperDuration:  520 * time.Millisecond, // promise3 sleep + a 20ms buffer duration.
					wantState:      Success,
					wantResVal: []GroupRes[string]{
						{Result: ValRes("promise1")},
						{Result: ValRes("promise2")},
						{Result: ErrRes[string](testStrError("promise3"))},
					},
				},
				{
					// it should return immediately with all results as [Success].
					lowerDuration: 0,
					upperDuration: 20 * time.Millisecond, // a 20ms buffer duration.
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
			groupResTestHelper(t, tc, (*Group[string]).JoinRes, "JoinRes")
		})
	}
}

// From: https://github.com/golang/go/issues/54045
func TestGroupDoesNotRunNewFunctionsAfterPreviousError(t *testing.T) {
	group := NewGroup[any](Size(1), FailuresCancelGroup())

	group.GoCtxRes(func(ctx context.Context) Result[any] {
		return ErrRes[any](errors.New("test error"))
	}).Wait()

	var ran atomic.Bool
	group.GoCtxRes(func(ctx context.Context) Result[any] {
		ran.Store(true)
		return nil
	})
	group.Wait()
	if ran.Load() {
		t.Errorf("Expected second func not to run, but it did")
	}
}
