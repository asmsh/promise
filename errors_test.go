// Copyright 2025 Ahmad Sameh(asmsh)
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
	"errors"
	"fmt"
	"testing"
)

func TestErrPromisePanicked(t *testing.T) {
	helper := func(t *testing.T, panicV any, res Result[any]) {
		if want, got := Panic, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %q, got %q", want, got)
		}

		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrPromisePanicked) {
			t.Errorf("Err: should be ErrPromisePanicked")
		}
		if perr := new(PanicError); !errors.As(err, perr) {
			t.Errorf("Err: should implement PanicError")
		} else if perr.V != panicV {
			t.Errorf("Err.V: want %q, got %q", panicV, perr.V)
		}
	}

	t.Run("async-1", func(t *testing.T) {
		panicV := newTestStrError()
		res := Go(func() { panic(panicV) }).Res()
		helper(t, panicV, res)
	})

	t.Run("async-2", func(t *testing.T) {
		panicV := "test panic"
		g := NewGroup[any]()
		res := g.Go(func() { panic(panicV) }).Res()
		helper(t, panicV, res)
	})

	t.Run("sync-1", func(t *testing.T) {
		panicV := newTestStrError()
		res := Wrap(PanicRes[any](panicV)).Res()
		helper(t, panicV, res)
	})

	t.Run("sync-2", func(t *testing.T) {
		panicV := "test panic"
		g := NewGroup[any]()
		res := g.Wrap(PanicRes[any](panicV)).Res()
		helper(t, panicV, res)
	})

	t.Run("custom-result-1", func(t *testing.T) {
		// in this case, the error value in origRes implements resultPanicV
		// (through PanicError), so the panicV should be the wrapped value
		// from PanicError.
		panicV := "test panic"
		origRes := newResult[any](nil, PanicError{panicV}, Panic)
		res := Wrap(origRes).Res()
		helper(t, panicV, res)
	})

	t.Run("custom-result-2", func(t *testing.T) {
		type customRes[T any] struct {
			Result[T]
		}

		// in this case, the origRes doesn't have an error value,
		// so the panicV should be the whole origRes value.
		g := NewGroup[any]()
		origRes := customRes[any]{
			Result: newResult[any]("test panic", nil, Panic),
		}
		res := g.Wrap(origRes).Res()
		helper(t, origRes, res)
	})

	t.Run("custom-result-3", func(t *testing.T) {
		type customRes[T any] struct {
			Result[T]
		}

		// in this case, the origRes have an error value that doesn't implement
		// resultPanicV, so the panicV should be the whole origRes value.
		g := NewGroup[any]()
		origRes := customRes[any]{
			Result: newResult[any]("test panic", fmt.Errorf("%w", PanicError{"test panic"}), Panic),
		}
		res := g.Wrap(origRes).Res()
		helper(t, origRes, res)
	})

	t.Run("custom-result-4", func(t *testing.T) {
		type customRes[T any] struct {
			Result[T]
		}

		// in this case, the origRes have an error value that implements
		// resultPanicV (through PanicError), so the panicV should be the
		// wrapped value from PanicError.
		panicV := "test panic"
		g := NewGroup[any]()
		origRes := customRes[any]{
			Result: newResult[any](nil, PanicError{panicV}, Panic),
		}
		res := g.Wrap(origRes).Res()
		helper(t, panicV, res)
	})
}

func TestErrPromiseConsumed(t *testing.T) {
	helper := func(t *testing.T, wantState State, wantV any, p *Promise[any]) {
		// first result should reflect the wanted state and value.
		res1 := p.Res()
		if want, got := wantState, res1.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := wantV, res1.Val(); want != got {
			t.Errorf("Val: want %q, got %q", want, got)
		}

		// second result and onward should be Error of ErrPromiseConsumed.
		res2 := p.Res()
		if want, got := Error, res2.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res2.Val(); want != got {
			t.Errorf("Val: want %q, got %q", want, got)
		}
		if err := res2.Err(); err == nil {
			t.Errorf("Err: should not be nil")
		} else {
			if !errors.Is(err, ErrPromiseConsumed) {
				t.Errorf("Err: should be ErrPromiseConsumed")
			}
		}

		// subsequent calls should return the same Result.
		res3 := p.Res()
		if res2 != res3 {
			t.Errorf("Res: want %q, got %q", res2, res3)
		}

		// first result value shouldn't be modified.
		if want, got := wantState, res1.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := wantV, res1.Val(); want != got {
			t.Errorf("Val: want %q, got %q", want, got)
		}
	}

	t.Run("async-1", func(t *testing.T) {
		v := "test value"
		g := NewGroup[any](OnetimeHandling())
		p := g.GoValErr(func() (any, error) { return v, nil })
		helper(t, Success, v, p)
	})

	t.Run("async-2", func(t *testing.T) {
		v := "test value"
		e := newTestStrError()
		g := NewGroup[any](OnetimeHandling())
		p := g.GoValErr(func() (any, error) { return v, e })
		helper(t, Error, v, p)
	})

	t.Run("sync-1", func(t *testing.T) {
		v := "test value"
		e := newTestStrError()
		g := NewGroup[any](OnetimeHandling())
		p := g.Wrap(ValErrRes[any](v, e))
		helper(t, Error, v, p)
	})

	t.Run("sync-2", func(t *testing.T) {
		g := NewGroup[any](OnetimeHandling())
		p := g.Wrap(PanicRes[any]("test panic"))
		helper(t, Panic, nil, p)
	})
}
