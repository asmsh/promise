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
	"sync/atomic"
	"testing"
)

// testStrError is an error implementation that's used only for testing.
// it's a string to allow comparing its values.
type testStrError string

func (t testStrError) Error() string {
	return string(t)
}

func newStrError() error {
	return testStrError("str_test_error")
}

// testPtrError is an error implementation that's used only for testing.
// it's a pointer-based error, to mimic most error structures in real-scenarios.
type testPtrError struct {
	txt string
}

func (t *testPtrError) Error() string {
	return t.txt
}

func newPtrError() error {
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

func TestRejection(t *testing.T) {
	wantErr := newStrError()

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
			return ErrRes[any](newStrError())
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
