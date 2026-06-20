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
	"context"
	"errors"
	"fmt"
	"runtime"
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
		res := Go(func() { panic(panicV) }).WaitRes()
		helper(t, panicV, res)
	})

	t.Run("async-2", func(t *testing.T) {
		panicV := "test panic"
		g := NewGroup[any]()
		res := g.Go(func() { panic(panicV) }).WaitRes()
		helper(t, panicV, res)
	})

	t.Run("sync-1", func(t *testing.T) {
		panicV := newTestStrError()
		res := Wrap(PanicRes[any](panicV)).WaitRes()
		helper(t, panicV, res)
	})

	t.Run("sync-2", func(t *testing.T) {
		panicV := "test panic"
		g := NewGroup[any]()
		res := g.Wrap(PanicRes[any](panicV)).WaitRes()
		helper(t, panicV, res)
	})

	t.Run("custom-result-1", func(t *testing.T) {
		// in this case, the error value in origRes implements resultPanicV
		// (through PanicError), so the panicV should be the wrapped value
		// from PanicError.
		panicV := "test panic"
		origRes := newResult[any](nil, PanicError{panicV}, Panic)
		res := Wrap(origRes).WaitRes()
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
		res := g.Wrap(origRes).WaitRes()
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
		res := g.Wrap(origRes).WaitRes()
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
		res := g.Wrap(origRes).WaitRes()
		helper(t, panicV, res)
	})
}

func TestErrPromiseConsumed(t *testing.T) {
	helper := func(t *testing.T, wantState State, wantV any, p *Promise[any]) {
		// first result should reflect the wanted state and value.
		res1 := p.WaitRes()
		if want, got := wantState, res1.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := wantV, res1.Val(); want != got {
			t.Errorf("Val: want %q, got %q", want, got)
		}

		// second result and onward should be Error of ErrPromiseConsumed.
		res2 := p.WaitRes()
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
		res3 := p.WaitRes()
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

func TestErrPromiseGoexit(t *testing.T) {
	helper := func(t *testing.T, res Result[any]) {
		t.Helper()
		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %v, got %v", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrPromiseGoexit) {
			t.Errorf("Err: should be ErrPromiseGoexit, got %v", err)
		}
	}

	t.Run("async-1", func(t *testing.T) {
		res := Go(func() { runtime.Goexit() }).WaitRes()
		helper(t, res)
	})

	t.Run("async-2", func(t *testing.T) {
		g := NewGroup[any]()
		res := g.Go(func() { runtime.Goexit() }).WaitRes()
		helper(t, res)
	})
}

func TestErrNilCtxDone(t *testing.T) {
	helper := func(t *testing.T, res Result[any]) {
		t.Helper()
		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %v, got %v", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrNilCtxDone) {
			t.Errorf("Err: should be ErrNilCtxDone, got %v", err)
		}
	}

	// context.Background() has a nil Done channel.
	t.Run("ctx-1", func(t *testing.T) {
		g := NewGroup[any](NoNilCtxDoneChan())
		res := g.Ctx(context.Background()).WaitRes()
		helper(t, res)
	})

	// context.TODO() also has a nil Done channel.
	t.Run("ctx-2", func(t *testing.T) {
		g := NewGroup[any](NoNilCtxDoneChan())
		res := g.Ctx(context.TODO()).WaitRes()
		helper(t, res)
	})
}

func TestErrGroupBusy(t *testing.T) {
	helper := func(t *testing.T, res Result[any]) {
		t.Helper()
		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %v, got %v", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrGroupBusy) {
			t.Errorf("Err: should be ErrGroupBusy, got %v", err)
		}
	}

	// Size(1) + NoWaitingBusyGroup: the first call fills the single slot, the
	// second returns immediately with ErrGroupBusy without blocking.
	t.Run("busy-1", func(t *testing.T) {
		release := make(chan struct{})
		g := NewGroup[any](Size(1), NoWaitingBusyGroup())
		g.Go(func() { <-release }) // reserve the only slot
		res := g.Go(func() {}).WaitRes()
		close(release)
		g.Wait()
		helper(t, res)
	})
}

