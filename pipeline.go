package promise

import (
	"time"
)

type PipelineConfig struct {
	UncaughtPanicHandler func(v any)
	UncaughtErrHandler   func(err error)
}

type Pipeline[T any] struct {
	config *PipelineConfig
}

func NewPipeline[T any](c PipelineConfig) Pipeline[T] {
	return Pipeline[T]{
		config: &c,
	}
}

func (pp *Pipeline[T]) Go(fun func()) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	p := newPromInter[T]()
	go runCallback[T](p, goCallback[T](fun), false, nil, 0)
	return p
}

func (pp *Pipeline[T]) GoErr(fun func() error) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	p := newPromInter[T]()
	go runCallback[T](p, goErrCallback[T](fun), true, nil, 0)
	return p
}

func (pp *Pipeline[T]) GoRes(fun func() Result[T]) Promise[T] {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}

	p := newPromInter[T]()
	go runCallback[T](p, goResCallback[T](fun), true, nil, 0)
	return p
}

func (pp *Pipeline[T]) New(resChan chan Result[T]) Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	prom := newPromExter(resChan)
	return prom
}

func (pp *Pipeline[T]) Resolver(resolverCb func(
	fulfill func(val ...T),
	reject func(err error, val ...T),
)) Promise[T] {
	if resolverCb == nil {
		panic(nilCallbackPanicMsg)
	}

	p := newPromInter[T]()
	go resolverCall(p, resolverCb)
	return p
}

func resolverCall[T any](
	p *GenericPromise[T],
	cb func(fulfill func(...T), reject func(error, ...T)),
) {
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
		set, _ := p.status.SetResolving()
		if !set {
			return
		}

		// only one call(from fulfill or reject) will reach this point

		if err == nil {
			fulfill(val...)
			return
		}

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
	onSucceed, onFail bool,
) Promise[T] {
	p := newPromInter[T]()
	go delayCall(p, res, d, onSucceed, onFail)
	return p
}

// handles rejection and fulfillment only
func delayCall[T any](
	p *GenericPromise[T],
	res Result[T],
	d time.Duration,
	onSucceed bool,
	onFail bool,
) {
	if res == nil {
		if onFail {
			time.Sleep(d)
		}
		p.resolveToRejectedRes(Err[T](ErrPromiseNilResult), false)
	} else if err := res.Err(); err != nil {
		if onFail {
			time.Sleep(d)
		}
		p.resolveToRejectedRes(res, false)
	} else {
		if onSucceed {
			time.Sleep(d)
		}
		p.resolveToFulfilledRes(res, false)
	}
}

func (pp *Pipeline[T]) Wrap(res Result[T]) Promise[T] {
	p := newPromSync[T]()
	p.resolveToResSync(res)
	return p
}

func (pp *Pipeline[T]) Panic(v any) Promise[T] {
	p := newPromSync[T]()
	p.panicSync(Err[T](newUncaughtPanic(v)))
	return p
}
