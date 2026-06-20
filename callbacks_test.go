// Copyright 2026 Ahmad Sameh(asmsh)
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
	"testing"
)

func BenchmarkCallbackFrom(b *testing.B) {
	b.Run("Standard Callback", func(b *testing.B) {
		var cb callback[int, string]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			cb = callbackFrom[int, string](func() int {
				return 10
			})
		}
		_ = cb
	})
}

// TestCallbackFrom_SupportedSignatures makes sure that all supported signatures
// included in the [Func] constraint are also supported by [callbackFrom].
func TestCallbackFrom_SupportedSignatures(t *testing.T) {
	type testCase[NextT any, PrevT any] struct {
		name   string
		constr func() callback[NextT, PrevT]
	}
	tests := []testCase[any, any]{
		// Go callbacks.
		{
			name: "GoFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func() {})
			},
		},
		{
			name: "GoNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func() error { return nil })
			},
		},
		{
			name: "GoNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func() any { return nil })
			},
		},
		{
			name: "GoNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func() (any, error) { return nil, nil })
			},
		},
		{
			name: "GoNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func() Result[any] { return nil })
			},
		},

		// PrevVal callbacks.
		{
			name: "PrevValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(any) {})
			},
		},
		{
			name: "PrevValNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(any) error { return nil })
			},
		},
		{
			name: "PrevValNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(any) any { return nil })
			},
		},
		{
			name: "PrevValNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(any) (any, error) { return nil, nil })
			},
		},
		{
			name: "PrevValNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(any) Result[any] { return nil })
			},
		},

		// PrevRes callbacks.
		{
			name: "PrevResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(Result[any]) {})
			},
		},
		{
			name: "PrevResNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(Result[any]) error { return nil })
			},
		},
		{
			name: "PrevResNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(Result[any]) any { return nil })
			},
		},
		{
			name: "PrevResNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(Result[any]) (any, error) { return nil, nil })
			},
		},
		{
			name: "PrevResNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(Result[any]) Result[any] { return nil })
			},
		},

		// Ctx callbacks.
		{
			name: "CtxFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context) {})
			},
		},
		{
			name: "CtxNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context) error { return nil })
			},
		},
		{
			name: "CtxNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context) any { return nil })
			},
		},
		{
			name: "CtxNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context) (any, error) { return nil, nil })
			},
		},
		{
			name: "CtxNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context) Result[any] { return nil })
			},
		},

		// FollowVal callbacks.
		{
			name: "FollowValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, any) {})
			},
		},
		{
			name: "FollowValNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, any) error { return nil })
			},
		},
		{
			name: "FollowValNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, any) any { return nil })
			},
		},
		{
			name: "FollowValNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, any) (any, error) { return nil, nil })
			},
		},
		{
			name: "FollowValNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, any) Result[any] { return nil })
			},
		},

		// FollowRes callbacks.
		{
			name: "FollowResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, Result[any]) {})
			},
		},
		{
			name: "FollowResNextErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, Result[any]) error { return nil })
			},
		},
		{
			name: "FollowResNextValFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, Result[any]) any { return nil })
			},
		},
		{
			name: "FollowResNextValErrFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, Result[any]) (any, error) { return nil, nil })
			},
		},
		{
			name: "FollowResNextResFunc",
			constr: func() callback[any, any] {
				return callbackFrom[any, any](func(context.Context, Result[any]) Result[any] { return nil })
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
			if _, ok := got.(callback[any, any]); !ok {
				t.Errorf("CallbackFrom(): got %v, want %v", got, callback[any, any](nil))
			}
		})
	}
}
