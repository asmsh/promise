package promise

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type GroupConfig struct {
	UncaughtPanicHandler func(v any)
	UncaughtErrorHandler func(v error)

	// Size is the allowed number of goroutines which this group can run.
	// This includes goroutines created for both, constructor calls(Go, GoRes, etc.)
	// and follow calls(Then, Catch, etc.).
	// If it's 0 or less, then the group size is unlimited.
	Size int

	// CancelAllCtxOnFailure, if true, will result in canceling all Context values
	// passed to all callbacks, once any callback returns an error or cause a panic
	// that's not caught or recovered, through Catch or Recover, respectively.
	// The default behavior is never canceling the callbacks' [context.Context] value
	// on any failures.
	CancelAllCtxOnFailure bool

	// NeverCancelCallbackCtx, if true, will result in passing a never canceled
	// [context.Context] value to all callbacks.
	// If CancelAllCtxOnFailure is true, this will be set to false.
	// The default behavior is always canceling the callbacks' [context.Context] value.
	NeverCancelCallbackCtx bool

	// OnetimeResultHandling is used to enforce that the Result value returned from
	// any callback is passed around only one-time and only to a single goroutine.
	// Any further attempt to use the Result value will return an erroneous Result
	// value with its Err method returning an ErrPromiseConsumed error.
	// The value might be passed around as an argument to another callback, or by
	// returning it from the Result.GetRes method.
	OnetimeResultHandling bool

	// ErrorWhenCtxNilDone, if true, will cause new calls to [Group.Ctx] to return a
	// [Rejected] [Result] with [ErrPromiseNilCtxDone], when the [context.Context]
	// value passed has a nil Done channel ([context.Context.Done]).
	// Otherwise, it will allow the creation of the [Promise], which will be a never
	// resolved (blocked) promise, causing all follow promises to be blocked as well.
	ErrorWhenCtxNilDone bool

	// ErrorWhenGroupBusy, if true, will cause new calls to [Group.Chan], [Group.Delay],
	// and all [Group.Go], to return a [Rejected] [Result] with [ErrPromiseGroupBusy],
	// when the [Group] is currently handling promises that equals the [Size] value (if set).
	// Otherwise, it will wait until there's a place for the new promise.
	ErrorWhenGroupBusy bool
}

type Group[T any] struct {
	core groupCore
}

func NewGroup[T any](c ...GroupConfig) *Group[T] {
	g := &Group[T]{}

	if len(c) != 0 {
		if cb := c[0].UncaughtPanicHandler; cb != nil {
			g.core.uncaughtPanicHandler = cb
		}
		if cb := c[0].UncaughtErrorHandler; cb != nil {
			g.core.uncaughtErrorHandler = cb
		}

		if size := c[0].Size; size > 0 {
			g.core.reserveChan = make(chan struct{}, size)
		}

		if c[0].CancelAllCtxOnFailure {
			g.core.ctx, g.core.cancel = context.WithCancel(context.Background())
		}

		if c[0].NeverCancelCallbackCtx && !c[0].CancelAllCtxOnFailure {
			g.core.neverCancelCallbackCtx = true
		}

		if c[0].OnetimeResultHandling {
			g.core.onetimeResultHandling = true
		}

		if c[0].ErrorWhenCtxNilDone {
			g.core.errorWhenCtxNilDone = true
		}

		if c[0].ErrorWhenGroupBusy {
			g.core.errorWhenGroupBusy = true
		}
	}

	return g
}

func noopRegFunc() {
	// do nothing
}

func (g *Group[T]) Go(cb func()) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoHandler)
	go goHandler(p, cb, ctx, cancel)
	return p
}

func goHandler[T any](
	p *Promise[T],
	cb goCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, false, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoHandler)
}

func (g *Group[T]) GoErr(cb func() error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoErrHandler)
	go goErrHandler(p, cb, ctx, cancel)
	return p
}

func goErrHandler[T any](
	p *Promise[T],
	cb goErrCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoErrHandler)
}

func (g *Group[T]) GoRes(cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	debug(p, startHandler, startConstrHandler, startConstrGoResHandler)
	go goResHandler(p, cb, ctx, cancel)
	return p
}

func goResHandler[T any](
	p *Promise[T],
	cb goResCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
	debug(p, endHandler, endConstrHandler, endConstrGoResHandler)
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	flags := getDelayFlags(cond)
	debug(p, startHandler, startConstrHandler, startConstrDelayHandler)
	go delayHandler(p, res, d, flags)
	return p
}

func delayHandler[T any](
	p *Promise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer p.group.freeGoroutine()
	p.resolveToResWithDelay(res, dd, flags)
	debug(p, endHandler, endConstrHandler, endConstrDelayHandler)
}

