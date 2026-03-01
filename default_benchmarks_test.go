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
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Chan[any](resChan)
		}
		_ = p
	})
}

func BenchmarkCtx(b *testing.B) {
	b.Run("empty-ctx", func(b *testing.B) {
		var p *Promise[any]
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Ctx(ctx)
		}
		_ = p
	})

	b.Run("non-empty-ctx", func(b *testing.B) {
		var p *Promise[any]
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Ctx(ctx)
		}
		_ = p
	})
}

func BenchmarkGo(b *testing.B) {
	b.Run("Success", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Go(func() {})
		}
		_ = p
	})

	b.Run("Panic", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Go(func() {
				panic("panic")
			})
		}
		_ = p
	})
}

func BenchmarkGoErr(b *testing.B) {
	b.Run("Success", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoErr(func() error { return nil })
		}
		_ = p
	})

	b.Run("Error", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoErr(func() error {
				return testStrError("test error")
			})
		}
		_ = p
	})

	b.Run("Panic", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoErr(func() error {
				panic("panic")
			})
		}
		_ = p
	})
}

func BenchmarkGoCtxRes(b *testing.B) {
	b.Run("empty result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoCtxRes(func(ctx context.Context) Result[any] {
				return ZeroRes[any]()
			})
		}
		_ = p
	})

	b.Run("value result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoCtxRes(func(ctx context.Context) Result[any] {
				return ValRes[any]("golang")
			})
		}
		_ = p
	})

	b.Run("nil-error result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoCtxRes(func(ctx context.Context) Result[any] {
				return ValErrRes[any]("golang", nil)
			})
		}
		_ = p
	})

	b.Run("non-nil-error result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoCtxRes(func(ctx context.Context) Result[any] {
				return ValErrRes[any]("golang", newTestStrError())
			})
		}
		_ = p
	})
}

func BenchmarkGoFunc(b *testing.B) {
	b.Run("nil error", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoFunc[any, any](func() error {
				return nil
			})
		}
		_ = p
	})

	b.Run("non-ptr error", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoFunc[any, int](func() error {
				return newTestStrError()
			})
		}
		_ = p
	})

	b.Run("ptr error", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = GoFunc[any, any](func() error {
				return newTestPtrError()
			})
		}
		_ = p
	})
}

func BenchmarkDelay(b *testing.B) {
	b.Run("empty result with no conditions", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Delay(ZeroRes[any](), time.Microsecond)
		}
		_ = p
	})

	b.Run("empty result with conditions", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Delay[any](nil, time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})

	b.Run("non-empty success result with conditions", func(b *testing.B) {
		var p *Promise[string]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Delay(ValRes("golang"), time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})

	b.Run("non-empty failed result with conditions", func(b *testing.B) {
		var p *Promise[string]
		var err = newTestStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Delay(ErrRes[string](err), time.Microsecond, OnSuccess, OnError, OnPanic, OnAll)
		}
		_ = p
	})
}

func BenchmarkWrap(b *testing.B) {
	b.Run("nil result", func(b *testing.B) {
		var p *Promise[any]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Wrap[any](nil)
		}
		_ = p
	})

	b.Run("Success result", func(b *testing.B) {
		var p *Promise[string]
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Wrap(ValRes("golang"))
		}
		_ = p
	})

	b.Run("Error result", func(b *testing.B) {
		var p *Promise[string]
		var err = newTestStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Wrap(ErrRes[string](err))
		}
		_ = p
	})

	b.Run("Panic result", func(b *testing.B) {
		var p *Promise[string]
		var err = newTestStrError()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p = Wrap(PanicRes[string](err))
		}
		_ = p
	})
}
