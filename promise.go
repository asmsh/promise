// Copyright 2020 Ahmad Sameh(asmsh)
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

package promise

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Promise represents some asynchronous work (a goroutine).
//
// The zero value will block forever on any calls.
// It implements the [Result] interface
type Promise[T any] struct {
	// group is a pointer to the promise group which this promise is part of,
	// or nil, if it's not part of any group.
	group *Group[T]

	// initiated lazily, by the first extension call.
	// this is sent on by the different extension calls.
	// it's never closed.
	extChanP atomic.Pointer[chan extQueue[T]]

	// closed when this promise is resolved.
	// its channel has one writer (one goroutine), which is the owner,
	// which will close it, but can have multiple readers (chain promises).
	syncCtx context.Context

	// holds the result of the promise.
	// written once, before the syncCtx channel is closed.
	//
	// don't read it unless the syncCtx is known to be closed.
	res Result[T]

	// chainStatus holds the status of this promise's chain.
	// once there are any calls registered on this promise, or
	// the promise has been already handled, it will be non-zero.
	chainStatus atomic.Uint32
}

// extQueue wil be owned, at any time, by a single goroutine.
type extQueue[T any] struct {
	// initState is the state value at the time this callsQ was created.
	// note: the reason for this field is, during extension calls,
	// if the promise was resolved while creating this extQueue's chan,
	// the two select cases for the blocking scenario might be available
	// at the same time.
	// because the syncCtx.chan will be closed, and the extQ value will
	// be available only to this goroutine, at the same time.
	// so we need a way to still detect if the promise was resolved and
	// read its result sync, otherwise its result would be lost.
	initState State

	// whether the call value is valid or not.
	valid bool

	// call is the default extension call.
	call extCall[T]

	// extra holds any other extension calls, in addition to the one in call.
	extra []extCall[T]
}

// extCall describes an extension call and how to communicate back to it.
type extCall[T any] struct {
	// resChan is used to send the result back to the extension call's promise.
	// this is a new, per extension call, unbuffered channel.
	resChan chan<- IdxRes[T]

	// syncChan is the channel used to communicate that the extension call's
	// promise has been resolved, so that the sending promise can return.
	// this is typically extension call's promise's syncChan.
	syncChan <-chan struct{}

	// idx is the index of this result's promise within the list of promises
	// passed to the extension call.
	idx int
}

func (p *Promise[T]) Val() T {
	return p.Res().Val()
}

func (p *Promise[T]) Err() error {
	return p.Res().Err()
}

func (p *Promise[T]) State() State {
	return p.Res().State()
}

// String will block until the promise is resolved.
func (p *Promise[T]) String() string {
	return fmt.Sprintf("%v", p.Res())
}

func (p *Promise[T]) Wait() {
	p.regChainWait()
	p.waitCall()
}

func (p *Promise[T]) WaitChan() <-chan struct{} {
	return p.syncCtx.Done()
}

func (p *Promise[T]) waitCall() {
	// wait the promise to be resolved and get its [State]
	p.waitState()
	// execute the side effects, if the chain doesn't have a read call nor
	// is already handled
	p.chainSideEffects(p.chainStatus.Load(), true)
}

func (p *Promise[T]) Res() Result[T] {
	p.regChainRead()
	return p.resCall()
}

func (p *Promise[T]) resCall() Result[T] {
	// wait the promise to be resolved and get its [State]
	p.waitState()

	// if it's a call to handle the result, set the 'Handled' flag.
	// also, keep track of whether this handle was valid(first) or not,
	// to decide whether we should return the actual result of the promise
	// or an erroneous one.
	validHandle := p.setChainHandled()

	// if the promise isn't a one-time promise, all handle calls will be valid
	if !p.isOnetimePromise() {
		validHandle = true
	}

	// if the promise result has been used, return the expected error
	if !validHandle {
		return errPromiseConsumedResult[T]{}
	}

	// the promise result can be accessed multiple times...
	return getFinalRes(p.res)
}

// getFinalRes returns the final result to be used when returned outside
// the scope of the internal functions here.
func getFinalRes[T any](res Result[T]) Result[T] {
	// if no result was set, then it's implicitly the empty result
	if res == nil {
		return emptyResult[T]{}
	}
	return res
}

func (p *Promise[T]) Delay(
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		// since the syncCtx is nil, this promise will never be resolved,
		// so no point in allocating a new value.
		// note: this can only happen if the promise is created by passing a
		// Context value that's never canceled(nil Done) to the Ctx constructor.
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	flags := getDelayFlags(cond)
	debug(p, startHandler, startPromiseHandler, startPromiseDelayHandler)
	go delayFollowHandler(p, nextProm, d, flags)
	return nextProm
}

