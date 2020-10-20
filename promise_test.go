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

func TestGoPromise_Await_WithPanic(t *testing.T) {
	p := promise.Go(func() {
		panic("my panic msg")
	})
	p.Recover(func(v interface{}, ok bool) (res promise.Res) {
		return
	})
	if got := p.Wait(); got != false {
		t.Errorf("Wait() = %v, want: false", got)
	}
}

func TestGoPromise_WaitChan(t *testing.T) {

}

func TestGoPromise_WaitUntil(t *testing.T) {
	promise.Go(func() {

	})
}

func TestGoPromise_GetResUntil(t *testing.T) {

}

func TestImmutability(t *testing.T) {
	expectedVal := "initial value"
	newVal := "new value"

	t.Run("from the callback's parameter", func(t *testing.T) {
		p := promise.GoRes(func() (res promise.Res) {
			return promise.Res{expectedVal}
		})

		// change the returned value from inside a callback
		p.Then(func(prevRes promise.Res, ok bool) (res promise.Res) {
			// change the first element value
			prevRes[0] = newVal
			return
		}).Wait()

		// ensure that the returned result still the same as returned from creation
		res, _ := p.GetRes()
		if got := res[0]; got != expectedVal {
			t.Errorf("Wait()[0] = %v, want: %v", got, expectedVal)
		}

		// ensure that the passed result still the same as returned from creation
		p.Then(func(prevRes promise.Res, ok bool) (res promise.Res) {
			if got := prevRes[0]; got != expectedVal {
				t.Errorf("callbacks' prevRes[0] = %v, want: %v", got, expectedVal)
			}
			return
		})
	})

	t.Run("from the GetRes' result", func(t *testing.T) {
		p := promise.GoRes(func() (res promise.Res) {
			return promise.Res{expectedVal}
		})
		res, _ := p.GetRes()
		// change the first element value
		res[0] = newVal

		// ensure that the returned result still the same as returned from creation
		res2, _ := p.GetRes()
		if got := res2[0]; got != expectedVal {
			t.Errorf("Wait()[0] = %v, want: %v", got, expectedVal)
		}

		// ensure that the passed result still the same as returned from creation
		p.Then(func(prevRes promise.Res, ok bool) (res promise.Res) {
			// the first element must be the same as returned from creation
			if got := prevRes[0]; got != expectedVal {
				t.Errorf("callbacks' prevRes[0] = %v, want: %v", got, expectedVal)
			}
			return
		})
	})
}

func TestNew(t *testing.T) {
	t.Run("sending normal res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang"}
		resChan := make(chan promise.Res, 1)
		p := promise.New(resChan)
		go func() {
			resChan <- wantRes
		}()

		parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
	})

	t.Run("sending error res", func(t *testing.T) {
		wantRes := promise.Res{"go", newTestErr("golang")}
		resChan := make(chan promise.Res, 1)
		p := promise.New(resChan)
		go func() {
			resChan <- wantRes
		}()

		parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
	})

	t.Run("closing", func(t *testing.T) {
		resChan := make(chan promise.Res, 1)
		p := promise.New(resChan)
		go func() {
			close(resChan)
		}()

		parallelChainTestRunner(t, p, nil, nil, nil, true, false, false)
	})
}

func TestNew_Panic(t *testing.T) {
	defer func() {
		v := recover()
		if v == nil {
			t.Fatalf("New(): sent two values on resChan with no panic")
		}
		if v != "promise: Only one send should be done on the resChan" {
			t.Errorf("New(): panic with unexpected value")
		}
	}()

	wantRes := promise.Res{"go", "golang"}
	resChan := make(chan promise.Res, 1)
	p1 := promise.New(resChan)
	go func() {
		resChan <- wantRes[:1]
		resChan <- wantRes[1:]
	}()

	p1.Wait()
	p1.GetRes()
	t.Errorf("New().GetRes() should panic, but it didn't")
}

func TestGo(t *testing.T) {
	t.Run("normal res", func(t *testing.T) {
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Go(func() {})
			parallelChainTestRunner(t, p, nil, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Go(func() {})
			sequentialChainTestRunner(t, p, nil, nil, nil, true, false, false)
		})
	})

	t.Run("panic res", func(t *testing.T) {
		wantV := newTestErr("golang")
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Go(func() {
				panic(wantV)
			})
			parallelChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Go(func() {
				panic(wantV)
			})
			sequentialChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
	})
}

