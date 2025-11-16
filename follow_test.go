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

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// intPtrToStringCB is used to test passing a custom implementation of [Callback].
type intPtrToStringCB struct{}

func (intPtrToStringCB) Call(_ context.Context, prevRes Result[*int]) (nextRes Result[string]) {
	return ValRes(fmt.Sprintf("%d", *prevRes.Val()))
}

// stringToIntCB is used to test passing a custom implementation of [Callback].
type stringToIntCB struct{}

func (stringToIntCB) Call(_ context.Context, prevRes Result[string]) (nextRes Result[int]) {
	return ValErrRes(strconv.Atoi(prevRes.Val()))
}

// testResult is used to test returning a custom implementation of [Result].
type testResult struct {
	val string
}

func (t testResult) Val() string { return t.val }
func (testResult) Err() error    { return nil }
func (testResult) State() State  { return Success }

func TestFollow(t *testing.T) {
	t.Run("standard Result", func(t *testing.T) {
		pAny := GoCtxRes(func(ctx context.Context) Result[*int] {
			v := 10
			return ValRes(&v)
		})
		pStr := Follow[string](
			pAny,
			func(ctx context.Context, res Result[*int]) Result[string] {
				return ValRes(fmt.Sprintf("%d", *res.Val()))
			},
		)
		pInt := Follow[int](
			pStr,
			func(ctx context.Context, res Result[string]) Result[int] {
				return ValErrRes(strconv.Atoi(res.Val()))
			},
		)

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		t.Logf("pAny: %+v\n", pAny.Res())
		t.Logf("pStr: %+v\n", pStr.Res())
		t.Logf("pInt: %+v\n", pInt.Res())
	})

	t.Run("custom Result", func(t *testing.T) {
		pAny := GoCtxRes(func(ctx context.Context) Result[*int] {
			v := 10
			return ValRes(&v)
		})
		pStr := Follow[string](
			pAny,
			func(ctx context.Context, res Result[*int]) Result[string] {
				return testResult{fmt.Sprintf("%d", *res.Val())}
			},
		)
		pInt := Follow[int](
			pStr,
			func(ctx context.Context, res Result[string]) Result[int] {
				return ValErrRes(strconv.Atoi(res.Val()))
			},
		)

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		t.Logf("pAny: %+v\n", pAny.Res())
		t.Logf("pStr: %+v\n", pStr.Res())
		t.Logf("pInt: %+v\n", pInt.Res())
	})
}

func TestFollowCallback(t *testing.T) {
	t.Run("standard Callback", func(t *testing.T) {
		pAny := GoCallback(
			CallbackFrom[*int, any](func(ctx context.Context) *int {
				v := 10
				return &v
			}),
		)
		pStr := FollowCallback(
			pAny,
			CallbackFrom[string, *int](func(res Result[*int]) Result[string] {
				return ValRes(fmt.Sprintf("%d", *res.Val()))
			}),
		)
		pInt := FollowCallback(
			pStr,
			CallbackFrom[int, string](func(res Result[string]) Result[int] {
				return ValErrRes(strconv.Atoi(res.Val()))
			}),
		)

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		t.Logf("pAny: %+v\n", pAny.Res())
		t.Logf("pStr: %+v\n", pStr.Res())
		t.Logf("pInt: %+v\n", pInt.Res())
	})

	t.Run("custom Callback", func(t *testing.T) {
		pAny := GoCallback(
			CallbackFrom[*int, any](func(ctx context.Context) *int {
				v := 10
				return &v
			}),
		)
		pStr := FollowCallback(pAny, intPtrToStringCB{})
		pInt := FollowCallback(pStr, stringToIntCB{})

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		t.Logf("pAny: %+v\n", pAny.Res())
		t.Logf("pStr: %+v\n", pStr.Res())
		t.Logf("pInt: %+v\n", pInt.Res())
	})
}

