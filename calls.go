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

func ctxCall[T any](g *Group[T], ctx context.Context) Promise[T] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}
	if ctx.Done() == nil {
		// since this ctx value will never be closed, the equivalent outcome would
		// be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded resources.
		return newPromBlocking[T]()
	}

	return newPromCtx[T](g, ctx)
}

func goCall[T any](g *Group[T], cb goCallback[T, T]) Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go goHandler(p, cb, ctx, cancel)
	return p
}

func goHandler[T any](
	p *genericPromise[T],
	cb goCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, false, true, ctx, cancel)
}

func goErrCall[T any](g *Group[T], cb goErrCallback[T, T]) Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go goErrHandler(p, cb, ctx, cancel)
	return p
}

func goErrHandler[T any](
	p *genericPromise[T],
	cb goErrCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
}

func goResCall[T any](
	g *Group[T],
	cb goResCallback[T, T],
) Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	g.reserveGoroutine()
	p := newPromInter[T](g)
	ctx, cancel := g.callbackCtx(p.syncCtx)
	go goResHandler(p, cb, ctx, cancel)
	return p
}

func goResHandler[T any](
	p *genericPromise[T],
	cb goResCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer p.group.freeGoroutine()
	runCallbackHandler[T, T](p, cb, nil, true, true, ctx, cancel)
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

func wrapCall[T any](g *Group[T], res Result[T]) Promise[T] {
	p := newPromSync[T](g)
	p.resolveToResSync(res)
	return p
}

func panicCall[T any](g *Group[T], v any) Promise[T] {
	p := newPromSync[T](g)
	p.panicSync(promisePanickedResult[T]{v: v})
	return p
}