func TestGoRes(t *testing.T) {
	t.Run("normal res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang"}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				return wantRes
			})
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				return wantRes
			})
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})

	t.Run("error res", func(t *testing.T) {
		wantRes := promise.Res{"go", newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				//time.Sleep(time.Millisecond) // wait for the method to be registered
				return wantRes
			})
			parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				//time.Sleep(time.Millisecond) // wait for the method to be registered
				return wantRes
			})
			sequentialChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
	})

	t.Run("panic res", func(t *testing.T) {
		wantV := newTestErr("golang")
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				panic(wantV)
			})
			parallelChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.GoRes(func() (res promise.Res) {
				panic(wantV)
			})
			sequentialChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
	})
}

func TestWrap(t *testing.T) {
	t.Run("normal res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang"}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Wrap(wantRes)
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Wrap(wantRes)
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})

	t.Run("error res", func(t *testing.T) {
		wantRes := promise.Res{"go", newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Wrap(wantRes)
			parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Wrap(wantRes)
			sequentialChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
	})
}

func TestFulfill(t *testing.T) {
	t.Run("normal res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang"}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Fulfill(wantRes...)
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Fulfill(wantRes...)
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})

	t.Run("error res", func(t *testing.T) {
		wantRes := promise.Res{"go", newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Fulfill(wantRes...)
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Fulfill(wantRes...)
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})
}

func TestReject(t *testing.T) {
	t.Run("nil error with res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang", nil}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Reject(nil, "go", "golang")
			parallelChainTestRunner(t, p, wantRes, nil, nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Reject(nil, "go", "golang")
			sequentialChainTestRunner(t, p, wantRes, nil, nil, false, true, false)
		})
	})
	t.Run("non-nil error with res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang", newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Reject(newTestErr("golang"), "go", "golang")
			parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Reject(newTestErr("golang"), "go", "golang")
			sequentialChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
	})
	t.Run("non-nil error with no res", func(t *testing.T) {
		wantRes := promise.Res{newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Reject(newTestErr("golang"))
			parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Reject(newTestErr("golang"))
			sequentialChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
	})
}

func TestPanic(t *testing.T) {
	wantV := newTestErr("golang")
	t.Run("parallel chain", func(t *testing.T) {
		p := promise.Panic(wantV)
		parallelChainTestRunner(t, p, nil, nil, wantV, false, false, true)
	})
	t.Run("sequential chain", func(t *testing.T) {
		p := promise.Panic(wantV)
		sequentialChainTestRunner(t, p, nil, nil, wantV, false, false, true)
	})
}

func TestResolver(t *testing.T) {
	t.Run("normal res", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang"}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				fulfill(wantRes...)
				reject(nil, wantRes...) // this will be a no-op
			})
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				fulfill(wantRes...)
				reject(nil) // this will be a no-op
			})
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})

	t.Run("error res", func(t *testing.T) {
		wantRes := promise.Res{"go", newTestErr("golang")}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				reject(newTestErr("golang"), wantRes[0])
				fulfill(nil) // this will be a no-op
			})
			parallelChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				reject(newTestErr("golang"), wantRes[0])
				fulfill(nil) // this will be a no-op
			})
			sequentialChainTestRunner(t, p, wantRes, newTestErr("golang"), nil, false, true, false)
		})
	})

	t.Run("panic res", func(t *testing.T) {
		wantV := newTestErr("golang")
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				panic(wantV)
			})
			parallelChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				panic(wantV)
			})
			sequentialChainTestRunner(t, p, nil, nil, wantV, false, false, true)
		})
	})

	t.Run("reject with nil error", func(t *testing.T) {
		wantRes := promise.Res{"go", "golang", nil}
		t.Run("parallel chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				reject(nil, wantRes[:2]...)
				fulfill(nil) // this will be a no-op
			})
			parallelChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
		t.Run("sequential chain", func(t *testing.T) {
			p := promise.Resolver(func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{})) {
				reject(nil, wantRes[:2]...)
				fulfill(nil) // this will be a no-op
			})
			sequentialChainTestRunner(t, p, wantRes, nil, nil, true, false, false)
		})
	})
}

