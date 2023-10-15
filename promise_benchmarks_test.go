// Copyright 2020 Ahmad Sameh(asmsh)
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

func getErrorBenchmarkPromise() Promise[any] {
	setNoPanicsPipelineCore()

	prom := GoRes(func(ctx context.Context) Result[any] {
		time.Sleep(1 * time.Millisecond)
		return Err[any](newStrError())
	})
	return prom
}

func getSuccessBenchmarkPromise(res ...any) Promise[any] {
	var resVal any
	if len(res) > 0 {
		resVal = res[0]
	}

	prom := GoRes(func(ctx context.Context) Result[any] {
		time.Sleep(1 * time.Millisecond)
		return Val(resVal)
	})
	return prom
}

func BenchmarkPromise_Wait(b *testing.B) {
	b.Run("resolved-sync", func(b *testing.B) {
		prom := Wrap[any](nil)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			prom.Wait()
		}
	})

	b.Run("resolved-async", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			prom.Wait()
		}
	})
}

func BenchmarkPromise_Res(b *testing.B) {
	b.Run("success-resolved-sync_nil-res", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](nil)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})

	b.Run("success-resolved-sync_non-nil-res", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](Val[any]("test test"))

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})

	b.Run("error-resolved-sync", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](Err[any](newStrError()))

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})

	b.Run("success-resolved-async_nil-res", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})

	b.Run("success-resolved-async_non-nil-res", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise(Val[any]("test test"))
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})

	b.Run("error-resolved-async", func(b *testing.B) {
		var res Result[any]
		prom := getErrorBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res = prom.Res()
		}

		_ = res
	})
}

type promiseBench struct {
	// stressed will call 'SetParallelism(100)', if true, otherwise it won't.
	// it's special for the parallel benchmarks only.
	stressed bool

	callFollow bool
	callWait   bool
	callRes    bool

	name string
}

const parallelism = 100

var promiseBenchs = []promiseBench{
	{stressed: false, callFollow: true, callWait: false, callRes: false, name: "normal_follow-only"},
	{stressed: false, callFollow: false, callWait: true, callRes: false, name: "normal_wait-only"},
	{stressed: false, callFollow: false, callWait: false, callRes: true, name: "normal_res-only"},
	{stressed: true, callFollow: true, callWait: true, callRes: false, name: "stressed_follow-only"},
	{stressed: true, callFollow: false, callWait: true, callRes: false, name: "stressed_wait-only"},
	{stressed: true, callFollow: false, callWait: false, callRes: true, name: "stressed_res-only"},
}

func BenchmarkPromise_Then(b *testing.B) {
	b.Run("no-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := getSuccessBenchmarkPromise()
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					if bc.callFollow {
						prom.Then(func(ctx context.Context, val any) Result[any] {
							return nil
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.Res()
					}
				}
			})
		}
	})

	b.Run("with-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := getSuccessBenchmarkPromise()
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					if bc.callFollow {
						prom.Then(func(ctx context.Context, val any) Result[any] {
							return Val[any]("golang")
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.Res()
					}
				}
			})
		}
	})
}

func BenchmarkPromise_Catch(b *testing.B) {
	b.Run("no-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := getErrorBenchmarkPromise()
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					if bc.callFollow {
						prom.Catch(func(ctx context.Context, val any, err error) Result[any] {
							return nil
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.Res()
					}
				}
			})
		}
	})

	b.Run("with-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := getSuccessBenchmarkPromise()
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					if bc.callFollow {
						prom.Catch(func(ctx context.Context, val any, err error) Result[any] {
							return Val[any]("golang")
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.Res()
					}
				}
			})
		}
	})
}

