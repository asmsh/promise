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
	"errors"
	"testing"
	"time"
)

func getErrorBenchmarkPromise() *Promise[any] {
	waitChan := make(chan struct{})
	prom := GoCtxRes(func(ctx context.Context) Result[any] {
		close(waitChan)
		time.Sleep(100 * time.Microsecond)
		return ErrRes[any](newTestStrError())
	})
	<-waitChan // make sure the promise is started before we return it.
	return prom
}

func getSuccessBenchmarkPromise(res ...any) *Promise[any] {
	var resVal any
	if len(res) > 0 {
		resVal = res[0]
	}

	waitChan := make(chan struct{})
	prom := GoCtxRes(func(ctx context.Context) Result[any] {
		close(waitChan)
		time.Sleep(100 * time.Microsecond)
		return ValRes(resVal)
	})
	<-waitChan // make sure the promise is started before we return it.
	return prom
}

func BenchmarkPromise_Wait(b *testing.B) {
	b.Run("resolved-sync", func(b *testing.B) {
		prom := Wrap[any](nil)

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			prom.Wait()
		}
	})

	b.Run("resolved-async", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			prom.Wait()
		}
	})

	b.Run("not-resolved-async", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			b.StopTimer()
			prom := getSuccessBenchmarkPromise()
			b.StartTimer()

			prom.Wait()
		}
	})
}

func BenchmarkPromise_WaitRes(b *testing.B) {
	b.Run("sync-success-resolved_nil-res", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](nil)

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("sync-success-resolved_non-nil-res", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](ValRes[any]("test test"))

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("sync-error-resolved", func(b *testing.B) {
		var res Result[any]
		prom := Wrap[any](ErrRes[any](newTestStrError()))

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-success-resolved_nil-res", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-success-not-resolved_nil-res", func(b *testing.B) {
		var res Result[any]

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			b.StopTimer()
			prom := getSuccessBenchmarkPromise()
			b.StartTimer()

			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-success-resolved_non-nil-res", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise(ValRes[any]("test test"))
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-success-not-resolved_non-nil-res", func(b *testing.B) {
		var res Result[any]

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			b.StopTimer()
			prom := getSuccessBenchmarkPromise(ValRes[any]("test test"))
			b.StartTimer()

			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-error-resolved", func(b *testing.B) {
		var res Result[any]
		prom := getErrorBenchmarkPromise()
		prom.Wait()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res = prom.WaitRes()
		}

		_ = res
	})

	b.Run("async-error-not-resolved", func(b *testing.B) {
		var res Result[any]

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			b.StopTimer()
			prom := getErrorBenchmarkPromise()
			b.StartTimer()

			res = prom.WaitRes()
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

const parallelism = 100_000

var promiseBenchs = []promiseBench{
	{stressed: false, callFollow: true, callWait: false, callRes: false, name: "normal_follow-only"},
	{stressed: false, callFollow: false, callWait: true, callRes: false, name: "normal_wait-only"},
	{stressed: false, callFollow: false, callWait: false, callRes: true, name: "normal_res-only"},
	{stressed: true, callFollow: true, callWait: true, callRes: false, name: "stressed_follow-only"},
	{stressed: true, callFollow: false, callWait: true, callRes: false, name: "stressed_wait-only"},
	{stressed: true, callFollow: false, callWait: false, callRes: true, name: "stressed_res-only"},
}

func BenchmarkPromise_Callback(b *testing.B) {
	for _, bc := range promiseBenchs {
		if bc.stressed {
			continue
		}

		b.Run(bc.name, func(b *testing.B) {
			prom := getSuccessBenchmarkPromise()
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				prom.Callback(func(ctx context.Context, res Result[any]) {

				})
				if bc.callWait {
					prom.Wait()
				}
				if bc.callRes {
					prom.WaitRes()
				}
			}
		})
	}
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
				for range b.N {
					if bc.callFollow {
						prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
							return nil
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.WaitRes()
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
				for range b.N {
					if bc.callFollow {
						prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
							return ValRes[any]("golang")
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.WaitRes()
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
				for range b.N {
					if bc.callFollow {
						prom.Catch(func(ctx context.Context, res Result[any]) Result[any] {
							return nil
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						res := prom.WaitRes()
						_ = res
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
				for range b.N {
					if bc.callFollow {
						prom.Catch(func(ctx context.Context, res Result[any]) Result[any] {
							return ValRes[any]("golang")
						})
					}
					if bc.callWait {
						prom.Wait()
					}
					if bc.callRes {
						prom.WaitRes()
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
							prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
								return nil
							})
						}
						if bc.callWait {
							prom.Wait()
						}
						if bc.callRes {
							prom.WaitRes()
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
							prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
								return ValRes[any]("golang")
							})
						}
						if bc.callWait {
							prom.Wait()
						}
						if bc.callRes {
							prom.WaitRes()
						}
					}
				})
			})
		}
	})
}

