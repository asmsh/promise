// Copyright 2026 Ahmad Sameh(asmsh)
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
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asmsh/promise"
)

func TestSize(t *testing.T) {
	// Launches 6 callbacks into a Size(2) group. Each callback atomically
	// increments a counter on entry and decrements on exit. If the counter
	// ever exceeds 2 while a callback is running, the group allowed more
	// than the configured maximum to run at once.
	t.Run("limits concurrent goroutines", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.Size(2))

		var active atomic.Int32
		for i := 0; i < 6; i++ {
			g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
				n := active.Add(1)
				defer active.Add(-1)
				if n > 2 {
					t.Errorf("active goroutines = %d, want <= 2", n)
				}
				time.Sleep(5 * time.Millisecond)
				return nil
			})
		}
		g.Wait()
	})

	// Launches 3 callbacks into an unlimited group. Each one signals a
	// started channel then blocks on release. All 3 signals must arrive
	// before a short timeout, proving no goroutine was held back waiting
	// for a slot.
	t.Run("zero size allows unlimited concurrency", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.Size(0))

		started := make(chan struct{}, 3)
		release := make(chan struct{})
		for i := 0; i < 3; i++ {
			g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
				started <- struct{}{}
				<-release
				return nil
			})
		}
		timeout := time.After(100 * time.Millisecond)
		for i := 0; i < 3; i++ {
			select {
			case <-started:
			case <-timeout:
				t.Fatalf("goroutine %d didn't start (group may be unexpectedly size-limited)", i+1)
			}
		}
		close(release)
		g.Wait()
	})
}

func TestUnhandledPanicCB(t *testing.T) {
	// Registers a callback that records its argument and sets a flag. Runs a
	// callback that returns a PanicRes result without adding any follow calls
	// (.Wait() on the promise triggers the unhandled path). After the group
	// drains, checks the flag was set and the recorded value matches the panic
	// value passed to PanicRes.
	t.Run("with PanicRes result", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		var gotV any
		wantV := "boom"

		g := promise.NewGroup[any](promise.UnhandledPanicCB(func(v any) {
			gotV = v
			called.Store(true)
		}))

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.PanicRes[any](wantV)
		}).Wait()

		if !called.Load() {
			t.Fatal("UnhandledPanicCB was not called")
		}
		if gotV != wantV {
			t.Errorf("UnhandledPanicCB got %v, want %q", gotV, "boom")
		}
	})

	// Same setup but the callback calls panic() directly instead of returning
	// PanicRes. The group's goroutine wraps the recovered value into a panic
	// result internally. This exercises the runtime-panic recovery code path
	// rather than the explicit PanicRes path, verifying both routes fire the
	// UnhandledPanicCB.
	t.Run("with runtime panic in callback", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		var gotV any
		wantV := "boom"

		g := promise.NewGroup[any](promise.UnhandledPanicCB(func(v any) {
			gotV = v
			called.Store(true)
		}))

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			panic(wantV)
		}).Wait()

		if !called.Load() {
			t.Fatal("UnhandledPanicCB was not called")
		}
		if gotV != wantV {
			t.Errorf("UnhandledPanicCB got %v, want %q", gotV, "boom")
		}
	})
}

// Same structure as TestUnhandledPanicCB but with an Error result.
// The callback records the error passed to UnhandledErrorCB and
// the test verifies it wraps the original error via errors.Is.
func TestUnhandledErrorCB(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	var gotErr error
	wantErr := errors.New("boom")

	g := promise.NewGroup[any](promise.UnhandledErrorCB(func(err error) {
		gotErr = err
		called.Store(true)
	}))

	g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
		return promise.ErrRes[any](wantErr)
	}).Wait()

	if !called.Load() {
		t.Fatal("UnhandledErrorCB was not called")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("UnhandledErrorCB got %v, want %v", gotErr, wantErr)
	}
}