func TestErrGroupWaiting(t *testing.T) {
	helper := func(t *testing.T, res Result[any]) {
		t.Helper()
		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %v, got %v", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrGroupWaiting) {
			t.Errorf("Err: should be ErrGroupWaiting, got %v", err)
		}
	}

	// After Wait(), the group is in waiting mode and rejects new work.
	t.Run("wait-1", func(t *testing.T) {
		g := NewGroup[any]()
		g.Wait()
		res := g.Go(func() {}).WaitRes()
		helper(t, res)
	})

	// AllWaitRes also puts the group in waiting mode.
	t.Run("wait-2", func(t *testing.T) {
		g := NewGroup[any]()
		g.AllWaitRes()
		res := g.Go(func() {}).WaitRes()
		helper(t, res)
	})
}

func TestErrGroupCanceled(t *testing.T) {
	helper := func(t *testing.T, res Result[any]) {
		t.Helper()
		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		if want, got := any(nil), res.Val(); want != got {
			t.Errorf("Val: want %v, got %v", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrGroupCanceled) {
			t.Errorf("Err: should be ErrGroupCanceled, got %v", err)
		}
	}

	// An unhandled Error result cancels the group; subsequent calls get ErrGroupCanceled.
	t.Run("canceled-1", func(t *testing.T) {
		g := NewGroup[any](FailuresCancelGroup())
		g.GoErr(func() error { return newTestStrError() }).Wait()
		res := g.Go(func() {}).WaitRes()
		helper(t, res)
	})

	// An unhandled Panic result also cancels the group.
	t.Run("canceled-2", func(t *testing.T) {
		g := NewGroup[any](FailuresCancelGroup())
		g.Go(func() { panic("boom") }).Wait()
		res := g.Go(func() {}).WaitRes()
		helper(t, res)
	})
}

func TestIdxError(t *testing.T) {
	// A single failing promise produces an Error result whose Err() wraps an
	// IdxError with the correct Idx and the original error as its Err field.
	t.Run("error", func(t *testing.T) {
		wantErr := newTestStrError()
		res := All(Wrap(ErrRes[any](wantErr))).WaitRes()

		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("errors.Is: should match the wrapped error")
		}

		var idxErr IdxError
		if !errors.As(err, &idxErr) {
			t.Fatalf("errors.As: should extract IdxError")
		}
		if want, got := 0, idxErr.Idx; want != got {
			t.Errorf("IdxError.Idx: want %d, got %d", want, got)
		}
		if !errors.Is(idxErr.Err, wantErr) {
			t.Errorf("IdxError.Err: want %v, got %v", wantErr, idxErr.Err)
		}
	})

	// AllWait processes all promises; the Idx on the extracted IdxError must
	// reflect the original position in the slice, not the arrival order.
	t.Run("error-at-index", func(t *testing.T) {
		wantErr := newTestStrError()
		res := AllWait(
			Wrap[any](nil),             // index 0 – success
			Wrap(ErrRes[any](wantErr)), // index 1 – error
		).WaitRes()

		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}

		var idxErr IdxError
		if !errors.As(res.Err(), &idxErr) {
			t.Fatalf("errors.As: should extract IdxError")
		}
		if want, got := 1, idxErr.Idx; want != got {
			t.Errorf("IdxError.Idx: want %d, got %d", want, got)
		}
		if !errors.Is(idxErr.Err, wantErr) {
			t.Errorf("IdxError.Err: want %v, got %v", wantErr, idxErr.Err)
		}
	})

	// A panicking promise produces a Panic result; errors.As extracts both the
	// IdxError wrapper and the inner PanicError.
	t.Run("panic", func(t *testing.T) {
		panicV := "test panic"
		res := All(Go(func() { panic(panicV) })).WaitRes()

		if want, got := Panic, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrPromisePanicked) {
			t.Errorf("errors.Is: should match ErrPromisePanicked")
		}

		var idxErr IdxError
		if !errors.As(err, &idxErr) {
			t.Fatalf("errors.As: should extract IdxError")
		}
		if want, got := 0, idxErr.Idx; want != got {
			t.Errorf("IdxError.Idx: want %d, got %d", want, got)
		}

		var panicErr PanicError
		if !errors.As(err, &panicErr) {
			t.Fatalf("errors.As: should extract PanicError through IdxError chain")
		}
		if panicErr.V != panicV {
			t.Errorf("PanicError.V: want %q, got %v", panicV, panicErr.V)
		}
	})
}

