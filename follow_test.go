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

func TestFollow(t *testing.T) {
	t.Run("successful follow", func(t *testing.T) {
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

		fmt.Printf("pAny: %+v\n", pAny.Res())
		fmt.Printf("pStr: %+v\n", pStr.Res())
		fmt.Printf("pInt: %+v\n", pInt.Res())
	})
}

type numberStringToInt struct {
	fn func(ctx context.Context)
}

func (m numberStringToInt) Call(ctx context.Context, prevRes Result[string]) (nextRes Result[int]) {
	return ValErrRes(strconv.Atoi(prevRes.Val()))
}

func TestFollowCallback(t *testing.T) {
	t.Run("successful follow", func(t *testing.T) {
		pAny := GoCallback(CallbackFrom[*int, any](func(ctx context.Context) *int {
			v := 10
			return &v
		}))
		pStr := FollowCallback(
			pAny,
			CallbackFrom[string, *int](func(ctx context.Context, res Result[*int]) Result[string] {
				return ValRes(fmt.Sprintf("%d", *res.Val()))
			}),
		)
		pInt := FollowCallback[int](pStr, numberStringToInt{})

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		fmt.Printf("pAny: %+v\n", pAny.Res())
		fmt.Printf("pStr: %+v\n", pStr.Res())
		fmt.Printf("pInt: %+v\n", pInt.Res())
	})
}

func BenchmarkFollow(b *testing.B) {
	b.ReportAllocs()

	pBase := GoCtxRes(func(ctx context.Context) Result[string] {
		return ValRes("10")
	})
	for b.Loop() {
		pStr := Follow[int](
			pBase,
			func(ctx context.Context, res Result[string]) int {
				return 10
			},
		)
		pStr = pStr
	}
}

func BenchmarkFollowCallback(b *testing.B) {
	b.Run("Standard Callback", func(b *testing.B) {
		b.ReportAllocs()

		pBase := GoCtxRes(func(ctx context.Context) Result[string] {
			return ValRes("10")
		})
		for b.Loop() {
			pStr := FollowCallback(
				pBase,
				CallbackFrom[int, string](func() int {
					return 10
				}),
			)
			pStr = pStr
		}
	})

	b.Run("Custom Callback", func(b *testing.B) {
		b.ReportAllocs()

		pBase := GoCtxRes(func(ctx context.Context) Result[string] {
			return ValRes("10")
		})
		for b.Loop() {
			pStr := FollowCallback(pBase, numberStringToInt{})
			pStr = pStr
		}
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
			name: "GoErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() error { return nil })
			},
		},
		{
			name: "GoValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() any { return nil })
			},
		},
		{
			name: "GoValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() (any, error) { return nil, nil })
			},
		},
		{
			name: "GoResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func() Result[any] { return nil })
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
			name: "CtxErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) error { return nil })
			},
		},
		{
			name: "CtxValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) any { return nil })
			},
		},
		{
			name: "CtxValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) (any, error) { return nil, nil })
			},
		},
		{
			name: "CtxResFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context) Result[any] { return nil })
			},
		},
		// Follow callbacks.
		{
			name: "FollowFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) {})
			},
		},
		{
			name: "FollowErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) error { return nil })
			},
		},
		{
			name: "FollowValFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) any { return nil })
			},
		},
		{
			name: "FollowValErrFunc",
			constr: func() Callback[any, any] {
				return CallbackFrom[any, any](func(context.Context, Result[any]) (any, error) { return nil, nil })
			},
		},
		{
			name: "FollowResFunc",
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