func TestOnetimeHandling(t *testing.T) {
	// Creates a group with OnetimeHandling, starts a promise, and calls
	// WaitRes() twice on the same promise. The first call returns the real
	// result; the second must return an Error result whose error satisfies
	// errors.Is(err, ErrPromiseConsumed).
	t.Run("with WaitRes", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.OnetimeHandling())

		wantVal := "v"
		p := g.GoValErr(func() (any, error) { return wantVal, nil })

		res1 := p.WaitRes()
		if want, got := promise.Success, res1.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if want, got := wantVal, res1.Val(); want != got {
			t.Errorf("Val = %v, want %v", got, want)
		}

		res2 := p.WaitRes()
		if want, got := promise.Error, res2.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res2.Err(), promise.ErrPromiseConsumed) {
			t.Errorf("Err = %v, want ErrPromiseConsumed", res2.Err())
		}
	})

	// Creates a group with OnetimeHandling and attaches a Then follow callback
	// that consumes the result. Once the callback completes, calling WaitRes()
	// on the original promise must return ErrPromiseConsumed, because Then
	// already marked the result as handled.
	t.Run("with a follow callback", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.OnetimeHandling())

		wantVal := "v"
		p := g.GoValErr(func() (any, error) { return wantVal, nil })
		p.Follow(func(ctx context.Context, res promise.Result[any]) promise.Result[any] {
			if res.State() != promise.Success {
				return res
			}
			if want, got := promise.Success, res.State(); want != got {
				t.Errorf("State = %v, want %v", got, want)
			}
			if want, got := wantVal, res.Val(); want != got {
				t.Errorf("Val = %v, want %v", got, want)
			}
			return nil
		}).Wait()

		res := p.WaitRes()
		if want, got := promise.Error, res.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res.Err(), promise.ErrPromiseConsumed) {
			t.Errorf("Err = %v, want ErrPromiseConsumed", res.Err())
		}
	})
}

func TestFailuresCancelCBCtx(t *testing.T) {
	// Starts two callbacks concurrently. The first blocks on ctx.Done() and
	// closes a canceled channel when it unblocks. The second returns an
	// unhandled Error result immediately. Waiting on the second promise ensures
	// unhandledError runs, which calls the group's cancel(), canceling all
	// derived callback contexts. The test then selects on canceled with a
	// timeout to confirm the first callback's context was canceled.
	t.Run("triggered by unhandled Error", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelCBCtx())

		canceled := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-ctx.Done()
			close(canceled)
			return nil
		})

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()

		select {
		case <-canceled:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("callback ctx wasn't canceled after an unhandled error")
		}

		g.Wait()
	})

	// Same setup but the failing callback returns a Panic result instead of an
	// Error. The unhandledPanic path also calls the group's cancel(), so the
	// first callback's ctx must be canceled too. This exercises the Panic code
	// path of FailuresCancelCBCtx separately from the Error path.
	t.Run("triggered by unhandled Panic", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelCBCtx())

		canceled := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-ctx.Done()
			close(canceled)
			return nil
		})

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.PanicRes[any]("boom")
		}).Wait()

		select {
		case <-canceled:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("callback ctx wasn't canceled after an unhandled panic")
		}

		g.Wait()
	})
}

func TestFailuresCancelGroup(t *testing.T) {
	// Runs one callback that returns an unhandled Error result and waits for it
	// to finish, ensuring the group's canceled flag is set. Then calls GoErr
	// again; because the group is now canceled, it returns a pre-resolved Error
	// promise immediately. The test checks the result's error satisfies
	// errors.Is(err, ErrGroupCanceled).
	t.Run("cancels the group after unhandled Error", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelGroup())

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()

		res := g.GoErr(func() error { return nil }).WaitRes()

		if want, got := promise.Error, res.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res.Err(), promise.ErrGroupCanceled) {
			t.Errorf("Err = %v, want ErrGroupCanceled", res.Err())
		}
	})

	// Same as above but the failing callback returns a Panic result. The
	// unhandledPanic path also sets the canceled flag, so new goroutines
	// must also get ErrGroupCanceled.
	t.Run("cancels the group after unhandled Panic", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelGroup())

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.PanicRes[any]("boom")
		}).Wait()

		res := g.GoErr(func() error { return nil }).WaitRes()

		if want, got := promise.Error, res.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res.Err(), promise.ErrGroupCanceled) {
			t.Errorf("Err = %v, want ErrGroupCanceled", res.Err())
		}
	})

	// FailuresCancelGroup also sets the group's internal context (just as
	// FailuresCancelCBCtx does), so a failure cancels all running callback
	// contexts too. The first callback blocks on ctx.Done(); after the second
	// returns an unhandled Error, the group ctx is canceled and the first
	// callback unblocks. This verifies FailuresCancelGroup implies
	// FailuresCancelCBCtx behavior.
	t.Run("also cancels callback ctxs on failure", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelGroup())

		canceled := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-ctx.Done()
			close(canceled)
			return nil
		})

		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()

		select {
		case <-canceled:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("callback ctx wasn't canceled after failure with FailuresCancelGroup")
		}

		g.Wait()
	})
}