// create a Success promise, chain 1 callback, and call Wait and/or Res on the final promise.
func BenchmarkPromise_Chain_Short(b *testing.B) {
	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			res = p.WaitRes()
		}

		_ = res
	})

	b.Run("no-res_res-call-err", func(b *testing.B) {
		var res Result[any]
		var err error
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			res = p.WaitRes()
			err = res.Err()
			if err != nil {
				b.Fatalf("unexpected error: %v, from res: %v", err, res)
			}
		}

		_ = res
		_ = err
	})

	b.Run("no-res_res-call-val", func(b *testing.B) {
		var res Result[any]
		var val any
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			res = p.WaitRes()
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
		valOrigPtr := newTestPtrError()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any](valOrigPtr)
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()
		valOrigPtr := newTestPtrError()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any](valOrigPtr)
			})
			res = p.WaitRes()
		}

		_ = res
	})

	b.Run("with-res_res-call-err", func(b *testing.B) {
		var res Result[any]
		var err error
		prom := getSuccessBenchmarkPromise()
		valOrigPtr := newTestPtrError()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any](valOrigPtr)
			})
			res = p.WaitRes()
			err = res.Err()
			if err != nil {
				b.Fatalf("unexpected error: %v, from res: %v", err, res)
			}
		}

		_ = res
		_ = err
	})

	b.Run("with-res_res-call-val", func(b *testing.B) {
		var res Result[any]
		var val any
		prom := getSuccessBenchmarkPromise()
		valOrigPtr := newTestPtrError()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any](valOrigPtr)
			})
			res = p.WaitRes()
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
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				panic("test panic")
			})
			p.Wait()
		}
	})

	b.Run("panic_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				panic("test panic")
			})
			res = p.WaitRes()
		}

		_ = res
	})

	b.Run("panic_res-call-err", func(b *testing.B) {
		var res Result[any]
		var err error
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				panic("test panic")
			})
			res = p.WaitRes()
			err = res.Err()

			if upErr := (PanicError{}); errors.As(err, &upErr) && upErr.V != "test panic" {
				b.Fatalf("unexpected error: %v, from: %v", upErr, err)
			}
		}

		_ = res
		_ = err
	})

	b.Run("panic_res-call-val", func(b *testing.B) {
		var res Result[any]
		var val any
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				panic("test panic")
			})
			res = p.WaitRes()
			val = res.Val()
			if val != nil {
				b.Fatalf("unexpected val: %v, from res: %v", val, res)
			}
		}

		_ = res
		_ = val
	})
}

// create a Success promise, chain 3 callbacks, and call Wait and/or Res on the final promise.
func BenchmarkPromise_Chain_Medium(b *testing.B) {
	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			res = p.WaitRes()
		}

		_ = res
	})

	b.Run("with-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			})
			res = p.WaitRes()
		}

		_ = res
	})
}

// create a Success promise, chain 5 callbacks, and call Wait and/or Res on the final promise.
func BenchmarkPromise_Chain_Long(b *testing.B) {
	b.Run("no-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("no-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return nil
			})
			res = p.WaitRes()
		}

		_ = res
	})

	b.Run("with-res_wait-call", func(b *testing.B) {
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			})
			p.Wait()
		}
	})

	b.Run("with-res_res-call", func(b *testing.B) {
		var res Result[any]
		prom := getSuccessBenchmarkPromise()

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			p := prom.Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			}).Then(func(ctx context.Context, res Result[any]) Result[any] {
				return ValRes[any]("golang")
			})
			res = p.WaitRes()
		}

		_ = res
	})
}
