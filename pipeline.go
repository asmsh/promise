package promise

import (
	"context"
	"time"
)

type PipelineConfig struct {
	UncaughtPanicHandler func(v UncaughtPanic)
	UncaughtErrHandler   func(v UncaughtError)

	// Size is the allowed number of goroutines which this pipeline can run.
	// This includes goroutines created for both, constructor calls(Go, GoRes, etc.)
	// and follow calls(Then, Catch, etc.).
	// If it's 0 or less, then the pipeline size is unlimited.
	Size int
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
		if cb := c[0].UncaughtErrHandler; cb != nil {
			pp.core.uncaughtErrHandler = cb
		}
		if size := c[0].Size; size > 0 {
			pp.core.reserveChan = make(chan struct{}, size)
		}
	}

	return pp
}

func (pp *Pipeline[T]) Chan(resChan chan Result[T]) Promise[T] {
	return chanCall[T](&pp.core, resChan)
}

// TODO: panic if the resChan's capacity is 0
func chanCall[T any](pc *pipelineCore, resChan chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}
	return newPromExter(pc, resChan)
}

func (pp *Pipeline[T]) Go(fun func()) Promise[T] {
	return goCall[T](&pp.core, fun)
}

func goCall[T any](pc *pipelineCore, fun func()) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go runCallback[T, T](p, goCallback[T, T](fun), nil, false, true)
	return p
}

func (pp *Pipeline[T]) GoErr(fun func() error) Promise[T] {
	return goErrCall[T](&pp.core, fun)
}

func goErrCall[T any](pc *pipelineCore, fun func() error) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go runCallback[T, T](p, goErrCallback[T, T](fun), nil, true, true)
	return p
}

func (pp *Pipeline[T]) GoRes(fun func(ctx context.Context) Result[T]) Promise[T] {
	return goResCall[T](&pp.core, fun)
}

func goResCall[T any](
	pc *pipelineCore,
	fun func(ctx context.Context) Result[T],
) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go runCallback[T, T](p, goResCallback[T, T](fun), nil, true, true)
	return p
}

func (pp *Pipeline[T]) Resolver(
	ctx context.Context,
	fun func(
		ctx context.Context,
		fulfill func(val ...T),
		reject func(err error, val ...T),
	),
) Promise[T] {
	return resolverCall[T](&pp.core, fun)
}

func resolverCall[T any](
	pc *pipelineCore,
	resolverCb func(ctx context.Context, fulfill func(val ...T), reject func(err error, val ...T)),
) Promise[T] {
	if resolverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	pc.reserveGoroutine()
	p := newPromInter[T](pc)
	go resolverHandler(p, resolverCb)
	return p
}

func resolverHandler[T any](
	p *genericPromise[T],
	cb func(ctx context.Context, fulfill func(...T), reject func(error, ...T)),
) {
	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// defer the return handler to handle panics and runtime.Goexit calls,
	// or returns that didn't call neither fulfill nor reject.
	defer handleReturns(p, nil)

	// create the resolver functions and pass them to the callback
	fulfill := func(val ...T) {
		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call (from fulfill or reject) will reach this point

		if len(val) == 0 {
			resolveToFulfilledRes[T](p, nil)
		} else {
			resolveToFulfilledRes[T](p, Val(val[0]))
		}
	}

	reject := func(err error, val ...T) {
		if err == nil {
			fulfill(val...)
			return
		}

		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call(from fulfill or reject) will reach this point

		if len(val) == 0 {
			resolveToRejectedRes[T](p, Err[T](err))
		} else {
			resolveToRejectedRes[T](p, ValErr(val[0], err))
		}
	}

	cb(context.TODO(), fulfill, reject)
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

	if res != nil && res.Err() != nil {
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToRejectedRes(p, res)
	} else {
		if flags.onSuccess {
			time.Sleep(dd)
		}
		resolveToFulfilledRes(p, res)
	}
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
	p.panicSync(errPromisePanickedResult[T]{v: v})
	return p
}

type pipelineCore struct {
	uncaughtPanicHandler func(v UncaughtPanic)
	uncaughtErrHandler   func(v UncaughtError)

	reserveChan chan struct{}
}

func (pc *pipelineCore) reserveGoroutine() {
	if pc != nil {
		if pc.reserveChan != nil {
			pc.reserveChan <- struct{}{}
		}
	}
}

func (pc *pipelineCore) freeGoroutine() {
	if pc != nil {
		if pc.reserveChan != nil {
			<-pc.reserveChan
		}
	}
}