func TestNeverCancelCBCtx(t *testing.T) {
	// Without NeverCancelCBCtx the default callback context is a syncCtx
	// derived from a cancelable parent, so ctx.Done() is always non-nil.
	// This establishes the baseline that the option actually changes behavior.
	t.Run("without the option, ctx is cancelable", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any]()

		var doneIsNil bool
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			doneIsNil = ctx.Done() == nil
			return nil
		}).Wait()

		if doneIsNil {
			t.Error("ctx.Done() is nil, want non-nil without NeverCancelCBCtx")
		}
	})

	// Captures ctx.Done() inside the callback. With NeverCancelCBCtx, the
	// context is context.Background(), whose Done() returns nil. The test
	// asserts ctx.Done() == nil.
	t.Run("ctx is never canceled", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.NeverCancelCBCtx())

		var doneIsNil bool
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			doneIsNil = ctx.Done() == nil
			return nil
		}).Wait()

		if !doneIsNil {
			t.Error("ctx.Done() is non-nil, want nil with NeverCancelCBCtx")
		}
	})

	// Applies FailuresCancelCBCtx() first, then NeverCancelCBCtx(). Because
	// FailuresCancelCBCtx is already set, NeverCancelCBCtx has no effect
	// (per the option's guard). The callback context is derived from the
	// group's context, so Done() is non-nil.
	t.Run("ignored if FailuresCancelCBCtx is set", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelCBCtx(), promise.NeverCancelCBCtx())

		var doneIsNil bool
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			doneIsNil = ctx.Done() == nil
			return nil
		}).Wait()

		if doneIsNil {
			t.Error("ctx.Done() is nil, want non-nil since FailuresCancelCBCtx takes precedence")
		}
	})

	// Same as above but with FailuresCancelGroup() instead.
	t.Run("ignored if FailuresCancelGroup is set", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.FailuresCancelGroup(), promise.NeverCancelCBCtx())

		var doneIsNil bool
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			doneIsNil = ctx.Done() == nil
			return nil
		}).Wait()

		if doneIsNil {
			t.Error("ctx.Done() is nil, want non-nil since FailuresCancelGroup takes precedence")
		}
	})
}

func TestNoNilCtxDoneChan(t *testing.T) {
	// Calls g.Ctx(context.Background()). Because Background().Done() is nil,
	// the promise never resolves. The test selects on p.WaitChan() with a
	// short timeout and expects the timeout to fire first.
	t.Run("without the option, the promise never resolves", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any]()

		p := g.Ctx(context.Background())
		select {
		case <-p.WaitChan():
			t.Fatal("expected the promise to never resolve")
		case <-time.After(20 * time.Millisecond):
		}
	})

	// Same call but with NoNilCtxDoneChan() set. The group detects the nil
	// Done channel and returns a pre-resolved Error promise with ErrNilCtxDone
	// instead of blocking forever.
	t.Run("with the option, returns ErrNilCtxDone", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.NoNilCtxDoneChan())

		res := g.Ctx(context.Background()).WaitRes()
		if want, got := promise.Error, res.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res.Err(), promise.ErrNilCtxDone) {
			t.Errorf("Err = %v, want ErrNilCtxDone", res.Err())
		}
	})
}

func TestNoWaitingBusyGroup(t *testing.T) {
	// Fills the single slot with a long-running callback. Starts a second Go
	// call in a goroutine. Selects on done with a timeout; the second call
	// must still be blocking (waiting for the slot), so the timeout branch
	// should be taken. Then releases the first callback and waits for the
	// second to finish.
	t.Run("without the option, new calls block until a slot is free", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.Size(1))

		release := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-release
			return nil
		})

		done := make(chan struct{})
		go func() {
			g.Go(func() {})
			close(done)
		}()

		select {
		case <-done:
			t.Fatal("second call returned before the group had a free slot")
		case <-time.After(20 * time.Millisecond):
		}

		close(release)
		<-done
	})

	// Same setup, but NoWaitingBusyGroup() is set. The second call returns
	// immediately with a pre-resolved Error promise. Verifies the result's
	// error satisfies errors.Is(err, ErrGroupBusy).
	t.Run("with the option, returns ErrGroupBusy", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.Size(1), promise.NoWaitingBusyGroup())

		release := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-release
			return nil
		})

		var p *promise.Promise[any]
		done := make(chan struct{})
		go func() {
			p = g.Go(func() {})
			close(done)
		}()

		var res promise.Result[any]
		select {
		case <-done:
			close(release)
			res = p.WaitRes()
		case <-time.After(1 * time.Millisecond):
			t.Fatal("second call didn't returned in time")
		}

		if want, got := promise.Error, res.State(); want != got {
			t.Errorf("State = %v, want %v", got, want)
		}
		if !errors.Is(res.Err(), promise.ErrGroupBusy) {
			t.Errorf("Err = %v, want ErrGroupBusy", res.Err())
		}
	})
}