func BenchmarkFollow(b *testing.B) {
	var pStr *Promise[int]
	pBase := GoCtxRes(func(ctx context.Context) Result[string] {
		return ValRes("10")
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pStr = Follow[int](
			pBase,
			func(ctx context.Context, res Result[string]) int {
				return 10
			},
		)
	}
	_ = pStr
}

func BenchmarkFollowCallback(b *testing.B) {
	b.Run("Standard Callback", func(b *testing.B) {
		var pStr *Promise[int]
		pBase := GoCtxRes(func(ctx context.Context) Result[string] {
			return ValRes("10")
		})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			pStr = FollowCallback(
				pBase,
				CallbackFrom[int, string](func() int {
					return 10
				}),
			)
		}
		_ = pStr
	})

	b.Run("Custom Callback", func(b *testing.B) {
		var pStr *Promise[int]
		pBase := GoCtxRes(func(ctx context.Context) Result[string] {
			return ValRes("10")
		})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			pStr = FollowCallback(pBase, stringToIntCB{})
		}
		_ = pStr
	})
}

func BenchmarkCallbackFrom(b *testing.B) {
	b.Run("Standard Callback", func(b *testing.B) {
		var cb Callback[int, string]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			cb = CallbackFrom[int, string](func() int {
				return 10
			})
		}
		_ = cb
	})
}

// TestCallbackFrom_SupportedSignatures makes sure that all supported signatures
// included in the [CallbackFunc] constraint are also supported by [CallbackFrom].
func TestCallbackFrom_SupportedSignatures(t *testing.T) {
	type testCase[NextT any, PrevT any] struct {
		name   string
		constr func() Callback[NextT, PrevT]
	}
	tests := []testCase[any, any]{
		// Go callbacks.
		{
			name: "GoFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() {})
			},
		},
		{
			name: "GoNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() error { return nil })
			},
		},
		{
			name: "GoNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() any { return nil })
			},
		},
		{
			name: "GoNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() (any, error) { return nil, nil })
			},
		},
		{
			name: "GoNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() Result[any] { return nil })
			},
		},

		// PrevVal callbacks.
		{
			name: "PrevValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(any) {})
			},
		},
		{
			name: "PrevValNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(any) error { return nil })
			},
		},
		{
			name: "PrevValNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(any) any { return nil })
			},
		},
		{
			name: "PrevValNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(any) (any, error) { return nil, nil })
			},
		},
		{
			name: "PrevValNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(any) Result[any] { return nil })
			},
		},

		// PrevRes callbacks.
		{
			name: "PrevResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(Result[any]) {})
			},
		},
		{
			name: "PrevResNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(Result[any]) error { return nil })
			},
		},
		{
			name: "PrevResNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(Result[any]) any { return nil })
			},
		},
		{
			name: "PrevResNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(Result[any]) (any, error) { return nil, nil })
			},
		},
		{
			name: "PrevResNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(Result[any]) Result[any] { return nil })
			},
		},

		// Ctx callbacks.
		{
			name: "CtxFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) {})
			},
		},
		{
			name: "CtxNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) error { return nil })
			},
		},
		{
			name: "CtxNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) any { return nil })
			},
		},
		{
			name: "CtxNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) (any, error) { return nil, nil })
			},
		},
		{
			name: "CtxNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) Result[any] { return nil })
			},
		},

		// FollowVal callbacks.
		{
			name: "FollowValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, any) {})
			},
		},
		{
			name: "FollowValNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, any) error { return nil })
			},
		},
		{
			name: "FollowValNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, any) any { return nil })
			},
		},
		{
			name: "FollowValNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, any) (any, error) { return nil, nil })
			},
		},
		{
			name: "FollowValNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, any) Result[any] { return nil })
			},
		},

		// FollowRes callbacks.
		{
			name: "FollowResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) {})
			},
		},
		{
			name: "FollowResNextErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) error { return nil })
			},
		},
		{
			name: "FollowResNextValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) any { return nil })
			},
		},
		{
			name: "FollowResNextValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) (any, error) { return nil, nil })
			},
		},
		{
			name: "FollowResNextResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) Result[any] { return nil })
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("CallbackFrom() panicked: %v", r)
				}
			}()

			got := tt.constr()
			if got == nil {
				t.Errorf("CallbackFrom(): got nil (%#v)", got)
			}
			if _, ok := got.(Callback[any, any]); !ok {
				t.Errorf("CallbackFrom(): got %v, want %v", got, Callback[any, any](nil))
			}
		})
	}
}
