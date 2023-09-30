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

func BenchmarkNew(b *testing.B) {
	resChan := make(chan Result[any], 1)

	b.Run("nil ctx nil pipeline", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = New[any](nil, resChan, nil)
		}
		_ = p
	})

	b.Run("non-nil ctx nil pipeline", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = New[any](context.Background(), resChan, nil)
		}
		_ = p
	})

	b.Run("non-nil ctx default pipeline", func(b *testing.B) {
		var pp Pipeline[any]
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = New[any](context.Background(), resChan, &pp)
		}
		_ = p
	})

	b.Run("non-nil ctx custom pipeline", func(b *testing.B) {
		var pp = NewPipeline[any](&PipelineConfig{})
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = New[any](context.Background(), resChan, pp)
		}
		_ = p
	})
}

func BenchmarkChan(b *testing.B) {
	resChan := make(chan Result[any], 1)

	b.Run("nil ctx", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Chan[any](nil, resChan)
		}
		_ = p
	})

	b.Run("non-nil ctx", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Chan[any](context.Background(), resChan)
		}
		_ = p
	})
}

func BenchmarkGo(b *testing.B) {
	b.Run("nil ctx", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Go(nil, func() {})
		}
		_ = p
	})

	b.Run("non-nil ctx", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Go(context.Background(), func() {})
		}
		_ = p
	})
}

func BenchmarkGoErr(b *testing.B) {
	setNoPanicsPipelineCore()

	b.Run("nil ctx with nil error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(nil, func() error {
				return nil
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with nil error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(context.Background(), func() error {
				return nil
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with non-ptr error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(context.Background(), func() error {
				return newStrError()
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with ptr error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoErr(context.Background(), func() error {
				return newPtrError()
			})
		}
		_ = p
	})
}

func BenchmarkGoRes(b *testing.B) {
	setNoPanicsPipelineCore()

	b.Run("nil ctx with empty result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(nil, func(ctx context.Context) Result[any] {
				return Empty[any]()
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with empty result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(context.Background(), func(ctx context.Context) Result[any] {
				return Empty[any]()
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with value result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(context.Background(), func(ctx context.Context) Result[any] {
				return Val[any]("golang")
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with nil-error result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(context.Background(), func(ctx context.Context) Result[any] {
				return ValErr[any]("golang", nil)
			})
		}
		_ = p
	})

	b.Run("non-nil ctx with non-nil-error result", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = GoRes(context.Background(), func(ctx context.Context) Result[any] {
				return ValErr[any]("golang", newStrError())
			})
		}
		_ = p
	})
}

func BenchmarkResolver(b *testing.B) {
	setNoPanicsPipelineCore()

	b.Run("nil ctx fulfill", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Resolver[any](nil, func(ctx context.Context, fulfill func(val ...any), reject func(err error, val ...any)) {
				fulfill()
			})
		}
		_ = p
	})

	b.Run("non-nil ctx fulfill", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Resolver[any](context.Background(), func(ctx context.Context, fulfill func(val ...any), reject func(err error, val ...any)) {
				fulfill()
			})
		}
		_ = p
	})

	b.Run("non-nil ctx reject nil error", func(b *testing.B) {
		var p Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Resolver[any](context.Background(), func(ctx context.Context, fulfill func(val ...any), reject func(err error, val ...any)) {
				reject(nil, "success_value")
			})
		}
		_ = p
	})

	b.Run("non-nil ctx reject non-nil error", func(b *testing.B) {
		var p Promise[any]
		err := newStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p = Resolver[any](context.Background(), func(ctx context.Context, fulfill func(val ...any), reject func(err error, val ...any)) {
				reject(err, "success_value")
			})
		}
		_ = p
	})
}

func BenchmarkDelay(b *testing.B) {
	setNoPanicsPipelineCore()

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