func TestGroupError(t *testing.T) {
	// A failing promise in a group produces an Error result whose Err() wraps a
	// GroupError carrying the original error.
	t.Run("error", func(t *testing.T) {
		wantErr := newTestStrError()
		g := NewGroup[any]()
		res := g.AllWaitRes(func() {
			g.GoErr(func() error { return wantErr })
		})

		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("errors.Is: should match the wrapped error")
		}

		var groupErr GroupError
		if !errors.As(err, &groupErr) {
			t.Fatalf("errors.As: should extract GroupError")
		}
		if !errors.Is(groupErr.Err, wantErr) {
			t.Errorf("GroupError.Err: want %v, got %v", wantErr, groupErr.Err)
		}
	})

	// A panicking promise in a group produces a Panic result; errors.As extracts
	// both the GroupError wrapper and the inner PanicError.
	t.Run("panic", func(t *testing.T) {
		panicV := "test panic"
		g := NewGroup[any]()
		res := g.AllWaitRes(func() {
			g.Go(func() { panic(panicV) })
		})

		if want, got := Panic, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}
		if !errors.Is(err, ErrPromisePanicked) {
			t.Errorf("errors.Is: should match ErrPromisePanicked")
		}

		var groupErr GroupError
		if !errors.As(err, &groupErr) {
			t.Fatalf("errors.As: should extract GroupError")
		}

		var panicErr PanicError
		if !errors.As(err, &panicErr) {
			t.Fatalf("errors.As: should extract PanicError through GroupError chain")
		}
		if panicErr.V != panicV {
			t.Errorf("PanicError.V: want %q, got %v", panicV, panicErr.V)
		}
	})
}

func TestMultiError(t *testing.T) {
	// With a single error, Error() returns just that error's string.
	t.Run("single-error-string", func(t *testing.T) {
		e := newTestStrError("err1")
		merr := MultiError{Errs: []error{e}}
		if want, got := e.Error(), merr.Error(); want != got {
			t.Errorf("Error(): want %q, got %q", want, got)
		}
	})

	// With multiple errors, Error() returns them newline-separated.
	t.Run("multi-error-string", func(t *testing.T) {
		e1 := newTestStrError("err1")
		e2 := newTestStrError("err2")
		merr := MultiError{Errs: []error{e1, e2}}
		want := e1.Error() + "\n" + e2.Error()
		if got := merr.Error(); want != got {
			t.Errorf("Error(): want %q, got %q", want, got)
		}
	})

	// Unwrap returns the exact slice stored in Errs.
	t.Run("unwrap", func(t *testing.T) {
		e1 := newTestStrError("err1")
		e2 := newTestStrError("err2")
		merr := MultiError{Errs: []error{e1, e2}}
		errs := merr.Unwrap()
		if want, got := 2, len(errs); want != got {
			t.Fatalf("Unwrap() len: want %d, got %d", want, got)
		}
		if errs[0] != e1 {
			t.Errorf("Unwrap()[0]: want %v, got %v", e1, errs[0])
		}
		if errs[1] != e2 {
			t.Errorf("Unwrap()[1]: want %v, got %v", e2, errs[1])
		}
	})

	// AllWait with two failing promises produces a result whose Err() can be
	// extracted as a MultiError with both original errors reachable via errors.Is.
	t.Run("errors.As from AllWait", func(t *testing.T) {
		e1 := newTestStrError("err1")
		e2 := newTestStrError("err2")
		res := AllWait(
			Wrap(ErrRes[any](e1)),
			Wrap(ErrRes[any](e2)),
		).WaitRes()

		if want, got := Error, res.State(); want != got {
			t.Errorf("State: want %q, got %q", want, got)
		}
		err := res.Err()
		if err == nil {
			t.Fatalf("Err: should not be nil")
		}

		var multiErr MultiError
		if !errors.As(err, &multiErr) {
			t.Fatalf("errors.As: should extract MultiError")
		}
		if want, got := 2, len(multiErr.Errs); want != got {
			t.Fatalf("MultiError.Errs length: want %d, got %d", want, got)
		}
		if !errors.Is(err, e1) {
			t.Errorf("errors.Is(err, e1): should be true")
		}
		if !errors.Is(err, e2) {
			t.Errorf("errors.Is(err, e2): should be true")
		}
	})
}
