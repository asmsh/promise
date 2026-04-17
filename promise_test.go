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
	"runtime"
	"sync/atomic"
	"testing"
)

var testEnableLogs = false

// testStrError is an error implementation that's used only for testing.
// it's a string to allow comparing its values.
type testStrError string

func (t testStrError) Error() string { return string(t) }

func newTestStrError(msg ...string) error {
	if len(msg) == 0 {
		return testStrError("str_test_error")
	}
	return testStrError(msg[0])
}

// testPtrError is an error implementation that's used only for testing.
// it's a pointer-based error, to mimic most error structures in real-scenarios.
type testPtrError struct {
	txt string
}

func (t *testPtrError) Error() string { return t.txt }

func newTestPtrError() error {
	return &testPtrError{txt: "ptr_test_error"}
}

func TestPanicking(t *testing.T) {
	wantV := "test_panic"

	t.Run("Callback handling", func(t *testing.T) {
		p := Go(func() {
			panic(wantV)
		}).Recover(func(ctx context.Context, res Result[any]) Result[any] {
			if perr := new(PanicError); !errors.As(res.Err(), perr) || perr.V != wantV {
				t.Fatalf("Res() got unexpected error: %v", res.Err())
			}
			return nil
		})
		p.Wait()
	})

	t.Run("Res handling", func(t *testing.T) {
		p := Go(func() {
			panic(wantV)
		})
		res := p.Res()
		if res == nil {
			t.Fatalf("Res() = %v, want: non-nil", res)
		}
		if perr := new(PanicError); !errors.As(res.Err(), perr) || perr.V != wantV {
			t.Fatalf("Res() got unexpected error: %v", res.Err())
		}
	})
}

// TestPanicNil verifies that panic(nil) produces a [Panic] state, not an [Error]
// state with [ErrPromiseGoexit].  Under Go 1.21+ panic(nil) is wrapped in a
// *runtime.PanicNilError, making recover() return a non-nil value and keeping
// the distinction from runtime.Goexit intact.
func TestPanicNil(t *testing.T) {
	res := Go(func() {
		panic(nil)
	}).Res()

	if res == nil {
		t.Fatal("Res() = nil, want non-nil")
	}
	if got := res.State(); got != Panic {
		t.Errorf("State() = %v, want %v (panic(nil) should not be treated as Goexit)", got, Panic)
	}
	if errors.Is(res.Err(), ErrPromiseGoexit) {
		t.Error("Err() wraps ErrPromiseGoexit, but panic(nil) should produce Panic state")
	}
	if !errors.Is(res.Err(), ErrPromisePanicked) {
		t.Errorf("Err() = %v, want errors.Is(_, ErrPromisePanicked)", res.Err())
	}
}

// TestGoexit verifies that calling runtime.Goexit inside any callback causes
// the resulting Promise to resolve to an [Error] state wrapping [ErrPromiseGoexit],
// and that it is correctly distinguished from both a normal return and a panic.
func TestGoexit(t *testing.T) {
	// checkRes is a helper that asserts the Goexit result contract.
	checkRes := func(t *testing.T, res Result[any]) {
		t.Helper()
		if res == nil {
			t.Fatal("Res() = nil, want non-nil")
		}
		if got := res.State(); got != Error {
			t.Errorf("State() = %v, want %v", got, Error)
		}
		if !errors.Is(res.Err(), ErrPromiseGoexit) {
			t.Errorf("Err() = %v, want errors.Is(_, ErrPromiseGoexit)", res.Err())
		}
		if res.Val() != nil {
			t.Errorf("Val() = %v, want nil", res.Val())
		}
	}

	// --- constructor callbacks ---

	t.Run("Go", func(t *testing.T) {
		res := Go(func() {
			runtime.Goexit()
		}).Res()
		checkRes(t, res)
	})

	// --- follow callbacks ---

	t.Run("Follow", func(t *testing.T) {
		res := Go(func() {}).
			Follow(func(ctx context.Context, res Result[any]) Result[any] {
				runtime.Goexit()
				return nil
			}).
			Res()
		checkRes(t, res)
	})

	// Ensure that Goexit on a non-target state is a pass-through:
	// Catch's Goexit should not fire when the predecessor is Success.
	t.Run("Catch pass-through on Success", func(t *testing.T) {
		res := GoCtxRes(func(ctx context.Context) Result[any] {
			return ValRes[any]("ok")
		}).Catch(func(ctx context.Context, res Result[any]) Result[any] {
			// This must never be called; the promise should carry through.
			runtime.Goexit()
			return nil
		}).Res()
		if res == nil {
			t.Fatal("Res() = nil, want non-nil")
		}
		if got := res.State(); got != Success {
			t.Errorf("State() = %v, want %v (Catch should be skipped on Success)", got, Success)
		}
	})

	// Same pass-through check for Recover on a non-Panic predecessor.
	t.Run("Recover pass-through on Error", func(t *testing.T) {
		res := GoErr(func() error {
			return newTestStrError()
		}).Recover(func(ctx context.Context, res Result[any]) Result[any] {
			runtime.Goexit()
			return nil
		}).Res()
		if res == nil {
			t.Fatal("Res() = nil, want non-nil")
		}
		if got := res.State(); got != Error {
			t.Errorf("State() = %v, want %v (Recover should be skipped on Error)", got, Error)
		}
		if errors.Is(res.Err(), ErrPromiseGoexit) {
			t.Error("Err() wraps ErrPromiseGoexit, but Recover callback should not have been called")
		}
	})
}