func (g *Group[T]) Ctx(ctx context.Context) *Promise[T] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}
	if ctx.Done() != nil {
		return newPromCtx[T](g, ctx)
	} else if g != nil && g.core.errorWhenCtxNilDone {
		return newPromSync[T](g, errPromiseCtxNilDoneResult[T]{})
	}
	// since this ctx value will never be closed, the equivalent outcome would
	// be a Promise that's never resolved.
	// so, return that equivalent value without creating any unneeded resources.
	return newPromBlocked[T]()
}

func (g *Group[T]) Chan(resChan <-chan Result[T]) *Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}
	if g.isWaiting() {
		return newPromSync[T](g, errPromiseGroupDoneResult[T]{})
	}
	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync[T](g, errPromiseGroupBusyResult[T]{})
	}

	p := newPromInter[T](g)
	debug(p, startHandler, startConstrHandler, startConstrChanHandler)
	go chanHandler(p, resChan)
	return p
}

func chanHandler[T any](p *Promise[T], resChan <-chan Result[T]) {
	defer p.group.freeGoroutine()
	res := <-resChan
	p.resolveToRes(res)
	debug(p, endHandler, endConstrHandler, endConstrChanHandler)
}

func (g *Group[T]) Wrap(res Result[T]) *Promise[T] {
	return newPromSync[T](g, res)
}

func (g *Group[T]) Wait(triggers ...func()) {
	// execute the trigger functions to block the wait logic until they return.
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true) // blocks new calls from being started.
	g.core.wg.Wait()
}

type groupCore struct {
	debugCB func([]debugEvent)

	uncaughtPanicHandler func(v any)
	uncaughtErrorHandler func(v error)

	// wg is used for the basic WaitAll() method.
	wg sync.WaitGroup

	// waiting represents whether this group has entered the waiting
	// mode or not.
	// it can enter the waiting mode via one of the Wait methods.
	waiting atomic.Bool

	// reserveChan is used for limiting concurrency.
	reserveChan chan struct{}

	// flags for options...
	neverCancelCallbackCtx bool
	onetimeResultHandling  bool
	errorWhenCtxNilDone    bool
	errorWhenGroupBusy     bool

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it is rejected or panicked.
	// if neverCancelCallbackCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc
}

func (g *Group[T]) isWaiting() bool {
	if g == nil {
		return false
	}
	return g.core.waiting.Load()
}

func (g *Group[T]) reserveGoroutine(chainRegFunc func()) bool {
	if g == nil {
		// execute the register func and mark this reservation successful.
		chainRegFunc()
		return true
	}

	// make sure that this reservation is accounted for.
	g.core.wg.Add(1)

	// no size limitation is set, so return early.
	if g.core.reserveChan == nil {
		chainRegFunc()
		return true
	}

	// either block until a place is available, or return an error with no waiting.
	if g.core.errorWhenGroupBusy {
		select {
		case g.core.reserveChan <- struct{}{}:
			// since we entered this case, and this is a non-blocking select,
			// this case will happen immediately.
			chainRegFunc()
			return true
		default:
			return false
		}
	}

	// it's guaranteed to make a successful reservation in this flow (after waiting),
	// execute the register func before waiting.
	chainRegFunc()
	g.core.reserveChan <- struct{}{}
	return true
}

// note: if the program is exiting, this call might not be executed,
// so no important clean up or logic should be included here.
func (g *Group[T]) freeGoroutine() {
	if g == nil {
		return
	}
	g.core.wg.Done()
	if g.core.reserveChan != nil {
		<-g.core.reserveChan
	}
}

func noopCancelFunc() {
	// do nothing
}

// callbackCtx returns the effective Context for a callback, and its CancelFunc,
// if one is required, given the promise's syncCtx value.
// syncCtx should be a non-closed Context, or nil.
func (g *Group[T]) callbackCtx(syncCtx context.Context) (context.Context, context.CancelFunc) {
	// default scenario, either no Group or a Group with default behavior.
	// we return the syncCtx with no cancellation, if one is provided,
	// otherwise we return Background with cancellation.
	if g == nil || (g.core.ctx == nil && !g.core.neverCancelCallbackCtx) {
		if syncCtx == nil {
			return newSyncCtx(), nil
		}
		return syncCtx, noopCancelFunc
	}

	// there's a Group, if it's requested to never cancel callback Context,
	// then we return early with Background and no cancellation.
	if g.core.neverCancelCallbackCtx {
		return context.Background(), noopCancelFunc
	}

	// there's a Group with a group Context, so create the Context to be returned,
	// and arrange to close it when the promise's syncCtx is closed, if provided.
	if syncCtx == nil {
		return context.WithCancel(g.core.ctx)
	}

	// TODO: these 2 context calls can be replaced by a JoinContext that will be
	//  cancelled when any of them is cancelled.
	ctx, cancel := context.WithCancel(g.core.ctx)
	context.AfterFunc(syncCtx, cancel)
	return ctx, cancel
}
