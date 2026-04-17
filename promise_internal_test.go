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
	"testing"
)

// TestGetEffectiveNextRes_Flows reproduces flows that exercise the current
// getEffectiveNextRes behavior in the full promise pipeline.
func TestGetEffectiveNextRes_Flows(t *testing.T) {
	// Section 1: nextResP != nil
	// A callback that returns a concrete Result should win over any previous result.
	t.Run("Section1/normal_callback_return", func(t *testing.T) {
		res := GoCtxRes(func(ctx context.Context) Result[string] {
			return ValRes("ok")
		}).Res()

		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.State() != Success {
			t.Errorf("got state %v, want Success", res.State())
		}
		if res.Val() != "ok" {
			t.Errorf("got val %q, want 'ok'", res.Val())
		}
	})

	// Section 2: nextResP == nil && prevRes == nil
	// A nil previous Result should stay nil through pass-through handling.
	t.Run("Section2/nil_passthrough", func(t *testing.T) {
		// Go returns nil internally on completion.
		// Catch ignores Success and passes the nil Result down.
		res := Go(func() {}).
			Catch(func(ctx context.Context, res Result[any]) Result[any] {
				t.Fatal("Catch callback should not be executed")
				return nil
			}).Res()

		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.State() != Success {
			t.Errorf("got %v, want Success result for implicit success pass-through", res.State())
		}
		if res.Val() != nil {
			t.Errorf("got %v, want nil value", res.Val())
		}
	})

	// Section 3: nextResP == nil && prevRes != nil
	// When no new Result is returned, the previous Result is forwarded by
	// type assertion as Result[NextT].
	t.Run("Section3/same_type_passthrough", func(t *testing.T) {
		sentinelErr := newTestStrError("sentinel")
		res := GoErr(func() error {
			return sentinelErr
		}).Then(func(ctx context.Context, res Result[any]) Result[any] {
			t.Fatal("Then callback should not be executed on Error")
			return nil
		}).Res()

		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.State() != Error {
			t.Errorf("got state %v, want Error", res.State())
		}
		if !errors.Is(res.Err(), sentinelErr) {
			t.Errorf("got err %v, want %v", res.Err(), sentinelErr)
		}
	})
}

func BenchmarkNewPromInter(b *testing.B) {
	b.Run("nil group", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromInter[any](nil)
		}
		_ = p
	})

	b.Run("default group", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromInter[any](nil)
		}
		_ = p
	})
}

func BenchmarkNewPromCtx(b *testing.B) {
	b.Run("non-nil ctx", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromCtx[any](nil, context.Background())
		}
		_ = p
	})
}

func BenchmarkNewPromSync(b *testing.B) {
	b.Run("nil Result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromSync[any](nil, nil)
		}
		_ = p
	})
}

func BenchmarkNewPromBlocked(b *testing.B) {
	b.Run("", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromBlocked[any]()
		}
		_ = p
	})
}