func BenchmarkPromise_Then_Parallel(b *testing.B) {
	b.Run("no-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			b.Run(bc.name, func(b *testing.B) {
				prom := getSuccessBenchmarkPromise()

				if bc.stressed {
					b.SetParallelism(parallelism)
				}

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if bc.callFollow {
							prom.Then(func(ctx context.Context, val any) Result[any] {
								return nil
							})
						}
						if bc.callWait {
							prom.Wait()
						}
						if bc.callRes {
							prom.Res()
						}
					}
				})
			})
		}
	})

	b.Run("with-res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			b.Run(bc.name, func(b *testing.B) {
				prom := getSuccessBenchmarkPromise()

				if bc.stressed {
					b.SetParallelism(parallelism)
				}

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if bc.callFollow {
							prom.Then(func(ctx context.Context, val any) Result[any] {
								return Val[any]("golang")
							})
						}
						if bc.callWait {
							prom.Wait()
						}
						if bc.callRes {
							prom.Res()
						}
					}
				})
			})
		}
	})
}

// create a fulfilled promise, chain 1 callback, and callWait on the final promise
func BenchmarkPromise_Chain_Short(b *testing.B) {
	setNoPanicsPipelineCore()

	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			res = p.Res()
		}

		_ = res
	})

	b.Run("no-res_res-call-err", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var err error
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			res = p.Res()
			err = res.Err()
			if err != nil {
				b.Fatalf("unexpected error: %v, from res: %v", err, res)
			}
		}

		_ = res
		_ = err
	})

	b.Run("no-res_res-call-val", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var val any
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			res = p.Res()
			val = res.Val()
			if val != nil {
				b.Fatalf("unexpected val: %v, from res: %v", val, res)
			}
		}

		_ = res
		_ = val
	})

	b.Run("with-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		valOrigPtr := newPtrError()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any](valOrigPtr)
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		valOrigPtr := newPtrError()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any](valOrigPtr)
			})
			res = p.Res()
		}

		_ = res
	})

	b.Run("with-res_res-call-err", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var err error
		valOrigPtr := newPtrError()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any](valOrigPtr)
			})
			res = p.Res()
			err = res.Err()
			if err != nil {
				b.Fatalf("unexpected error: %v, from res: %v", err, res)
			}
		}

		_ = res
		_ = err
	})

	b.Run("with-res_res-call-val", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var val any
		valOrigPtr := newPtrError()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any](valOrigPtr)
			})
			res = p.Res()
			val = res.Val()
			if val != valOrigPtr {
				b.Fatalf("unexpected val: %v, from res: %v", val, res)
			}
		}

		_ = res
		_ = val
	})

	b.Run("panic_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				panic("test panic")
			})
			p.Wait()
		}
	})

	b.Run("panic_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				panic("test panic")
			})
			res = p.Res()
		}

		_ = res
	})

	b.Run("panic_res-call-err", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var err error
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				panic("test panic")
			})
			res = p.Res()
			err = res.Err()
			upErr := err.(UncaughtPanic)
			if upErr.v != "test panic" {
				b.Fatalf("unexpected error: %v, from: %v", upErr, err)
			}
		}

		_ = res
		_ = err
	})

	b.Run("panic_res-call-val", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		var val any
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				panic("test panic")
			})
			res = p.Res()
			val = res.Val()
			if val != nil {
				b.Fatalf("unexpected val: %v, from res: %v", val, res)
			}
		}

		_ = res
		_ = val
	})
}

// create a fulfilled promise, chain 3 callbacks, and callWait on the final promise
func BenchmarkPromise_Chain_Medium(b *testing.B) {
	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			res = p.Res()
		}

		_ = res
	})

	b.Run("with-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			})
			res = p.Res()
		}

		_ = res
	})
}

// create a fulfilled promise, chain 5 callbacks, and callWait on the final promise
func BenchmarkPromise_Chain_Long(b *testing.B) {
	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			}).Then(func(ctx context.Context, val any) Result[any] {
				return nil
			})
			res = p.Res()
		}

		_ = res
	})

	b.Run("with-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		var res Result[any]
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := prom.Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			}).Then(func(ctx context.Context, val any) Result[any] {
				return Val[any]("golang")
			})
			res = p.Res()
		}

		_ = res
	})
}
