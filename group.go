package promise

import (
	"context"
	"sync"
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
}

type Group[T any] struct {
	core groupCore
}

func NewGroup[T any](c ...*GroupConfig) *Group[T] {
	pp := &Group[T]{}

	if len(c) != 0 && c[0] != nil {
		if cb := c[0].UncaughtPanicHandler; cb != nil {
			pp.core.uncaughtPanicHandler = cb
		}
		if cb := c[0].UncaughtErrorHandler; cb != nil {
			pp.core.uncaughtErrorHandler = cb
		}

		if size := c[0].Size; size > 0 {
			pp.core.reserveChan = make(chan struct{}, size)
		}

		if c[0].CancelAllCtxOnFailure {
			pp.core.ctx, pp.core.cancel = context.WithCancel(context.Background())
		}

		if c[0].NeverCancelCallbackCtx && !c[0].CancelAllCtxOnFailure {
			pp.core.neverCancelCallbackCtx = true
		}
	}

	return pp
}

func (g *Group[T]) Chan(resChan chan Result[T]) Promise[T] {
	return chanCall[T](&g.core, resChan)
}

func chanCall[T any](gc *groupCore, resChan chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	gc.reserveGoroutine()
	p := newPromInter[T](gc)
	go chanHandler(p, resChan)
	return p
}

func chanHandler[T any](p *genericPromise[T], resChan chan Result[T]) {
	defer p.group.freeGoroutine()
	res := <-resChan
	resolveToRes(p, res)
}

func (g *Group[T]) Ctx(ctx context.Context) Promise[T] {
	return ctxCall[T](&g.core, ctx)
}

func ctxCall[T any](gc *groupCore, ctx context.Context) Promise[T] {
	if ctx == nil || ctx.Done() == nil {
		// since this ctx value will never be closed, the equivalent outcome would
		// be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded resources.
		return newPromBlocking[T]()
	}

	return newPromCtx[T](gc, ctx)
}

func (g *Group[T]) Go(fun func()) Promise[T] {
	return goCall[T](&g.core, fun)
}

func goCall[T any](gc *groupCore, fun goCallback[T, T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	gc.reserveGoroutine()
	p := newPromInter[T](gc)
	ctx, cancel := gc.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, fun, nil, false, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) GoErr(fun func() error) Promise[T] {
	return goErrCall[T](&g.core, fun)
}

func goErrCall[T any](gc *groupCore, fun goErrCallback[T, T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	gc.reserveGoroutine()
	p := newPromInter[T](gc)
	ctx, cancel := gc.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, fun, nil, true, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) GoRes(fun func(ctx context.Context) Result[T]) Promise[T] {
	return goResCall[T](&g.core, fun)
}

func goResCall[T any](
	gc *groupCore,
	fun goResCallback[T, T],
) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	gc.reserveGoroutine()
	p := newPromInter[T](gc)
	ctx, cancel := gc.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, fun, nil, true, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	return delayCall[T](&g.core, res, d, cond...)
}

func delayCall[T any](
	gc *groupCore,
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	flags := getDelayFlags(cond)
	gc.reserveGoroutine()
	p := newPromInter[T](gc)
	go delayHandler(p, res, d, flags)
	return p
}

// handles rejection and fulfillment only
func delayHandler[T any](
	p *genericPromise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer p.group.freeGoroutine()
	resolveToResWithDelay(p, res, dd, flags)
}

func (g *Group[T]) Wrap(res Result[T]) Promise[T] {
	return wrapCall[T](&g.core, res)
}

func wrapCall[T any](gc *groupCore, res Result[T]) Promise[T] {
	p := newPromSync[T](gc)
	p.resolveToResSync(res)
	return p
}

func (g *Group[T]) Panic(v any) Promise[T] {
	return panicCall[T](&g.core, v)
}

func panicCall[T any](gc *groupCore, v any) Promise[T] {
	p := newPromSync[T](gc)
	p.panicSync(promisePanickedResult[T]{v: v})
	return p
}

func (g *Group[T]) Wait() {
	g.core.wg.Wait()
}

type groupCore struct {
	uncaughtPanicHandler func(v any)
	uncaughtErrorHandler func(v error)

	wg          sync.WaitGroup
	reserveChan chan struct{}

	neverCancelCallbackCtx bool

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it is rejected or panicked.
	ctx    context.Context
	cancel context.CancelFunc
}

func (gc *groupCore) reserveGoroutine() {
	if gc == nil {
		return
	}
	// add to the wait group before waiting, to make sure that this goroutine
	// reservation is accounted for.
	gc.wg.Add(1)
	if gc.reserveChan != nil {
		gc.reserveChan <- struct{}{}
	}
}

func (gc *groupCore) freeGoroutine() {
	if gc == nil {
		return
	}
	gc.wg.Done()
	if gc.reserveChan != nil {
		<-gc.reserveChan
	}
}

func noop() {}

// callbackCtx returns the effective Context for a callback, and its CancelFunc,
// if one is needed.
// syncCtx should be a non-closed Context.
func (gc *groupCore) callbackCtx(syncCtx context.Context) (context.Context, context.CancelFunc) {
	if gc == nil {
		if syncCtx != nil {
			return syncCtx, noop
		}
		return context.WithCancel(context.Background())
	}
	if gc.ctx == nil && gc.neverCancelCallbackCtx {
		return context.Background(), noop
	}
	return context.WithCancel(gc.ctx)
}
