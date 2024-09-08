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

	// OnetimeResultHandling is used to enforce that the Result value returned from
	// any callback is passed around only one-time and only to a single goroutine.
	// Any further attempt to use the Result value will return an erroneous Result
	// value with its Err method returning an ErrPromiseConsumed error.
	// The value might be passed around as an argument to another callback, or by
	// returning it from the Result.GetRes method.
	OnetimeResultHandling bool
}

type Group[T any] struct {
	core groupCore
}

func NewGroup[T any](c ...*GroupConfig) *Group[T] {
	g := &Group[T]{}

	if len(c) != 0 && c[0] != nil {
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
	}

	return g
}

func (g *Group[T]) Chan(resChan chan Result[T]) Promise[T] {
	return chanCall[T](g, resChan)
}

func chanCall[T any](g *Group[T], resChan <-chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	go chanHandler(p, resChan)
	return p
}

func chanHandler[T any](p *genericPromise[T], resChan <-chan Result[T]) {
	defer p.group.freeGoroutine()
	res := <-resChan
	resolveToRes(p, res)
}

func (g *Group[T]) Ctx(ctx context.Context) Promise[T] {
	return ctxCall[T](g, ctx)
}

func ctxCall[T any](g *Group[T], ctx context.Context) Promise[T] {
	if ctx == nil || ctx.Done() == nil {
		// since this ctx value will never be closed, the equivalent outcome would
		// be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded resources.
		return newPromBlocking[T]()
	}

	return newPromCtx[T](g, ctx)
}

func (g *Group[T]) Go(fun func()) Promise[T] {
	return goCall[T](g, fun)
}

func goCall[T any](g *Group[T], cb goCallback[T, T]) Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, cb, nil, false, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) GoErr(fun func() error) Promise[T] {
	return goErrCall[T](g, fun)
}

func goErrCall[T any](g *Group[T], cb goErrCallback[T, T]) Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, cb, nil, true, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) GoRes(fun func(ctx context.Context) Result[T]) Promise[T] {
	return goResCall[T](g, fun)
}

func goResCall[T any](
	g *Group[T],
	fun goResCallback[T, T],
) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go runCallback[T, T](p, fun, nil, true, true, true, ctx, cancel)
	return p
}

func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	return delayCall[T](g, res, d, cond...)
}

func delayCall[T any](
	g *Group[T],
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	flags := getDelayFlags(cond)
	g.reserveGoroutine()
	p := newPromInter[T](g)
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
	return wrapCall[T](g, res)
}

func wrapCall[T any](g *Group[T], res Result[T]) Promise[T] {
	p := newPromSync[T](g)
	p.resolveToResSync(res)
	return p
}

func (g *Group[T]) Panic(v any) Promise[T] {
	return panicCall[T](g, v)
}

func panicCall[T any](g *Group[T], v any) Promise[T] {
	p := newPromSync[T](g)
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
	onetimeResultHandling  bool

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it is rejected or panicked.
	ctx    context.Context
	cancel context.CancelFunc
}

func (g *Group[T]) reserveGoroutine() {
	if g == nil {
		return
	}
	// add to the wait group before waiting, to make sure that this goroutine
	// reservation is accounted for.
	g.core.wg.Add(1)
	if g.core.reserveChan != nil {
		g.core.reserveChan <- struct{}{}
	}
}

func (g *Group[T]) freeGoroutine() {
	if g == nil {
		return
	}
	g.core.wg.Done()
	if g.core.reserveChan != nil {
		<-g.core.reserveChan
	}
}

func noop() {}

// callbackCtx returns the effective Context for a callback, and its CancelFunc,
// if one is needed.
// syncCtx should be a non-closed Context.
func (g *Group[T]) callbackCtx(syncCtx context.Context) (context.Context, context.CancelFunc) {
	if g == nil {
		if syncCtx != nil {
			return syncCtx, noop
		}
		return context.WithCancel(context.Background())
	}
	if g.core.ctx == nil || g.core.neverCancelCallbackCtx {
		return context.Background(), noop
	}
	return context.WithCancel(g.core.ctx)
}
