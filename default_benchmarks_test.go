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
	"time"
)

func BenchmarkChan(b *testing.B) {
	resChan := make(chan Result[any], 1)

	b.Run("", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Chan[any](resChan)
		}
		_ = p
	})
}

func BenchmarkGo(b *testing.B) {
	b.Run("", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Go(func() {})
		}
		_ = p
	})
}

func BenchmarkGoErr(b *testing.B) {
	b.Run("nil error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(func() error {
				return nil
			})
		}
		_ = p
	})

	b.Run("non-ptr error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(func() error {
				return newStrError()
			})
		}
		_ = p
	})

	b.Run("ptr error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(func() error {
				return newPtrError()
			})
		}
		_ = p
	})
}

func BenchmarkGoRes(b *testing.B) {
	b.Run("empty result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(func(ctx context.Context) Result[any] {
				return Empty[any]()
			})
		}
		_ = p
	})

	b.Run("value result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(func(ctx context.Context) Result[any] {
				return Val[any]("golang")
			})
		}
		_ = p
	})

	b.Run("nil-error result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(func(ctx context.Context) Result[any] {
				return ValErr[any]("golang", nil)
			})
		}
		_ = p
	})

	b.Run("non-nil-error result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(func(ctx context.Context) Result[any] {
				return ValErr[any]("golang", newStrError())
			})
		}
		_ = p
	})
}

func BenchmarkDelay(b *testing.B) {
	b.Run("empty result with no conditions", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Delay(Empty[any](), time.Microsecond)
		}
		_ = p
	})

	b.Run("empty result with conditions", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Delay[any](nil, time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})

	b.Run("non-empty success result with conditions", func(b *testing.B) {
		var p Promise[string]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Delay(Val("golang"), time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})

	b.Run("non-empty failed result with conditions", func(b *testing.B) {
		var p Promise[string]
		var err = newStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Delay(Err[string](err), time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})
}

func BenchmarkWrap(b *testing.B) {
	b.Run("empty result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Wrap[any](nil)
		}
		_ = p
	})

	b.Run("non-empty success result", func(b *testing.B) {
		var p Promise[string]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Wrap(Val("golang"))
		}
		_ = p
	})

	b.Run("non-empty failed result", func(b *testing.B) {
		var p Promise[string]
		var err = newStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Wrap(Err[string](err))
		}
		_ = p
	})
}

func BenchmarkPanic(b *testing.B) {
	b.Run("nil val", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Panic[any](nil)
		}
		_ = p
	})

	b.Run("non-nil val", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Panic[any]("golang")
		}
		_ = p
	})
}