// delay creates Promise values with the same type
func delayFollowHandler[T any](
	prevProm *Promise[T],
	nextProm *Promise[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer prevProm.group.freeGoroutine()
	prevProm.waitState()

	// mark prevProm as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be used.
	res, ok := handleFollow(prevProm, nextProm, false)
	if !ok {
		// it's not a valid handle. it's considered a failure.
		if flags.onError {
			time.Sleep(dd)
		}
		nextProm.resolveToErrorRes(res)
		return
	}

	nextProm.resolveToResWithDelay(res, dd, flags)
	debug(prevProm, endHandler, endPromiseHandler, endPromiseDelayHandler)
}

func (p *Promise[T]) Callback(
	cb func(ctx context.Context, res Result[T]),
) {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return
	}
	if p.syncCtx == neverClosedSyncCtx {
		return
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return
	}

	ctx, cancel := callbackCtx(p.group, nil)
	debug(p, startHandler, startPromiseHandler, startPromiseCallbackHandler)
	go followHandler(
		p,
		nil,
		followFunc[T, T](cb),
		ctx,
		cancel,
		callbackOp,
		endHandler,
		endPromiseHandler,
		endPromiseCallbackHandler,
	)
}

func (p *Promise[T]) Follow(
	cb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseFollowHandler)
	go followHandler(
		p,
		nextProm,
		followResFunc[T, T](cb),
		ctx,
		cancel,
		followOp,
		endHandler,
		endPromiseHandler,
		endPromiseFollowHandler,
	)
	return nextProm
}

func (p *Promise[T]) FollowCallback(cb Callback[T, T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseFollowCallbackHandler)
	go followHandler(
		p,
		nextProm,
		cb,
		ctx,
		cancel,
		followOp,
		endHandler,
		endPromiseHandler,
		endPromiseFollowCallbackHandler,
	)
	return nextProm
}

func (p *Promise[T]) Then(
	cb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseThenHandler)
	go followHandler(
		p,
		nextProm,
		followResFunc[T, T](cb),
		ctx,
		cancel,
		thenOp,
		endHandler,
		endPromiseHandler,
		endPromiseThenHandler,
	)
	return nextProm
}

func (p *Promise[T]) Catch(
	cb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseCatchHandler)
	go followHandler(
		p,
		nextProm,
		followResFunc[T, T](cb),
		ctx,
		cancel,
		catchOp,
		endHandler,
		endPromiseHandler,
		endPromiseCatchHandler,
	)
	return nextProm
}

func (p *Promise[T]) Recover(
	cb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseRecoverHandler)
	go followHandler(
		p,
		nextProm,
		followResFunc[T, T](cb),
		ctx,
		cancel,
		recoverOp,
		endHandler,
		endPromiseHandler,
		endPromiseRecoverHandler,
	)
	return nextProm
}

func (p *Promise[T]) Finally(
	cb func(ctx context.Context),
) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.group.isWaiting() {
		return newPromSync[T](p.group, errPromiseGroupDoneResult[T]{})
	}
	if p.syncCtx == neverClosedSyncCtx {
		return p
	}
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errPromiseGroupBusyResult[T]{})
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncCtx)
	debug(p, startHandler, startPromiseHandler, startPromiseFinallyHandler)
	go followHandler(
		p,
		nextProm,
		ctxFunc[T, T](cb),
		ctx,
		cancel,
		finallyOp,
		endHandler,
		endPromiseHandler,
		endPromiseFinallyHandler,
	)
	return nextProm
}

func followHandler[NextT, PrevT any](
	prevProm *Promise[PrevT],
	nextProm *Promise[NextT],
	cb Callback[NextT, PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
	op followOperationLogic,
	endDebugEvents ...debugEvent,
) {
	defer prevProm.group.freeGoroutine()
	s := prevProm.waitState()

	// return and carry the current Result to the next Promise if it's not the target.
	if !op.IsTargetState(s) {
		nextRes := getEffectiveNextRes[NextT, PrevT](prevProm.res, nil)
		nextProm.resolveToRes(nextRes)
		return
	}

	// start with the previous Result.
	res := prevProm.res

	// mark prev as 'Handled', and check whether we should continue or not.
	if op.CanHandle() {
		var ok bool
		res, ok = handleFollow(prevProm, nextProm, op.ResolveOnInvalidHandle())
		if !ok && op.ReturnOnInvalidHandle() {
			return
		}
	}

	// run the callback with the actual promise result.
	runCallbackHandler(nextProm, cb, res, op.SupportHandleReturns(), ctx, cancel)
	debug(prevProm, endDebugEvents...)
}
