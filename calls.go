// Copyright 2024 Ahmad Sameh(asmsh)
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

// calls.go: contains all the functions that implement other higher-level functions.
// they are extracted here since they are being reused.

package promise

import (
	"context"
	"time"
)

func noopRegFunc() {
	// do nothing
}

func chanCall[T any](g *Group[T], resChan <-chan Result[T]) *Promise[T] {
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

func ctxCall[T any](g *Group[T], ctx context.Context) *Promise[T] {
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

func goCall[T any](g *Group[T], cb goCallback[T, T]) *Promise[T] {
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

func goErrCall[T any](g *Group[T], cb goErrCallback[T, T]) *Promise[T] {
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

func goResCall[T any](
	g *Group[T],
	cb goResCallback[T, T],
) *Promise[T] {
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

func delayCall[T any](
	g *Group[T],
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

// handles rejection and fulfillment only
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

func wrapCall[T any](g *Group[T], res Result[T]) *Promise[T] {
	return newPromSync[T](g, res)
}
