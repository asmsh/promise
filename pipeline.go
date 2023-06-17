package promise

import (
	"context"
	"time"
)

type PipelineConfig struct {
	UncaughtPanicHandler func(v any)
	UncaughtErrHandler   func(err error)

	// Size is the allowed number of goroutines which this pipeline can run.
	// This includes goroutines created for both, constructor calls(Go, GoRes, etc.)
	// and follow calls(Then, Catch, etc.).
	// If it's 0 or less, then the pipeline size is unlimited.
	Size int
}

type Pipeline[T any] struct {
	config *PipelineConfig

	reserveChan chan struct{}
}

func NewPipeline[T any](c PipelineConfig) Pipeline[T] {
	var reserveChan chan struct{}
	if c.Size > 0 {
		reserveChan = make(chan struct{}, c.Size)
	}
	return Pipeline[T]{
		config:      &c,
		reserveChan: reserveChan,
	}
}

func (pp *Pipeline[T]) Go(ctx context.Context, fun func()) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pp.reserveGoroutine()
	p := newPromInter[T](pp)
	go runCallback[T](ctx, p, goCallback[T](fun), false, nil, 0, true)
	return p
}

func (pp *Pipeline[T]) GoErr(ctx context.Context, fun func() error) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pp.reserveGoroutine()
	p := newPromInter[T](pp)
	go runCallback[T](ctx, p, goErrCallback[T](fun), true, nil, 0, true)
	return p
}

func (pp *Pipeline[T]) GoRes(ctx context.Context, fun func(ctx context.Context) Result[T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pp.reserveGoroutine()
	p := newPromInter[T](pp)
	go runCallback[T](ctx, p, goResCallback[T](fun), true, nil, 0, true)
	return p
}

func (pp *Pipeline[T]) New(resChan chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	prom := newPromExter(pp, resChan)
	return prom
}

func (pp *Pipeline[T]) Resolver(resolverCb func(
	fulfill func(val ...T),
	reject func(err error, val ...T),
)) Promise[T] {
	if resolverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	pp.reserveGoroutine()
	p := newPromInter[T](pp)
	go resolverCall(p, resolverCb)
	return p
}

func resolverCall[T any](
	p *GenericPromise[T],
	cb func(fulfill func(...T), reject func(error, ...T)),
) {
	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	// defer the return handler to handle panics and runtime.Goexit calls
	defer p.handleReturns(nil)

	// create the resolver functions and pass them to the callback
	fulfill := func(val ...T) {
		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call(from fulfill or reject) will reach this point

		if len(val) == 0 {
			p.resolveToFulfilledRes(Empty[T](), false)
		} else {
			p.resolveToFulfilledRes(Val(val[0]), false)
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
			p.resolveToRejectedRes(Err[T](err), false)
		} else {
			p.resolveToRejectedRes(ValErr(val[0], err), false)
		}
	}

	cb(fulfill, reject)
}

func (pp *Pipeline[T]) Delay(
	res Result[T],
	d time.Duration,
	onSuccess bool,
	onFailure bool,
) Promise[T] {
	pp.reserveGoroutine()
	p := newPromInter[T](pp)
	go delayCall(p, res, d, onSuccess, onFailure)
	return p
}

// handles rejection and fulfillment only
func delayCall[T any](
	p *GenericPromise[T],
	res Result[T],
	d time.Duration,
	onSuccess bool,
	onFailure bool,
) {
	// make sure we free this goroutine reservation
	defer p.pipeline.freeGoroutine()

	if res == nil {
		if onFailure {
			time.Sleep(d)
		}
		p.resolveToRejectedRes(Err[T](ErrPromiseNilResult), false)
	} else if err := res.Err(); err != nil {
		if onFailure {
			time.Sleep(d)
		}
		p.resolveToRejectedRes(res, false)
	} else {
		if onSuccess {
			time.Sleep(d)
		}
		p.resolveToFulfilledRes(res, false)
	}
}

func (pp *Pipeline[T]) Wrap(res Result[T]) Promise[T] {
	p := newPromSync[T](pp)
	p.resolveToResSync(res)
	return p
}

func (pp *Pipeline[T]) Panic(v any) Promise[T] {
	p := newPromSync[T](pp)
	p.panicSync(Err[T](newUncaughtPanic(v)))
	return p
}

func (pp *Pipeline[T]) reserveGoroutine() {
	if pp.reserveChan != nil {
		pp.reserveChan <- struct{}{}
	}
}

func (pp *Pipeline[T]) freeGoroutine() {
	if pp.reserveChan != nil {
		<-pp.reserveChan
	}
}