// Starts 2 callbacks, then calls JoinRes() twice. The first call collects
// both results and saves them in the group's history. The second call reads
// from that history. Without SaveAllGroupResults the history holds only the
// first result, so the second call would return a slice of length 1. With
// the option, both results are saved, so len(res.Val()) must be 2.
func TestSaveAllGroupResults(t *testing.T) {
	t.Parallel()
	g := promise.NewGroup[any](promise.SaveAllGroupResults())

	g.GoCtxRes(func(ctx context.Context) promise.Result[any] { return promise.ValRes[any]("a") })
	g.GoCtxRes(func(ctx context.Context) promise.Result[any] { return promise.ValRes[any]("b") })

	g.JoinRes() // first call: collects both results into history
	res := g.JoinRes()

	if want, got := promise.Success, res.State(); want != got {
		t.Errorf("State = %v, want %v", got, want)
	}
	if got := len(res.Val()); got != 2 {
		t.Errorf("second JoinRes() returned %d results, want 2", got)
	}
}

// Contains one subtest per GroupConfig field, each verifying that the option
// takes effect when applied via ApplyConfig rather than the individual option
// function. The behavioral assertions are identical to the standalone tests
// above — ApplyConfig is validated end-to-end without inspecting any
// internal state.
func TestApplyConfig(t *testing.T) {
	t.Run("Size and NoWaitingBusyGroup", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{
			Size:               1,
			NoWaitingBusyGroup: true,
		}))

		release := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-release
			return nil
		})

		res := g.GoErr(func() error { return nil }).WaitRes()
		close(release)
		g.Wait()

		if !errors.Is(res.Err(), promise.ErrGroupBusy) {
			t.Errorf("Err = %v, want ErrGroupBusy", res.Err())
		}
	})

	t.Run("UnhandledPanicCB", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{
			UnhandledPanicCB: func(any) { called.Store(true) },
		}))
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.PanicRes[any]("boom")
		}).Wait()
		if !called.Load() {
			t.Fatal("UnhandledPanicCB was not called")
		}
	})

	t.Run("UnhandledErrorCB", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{
			UnhandledErrorCB: func(error) { called.Store(true) },
		}))
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()
		if !called.Load() {
			t.Fatal("UnhandledErrorCB was not called")
		}
	})

	t.Run("OnetimeHandling", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{OnetimeHandling: true}))
		p := g.GoValErr(func() (any, error) { return "v", nil })
		p.WaitRes()
		if !errors.Is(p.WaitRes().Err(), promise.ErrPromiseConsumed) {
			t.Error("expected ErrPromiseConsumed on second WaitRes()")
		}
	})

	t.Run("FailuresCancelCBCtx", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{FailuresCancelCBCtx: true}))

		canceled := make(chan struct{})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			<-ctx.Done()
			close(canceled)
			return nil
		})
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()

		select {
		case <-canceled:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("callback ctx wasn't canceled after an unhandled error")
		}
		g.Wait()
	})

	t.Run("FailuresCancelGroup", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{FailuresCancelGroup: true}))
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			return promise.ErrRes[any](errors.New("boom"))
		}).Wait()
		res := g.GoErr(func() error { return nil }).WaitRes()
		if !errors.Is(res.Err(), promise.ErrGroupCanceled) {
			t.Errorf("Err = %v, want ErrGroupCanceled", res.Err())
		}
	})

	t.Run("NeverCancelCBCtx", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{NeverCancelCBCtx: true}))
		var doneIsNil bool
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] {
			doneIsNil = ctx.Done() == nil
			return nil
		}).Wait()
		if !doneIsNil {
			t.Error("ctx.Done() is non-nil, want nil with NeverCancelCBCtx")
		}
	})

	t.Run("NoNilCtxDoneChan", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{NoNilCtxDoneChan: true}))
		res := g.Ctx(context.Background()).WaitRes()
		if !errors.Is(res.Err(), promise.ErrNilCtxDone) {
			t.Errorf("Err = %v, want ErrNilCtxDone", res.Err())
		}
	})

	t.Run("SaveAllGroupResults", func(t *testing.T) {
		t.Parallel()
		g := promise.NewGroup[any](promise.ApplyConfig(promise.GroupConfig{SaveAllGroupResults: true}))
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] { return promise.ValRes[any]("a") })
		g.GoCtxRes(func(ctx context.Context) promise.Result[any] { return promise.ValRes[any]("b") })
		g.JoinRes()
		res := g.JoinRes()
		if got := len(res.Val()); got != 2 {
			t.Errorf("second JoinRes() returned %d results, want 2", got)
		}
	})
}