func TestRejection(t *testing.T) {
	wantErr := newTestStrError()

	t.Run("Callback handling", func(t *testing.T) {
		p := GoCtxRes(func(ctx context.Context) Result[any] {
			return ErrRes[any](wantErr)
		}).Catch(func(ctx context.Context, res Result[any]) Result[any] {
			if !errors.Is(res.Err(), wantErr) {
				t.Errorf("Res() got unexpected error: %v", res.Err())
			}
			return nil
		})
		p.Wait()
	})

	t.Run("Res handling", func(t *testing.T) {
		p := GoCtxRes(func(ctx context.Context) Result[any] {
			return ErrRes[any](wantErr)
		})
		res := p.Res()
		if res == nil {
			t.Errorf("Res() = nil, want: non-nil")
		}
		if !errors.Is(res.Err(), wantErr) {
			t.Errorf("Res() got unexpected error: %v", res.Err())
		}
	})
}

func TestFinally(t *testing.T) {
	t.Run("Success callback on Success", func(t *testing.T) {
		called := atomic.Bool{}
		res := GoCtxRes(func(ctx context.Context) Result[any] {
			return ValRes[any]("val")
		}).Finally(func(ctx context.Context) {
			called.Store(true)
			return
		}).Res()

		if !called.Load() {
			t.Error("Finally wasn't called")
		}
		if res.State() != Success {
			t.Errorf("res.State() = %v, want: %v", res.State(), Success)
		}
		if res.Err() != nil {
			t.Errorf("res.Err() = %v, want: nil", res.Err())
		}
		if res.Val() != nil {
			t.Errorf("res.Val() = %v, want: nil", res.Val())
		}
	})

	t.Run("Panic callback on Success", func(t *testing.T) {
		wantV := "test_panic"
		called := atomic.Bool{}
		res := GoCtxRes(func(ctx context.Context) Result[any] {
			return ValRes[any]("val")
		}).Finally(func(ctx context.Context) {
			called.Store(true)
			panic(wantV)
		}).Res()

		if !called.Load() {
			t.Error("Finally wasn't called")
		}
		if res.State() != Panic {
			t.Errorf("res.State() = %v, want: %v", res.State(), Success)
		}
		if perr := new(PanicError); !errors.As(res.Err(), perr) || perr.V != wantV {
			t.Fatalf("res.Err() got unexpected error: %v", res.Err())
		}
		if res.Val() != nil {
			t.Errorf("res.Val() = %v, want: nil", res.Val())
		}
	})

	t.Run("Success callback on Error", func(t *testing.T) {
		called := atomic.Bool{}
		res := GoCtxRes(func(ctx context.Context) Result[any] {
			return ErrRes[any](newTestStrError())
		}).Finally(func(ctx context.Context) {
			called.Store(true)
			return
		}).Res()

		if !called.Load() {
			t.Error("Finally wasn't called")
		}
		if res.State() != Success {
			t.Errorf("res.State() = %v, want: %v", res.State(), Success)
		}
		if res.Err() != nil {
			t.Errorf("res.Err() = %v, want: nil", res.Err())
		}
		if res.Val() != nil {
			t.Errorf("res.Val() = %v, want: nil", res.Val())
		}
	})

	t.Run("Success callback on Panic", func(t *testing.T) {
		called := atomic.Bool{}
		res := GoCtxRes(func(ctx context.Context) Result[any] {
			return PanicRes[any]("test_panic")
		}).Finally(func(ctx context.Context) {
			called.Store(true)
			return
		}).Res()

		if !called.Load() {
			t.Error("Finally wasn't called")
		}
		if res.State() != Success {
			t.Errorf("res.State() = %v, want: %v", res.State(), Success)
		}
		if res.Err() != nil {
			t.Errorf("res.Err() = %v, want: nil", res.Err())
		}
		if res.Val() != nil {
			t.Errorf("res.Val() = %v, want: nil", res.Val())
		}
	})
}