// testErr is an error implementation that's used only for testing.
// it's a string to allow comparing its values.
type testErr string

func (t testErr) Error() string {
	return string(t)
}

func newTestErr(msg string) error {
	return testErr(msg)
}

func equalRess(res1, res2 promise.Res) bool {
	n1 := len(res1)
	n2 := len(res2)
	if n1 != n2 {
		return false
	}

	for i := 0; i < n1; i++ {
		if res1[i] != res2[i] {
			return false
		}
	}
	return true
}

// parallelChainTestRunner tests general usage of methods: Then, Catch, Recover,
// Finally, Wait, and GetRes
// corner cases are tested through explicit test functions
func parallelChainTestRunner(t *testing.T, p promise.Promise,
	wantRes promise.Res, // for Then, Catch, and GetRes
	wantErr error, // for Catch
	wantV interface{}, // for Recover
	callThen,
	callCatch,
	callRecover bool,
) {
	thenCalls := 0
	catchCalls := 0
	recoverCalls := 0
	finallyCalls := 0

	thenP := p.Then(func(res promise.Res, ok bool) promise.Res {
		if !callThen {
			t.Fatalf("Parallel Then() called, unexpectedly")
		}

		if !equalRess(res, wantRes) {
			t.Errorf("Parallel Then().res = %v, want %v", res, wantRes)
		}
		if ok != true {
			t.Errorf("Parallel Then().ok = %v, want: %v", ok, true)
		}

		thenCalls += 1
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res { return nil }).
		Recover(func(v interface{}, ok bool) promise.Res { return nil })

	catP := p.Catch(func(err error, res promise.Res, ok bool) promise.Res {
		if !callCatch {
			t.Fatalf("Parallel Catch() called, unexpectedly")
		}

		if err != wantErr {
			t.Errorf("Parallel Catch().err = %v, want %v", err, wantErr)
		}
		if !equalRess(res, wantRes) {
			t.Errorf("Parallel Catch().res = %v, want %v", res, wantRes)
		}
		if ok != true {
			t.Errorf("Parallel Catch().ok = %v, want: %v", ok, true)
		}

		catchCalls += 1
		return nil
	}).Recover(func(v interface{}, ok bool) promise.Res { return nil })

	recP := p.Recover(func(v interface{}, ok bool) promise.Res {
		if !callRecover {
			t.Fatalf("Parallel Recover() called, unexpectedly")
		}

		if v != wantV {
			t.Errorf("Parallel Recover().v = %v, want %v", v, wantV)
		}
		if ok != true {
			t.Errorf("Parallel Recover().ok = %v, want: %v", ok, true)
		}

		recoverCalls += 1
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res { return nil })

	wantOk := true
	if callRecover {
		wantOk = false
	}

	finalP := p.Finally(func(ok bool) promise.Res {
		if ok != wantOk {
			t.Errorf("Parallel Finally().ok = %v, want: %v", ok, wantOk)
		}

		finallyCalls += 1
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res { return nil }).
		Recover(func(v interface{}, ok bool) promise.Res { return nil })

	if gotOk := p.Wait(); gotOk != wantOk {
		t.Errorf("Parallel Wait() = %v, want: %v", gotOk, wantOk)
	}

	gotRes, gotOk := p.GetRes()
	if !equalRess(gotRes, wantRes) {
		t.Errorf("Parallel GetRes().res = %v, want %v", gotRes, wantRes)
	}
	if gotOk != wantOk {
		t.Errorf("Parallel GetRes().ok = %v, want: %v", gotOk, wantOk)
	}

	if gotOk := p.Wait(); gotOk != wantOk {
		t.Errorf("Parallel Wait() = %v, want: %v", gotOk, wantOk)
	}

	thenP.Wait()
	catP.Wait()
	recP.Wait()
	finalP.Wait()

	if callThen && thenCalls != 1 {
		t.Errorf("Parallel Then() did not called, unexpectedly")
	} else if !callThen && thenCalls != 0 {
		t.Errorf("Parallel Catch() called, unexpectedly")
	}
	if callCatch && catchCalls != 1 {
		t.Errorf("Parallel Catch() did not called, unexpectedly")
	} else if !callCatch && catchCalls != 0 {
		t.Errorf("Parallel Catch() called, unexpectedly")
	}
	if callRecover && recoverCalls != 1 {
		t.Errorf("Recover() did not called, unexpectedly")
	} else if !callRecover && recoverCalls != 0 {
		t.Errorf("Parallel Recover() called, unexpectedly")
	}
	if finallyCalls != 1 {
		t.Errorf("Parallel Finally() did not called, unexpectedly")
	}
}

func sequentialChainTestRunner(t *testing.T, p promise.Promise,
	wantRes promise.Res, // for Then, Catch, and GetRes
	wantErr error, // for Catch
	wantV interface{}, // for Recover
	callThen,
	callCatch,
	callRecover bool,
) {
	thenCalls := 0
	catchCalls := 0
	recoverCalls := 0
	finallyCalls := 0

	resP := p.Then(func(res promise.Res, ok bool) promise.Res {
		if !callThen {
			t.Fatalf("Sequential Then() called, unexpectedly")
		}

		if !equalRess(res, wantRes) {
			t.Errorf("Sequential Then().res = %v, want %v", res, wantRes)
		}
		if ok != true {
			t.Errorf("Sequential Then().ok = %v, want: %v", ok, true)
		}

		thenCalls += 1
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res {
		if !callCatch {
			t.Fatalf("Sequential Catch() called, unexpectedly")
		}

		if err != wantErr {
			t.Errorf("Sequential Catch().err = %v, want %v", err, wantErr)
		}
		if !equalRess(res, wantRes) {
			t.Errorf("Sequential Catch().res = %v, want %v", res, wantRes)
		}
		if ok != true {
			t.Errorf("Sequential Catch().ok = %v, want: %v", ok, true)
		}

		catchCalls += 1
		return nil
	}).Recover(func(v interface{}, ok bool) promise.Res {
		if !callRecover {
			t.Fatalf("Sequential Recover() called, unexpectedly")
		}

		if v != wantV {
			t.Errorf("Sequential Recover().v = %v, want %v", v, wantV)
		}
		if ok != true {
			t.Errorf("Sequential Recover().ok = %v, want: %v", ok, true)
		}

		recoverCalls += 1
		return nil
	}).Finally(func(ok bool) promise.Res {
		if !ok {
			t.Errorf("Sequential Finally().ok = %v, want: %v", ok, true)
		}

		finallyCalls += 1
		return nil
	})

	wantOk := true
	if callRecover {
		wantOk = false
	}

	if gotOk := p.Wait(); gotOk != wantOk {
		t.Errorf("Sequential Wait() = %v, want: %v", gotOk, wantOk)
	}

	gotRes, gotOk := p.GetRes()
	if !equalRess(gotRes, wantRes) {
		t.Errorf("Sequential GetRes().res = %v, want %v", gotRes, wantRes)
	}
	if gotOk != wantOk {
		t.Errorf("Sequential GetRes().ok = %v, want: %v", gotOk, wantOk)
	}

	if gotOk := p.Wait(); gotOk != wantOk {
		t.Errorf("Sequential Wait() = %v, want: %v", gotOk, wantOk)
	}

	resP.Wait()

	if callThen && thenCalls != 1 {
		t.Errorf("Sequential Then() did not called, unexpectedly")
	} else if !callThen && thenCalls != 0 {
		t.Errorf("Sequential Catch() called, unexpectedly")
	}
	if callCatch && catchCalls != 1 {
		t.Errorf("Sequential Catch() did not called, unexpectedly")
	} else if !callCatch && catchCalls != 0 {
		t.Errorf("Sequential Catch() called, unexpectedly")
	}
	if callRecover && recoverCalls != 1 {
		t.Errorf("Sequential Recover() did not called, unexpectedly")
	} else if !callRecover && recoverCalls != 0 {
		t.Errorf("Sequential Recover() called, unexpectedly")
	}
	if finallyCalls != 1 {
		t.Errorf("Sequential Finally() did not called, unexpectedly")
	}
}
