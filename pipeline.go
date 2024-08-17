package promise

import (
	"context"
	"sync"
	"time"
)

type PipelineConfig struct {
	UncaughtPanicHandler func(v any)
	UncaughtErrorHandler func(v error)

	// Size is the allowed number of goroutines which this pipeline can run.
	// This includes goroutines created for both, constructor calls(Go, GoRes, etc.)
	// and follow calls(Then, Catch, etc.).
	// If it's 0 or less, then the pipeline size is unlimited.
	Size int

	// CancelAllCtxOnFailure, if true, will result in canceling all Context values
	// passed to all callbacks, once any callback returns an error or cause a panic
	// that's not caught or recovered, through Catch or Recover, respectively.
	CancelAllCtxOnFailure bool

	// UseEmptyCallbackCtx, if true, will result in passing the empty Context value,
	// context.Background, to all callback functions.
	// If true, it will result in not canceling the Context value when the promise is
	// resolved.
	// If CancelAllCtxOnFailure is true, this will be set to false.
	UseEmptyCallbackCtx bool
}

type Pipeline[T any] struct {
	core pipelineCore
}

func NewPipeline[T any](c ...*PipelineConfig) *Pipeline[T] {
	pp := &Pipeline[T]{}

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

		if c[0].UseEmptyCallbackCtx && !c[0].CancelAllCtxOnFailure {
			pp.core.useEmptyCallbackCtx = true
		}
	}

	return pp
}

func (pp *Pipeline[T]) Chan(resChan chan Result[T]) Promise[T] {
	return chanCall[T](&pp.core, resChan)
}

func chanCall[T any](pc *pipelineCore, resChan chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go chanHandler(p, resChan)
	return p
}

func chanHandler[T any](p *genericPromise[T], resChan chan Result[T]) {
	res := <-resChan
	resolveToRes(p, res)
}

func (pp *Pipeline[T]) Ctx(ctx context.Context) Promise[T] {
	return ctxCall[T](&pp.core, ctx)
}

func ctxCall[T any](pc *pipelineCore, ctx context.Context) Promise[T] {
	if ctx.Done() == nil {
		// since this ctx value will never be closed, the equivalent outcome would
		// be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded resources.
		return newPromBlocking[T]()
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go ctxHandler(p, ctx)
	return p
}

func ctxHandler[T any](p *genericPromise[T], ctx context.Context) {
	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// wait until the Context is closed
	<-ctx.Done()

	// resolve to the equivalent result
	resolveToRes[T](p, ctxResult[T]{ctx: ctx})
}

func (pp *Pipeline[T]) Go(fun func()) Promise[T] {
	return goCall[T](&pp.core, fun)
}

func goCall[T any](pc *pipelineCore, fun goCallback[T, T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	ctx, cancel := pc.callbackCtx()
	go runCallback[T, T](p, fun, nil, false, true, true, ctx, cancel)
	return p
}

func (pp *Pipeline[T]) GoErr(fun func() error) Promise[T] {
	return goErrCall[T](&pp.core, fun)
}

func goErrCall[T any](pc *pipelineCore, fun goErrCallback[T, T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	ctx, cancel := pc.callbackCtx()
	go runCallback[T, T](p, fun, nil, true, true, true, ctx, cancel)
	return p
}

func (pp *Pipeline[T]) GoRes(fun func(ctx context.Context) Result[T]) Promise[T] {
	return goResCall[T](&pp.core, fun)
}

func goResCall[T any](
	pc *pipelineCore,
	fun goResCallback[T, T],
) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	ctx, cancel := pc.callbackCtx()
	go runCallback[T, T](p, fun, nil, true, true, true, ctx, cancel)
	return p
}

func (pp *Pipeline[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	return delayCall[T](&pp.core, res, d, cond...)
}

func delayCall[T any](
	pc *pipelineCore,
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) Promise[T] {
	flags := getDelayFlags(cond)
	pc.reserveGoroutine()
	p := newPromInter[T](pc)
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
	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	resolveToResWithDelay(p, res, dd, flags)
}

func (pp *Pipeline[T]) Wrap(res Result[T]) Promise[T] {
	return wrapCall[T](&pp.core, res)
}

func wrapCall[T any](pc *pipelineCore, res Result[T]) Promise[T] {
	p := newPromSync[T](pc)
	p.resolveToResSync(res)
	return p
}

func (pp *Pipeline[T]) Panic(v any) Promise[T] {
	return panicCall[T](&pp.core, v)
}

func panicCall[T any](pc *pipelineCore, v any) Promise[T] {
	p := newPromSync[T](pc)
	p.panicSync(promisePanickedResult[T]{v: v})
	return p
}

func (pp *Pipeline[T]) Wait() {
	pp.core.wg.Wait()
}

type pipelineCore struct {
	uncaughtPanicHandler func(v any)
	uncaughtErrorHandler func(v error)

	wg          sync.WaitGroup
	reserveChan chan struct{}

	useEmptyCallbackCtx bool

	// ctx will be non-nil if the Pipeline is meant to close all Context values
	// once any Promise that's created using it is rejected or panicked.
	ctx    context.Context
	cancel context.CancelFunc
}

func (pc *pipelineCore) reserveGoroutine() {
	if pc == nil {
		return
	}
	// add to the wait group before waiting, to make sure that this goroutine
	// reservation is accounted for.
	pc.wg.Add(1)
	if pc.reserveChan != nil {
		pc.reserveChan <- struct{}{}
	}
}

func (pc *pipelineCore) freeGoroutine() {
	if pc == nil {
		return
	}
	pc.wg.Done()
	if pc.reserveChan != nil {
		<-pc.reserveChan
	}
}

func (pc *pipelineCore) ctxParent() context.Context {
	if pc == nil || pc.ctx == nil {
		return context.Background()
	}
	return pc.ctx
}

func noop() {}

func (pc *pipelineCore) callbackCtx() (context.Context, context.CancelFunc) {
	if pc == nil || pc.useEmptyCallbackCtx {
		return context.Background(), noop
	}
	return context.WithCancel(pc.ctxParent())
}
