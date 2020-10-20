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

package promise_test

import (
	"testing"

	"github.com/asmsh/promise"
)

// the benchmarks of the OncePromise is almost the same as GoPromise,
// so only the GoPromise benchmarks are implemented.

func BenchmarkNew(b *testing.B) {
	var pp *promise.GoPromise
	resChan := make(chan promise.Res, 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pp = promise.New(resChan)
	}
	pp = pp
}

func BenchmarkGo(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		promise.Go(func() {})
	}
}

func BenchmarkGoRes(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		promise.GoRes(func() promise.Res {
			return promise.Res{"go", "golang"}
		})
	}
}

type promiseBench struct {
	name string

	// stressed will call 'SetParallelism(100)', if true, otherwise it won't.
	// it's special for the parallel benchmarks only.
	stressed bool

	// wait and getRes chooses whether to call Wait, GetRes, or none
	wait   bool
	getRes bool
}

var promiseBenchs = []promiseBench{
	{name: "normal"},
	{name: "normal_wait", wait: true},
	{name: "normal_getRes", getRes: true},
	{name: "stressed", stressed: true},
	{name: "stressed_wait", stressed: true, wait: true},
	{name: "stressed_getRes", stressed: true, getRes: true},
}

func BenchmarkGoPromise_Then(b *testing.B) {
	b.Run("no res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := promise.GoRes(func() promise.Res {
					return nil
				})

				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					p := prom.Then(func(res promise.Res, ok bool) promise.Res {
						return nil
					})

					if bc.wait {
						p.Wait()
					}
					if bc.getRes {
						p.GetRes()
					}
				}
			})
		}
	})

	b.Run("with res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			if bc.stressed {
				continue
			}

			b.Run(bc.name, func(b *testing.B) {
				prom := promise.GoRes(func() promise.Res {
					return promise.Res{"go", "golang"}
				})

				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					p := prom.Then(func(res promise.Res, ok bool) promise.Res {
						return promise.Res{"go", "golang"}
					})

					if bc.wait {
						p.Wait()
					}
					if bc.getRes {
						p.GetRes()
					}
				}
			})
		}
	})
}

func BenchmarkGoPromise_Then_Parallel(b *testing.B) {
	b.Run("no res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			b.Run(bc.name, func(b *testing.B) {
				prom := promise.GoRes(func() promise.Res {
					return nil
				})

				if bc.stressed {
					b.SetParallelism(100)
				}

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						p := prom.Then(func(res promise.Res, ok bool) promise.Res {
							return nil
						})

						if bc.wait {
							p.Wait()
						}
						if bc.getRes {
							p.GetRes()
						}
					}
				})
			})
		}
	})

	b.Run("with res", func(b *testing.B) {
		for _, bc := range promiseBenchs {
			b.Run(bc.name, func(b *testing.B) {
				prom := promise.GoRes(func() promise.Res {
					return promise.Res{"go", "golang"}
				})

				if bc.stressed {
					b.SetParallelism(100)
				}

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						p := prom.Then(func(res promise.Res, ok bool) promise.Res {
							return promise.Res{"go", "golang"}
						})

						if bc.wait {
							p.Wait()
						}
						if bc.getRes {
							p.GetRes()
						}
					}
				})
			})
		}
	})
}

// create a fulfilled promise, chain 1 callback, and wait on the final promise
func BenchmarkGoPromise_Chain_Short(b *testing.B) {
	b.Run("no res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("with res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			})
			p.Wait()
		}
	})
}

// create a fulfilled promise, chain 3 callbacks, and wait on the final promise
func BenchmarkGoPromise_Chain_Medium(b *testing.B) {
	b.Run("no res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("with res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			})
			p.Wait()
		}
	})
}

// create a fulfilled promise, chain 5 callbacks, and wait on the final promise
func BenchmarkGoPromise_Chain_Long(b *testing.B) {
	b.Run("no res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return nil
			})
			p.Wait()
		}
	})

	b.Run("with res", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := promise.Fulfill().Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			}).Then(func(res promise.Res, ok bool) promise.Res {
				return promise.Res{"go", "golang"}
			})
			p.Wait()
		}
	})
}
