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
	"testing"
)

func BenchmarkNewPromAsync(b *testing.B) {
	b.Run("nil group", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromAsync[any](nil)
		}
		_ = p
	})

	b.Run("default group", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = newPromAsync[any](nil)
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
