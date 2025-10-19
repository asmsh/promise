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

// Promise represents some asynchronous work (a goroutine),
// tracking its progress and recording its result.
//
// A Promise is safe for concurrent use by multiple goroutines.
//
// It implements the [Result] interface, so it can be returned from
// supported callbacks.
//
// The zero value will block forever on any calls.
type Promise[T any] struct {
	// group is a pointer to the promise group which this promise is part of,
	// or nil, if it's not part of any group.
	group *Group[T]

	// initiated lazily, by the first extension call.
	// this is sent on by the different extension calls.
	// it's never closed.
	extChanP atomic.Pointer[chan extQueue[T]]

	// closed when this promise is resolved.
	// it has one writer (one goroutine), which is the owner (this promise),
	// which will close it, but can have multiple readers (chain promises).
	//
	// note: it should be accessed directly only if it's known to not be nil,
	// otherwise it will be saved in res and should be accessed via WaitChan.
	syncChan chan struct{}

	// holds the result of the promise.
	// written once, before the syncChan is closed.
	//
	// don't read it unless the syncChan is known to be closed.
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
	// because the syncChan will be closed, and the extQ value will
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

// WaitChan returns a channel that will be closed once this [Promise]
// is resolved and its [Result] is available.
//
// Once the returned channel is closed, all blocked calls on this [Promise]
// will be unblocked, and all subsequent calls will return with no blocking.
//
// Subsequent calls return the same value.
func (p *Promise[T]) WaitChan() <-chan struct{} {
	// note: internally, this method is used whenever we want to wait
	// on Promise which its syncChan isn't guaranteed to be non-nil.
	// this generally only needed because of the Ctx constructors, because
	// the Done chan is a receive-only, and this implementation expects
	// a bi-direction syncChan, in order to be able to close it.
	// however, since the Ctx Done chan is closed by external actor,
	// we can't close it, and we only need to care about waiting on it.
	// so, for the purpose of saving the receive-only syncChan, it will
	// be saved in a Result value that implements: WaitChan() <-chan struct{}.

	// if the syncChan is already set, return it.
	if p.syncChan != nil {
		return p.syncChan
	}

	// if the syncChan is nil, then the current res is final and
	// the syncChan is saved in it.
	// note: this is needed for the Ctx constructors.
	if sr, ok := p.res.(interface{ WaitChan() <-chan struct{} }); ok {
		return sr.WaitChan()
	}

	// this will never happen.
	panic("promise: internal: unexpected syncChan value")
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

func (p *Promise[T]) validateState() *Promise[T] {
	// return an error if its group is in the waiting mode,
	// disallowing the initiation of any new work.
	if errRes := p.group.validateActive(); errRes != nil {
		return newPromSync[T](p.group, errRes)
	}

	if p.syncChan == neverClosedSyncChan {
		// since the syncChan is the never closed chan value, this promise
		// will never be resolved, so no point in allocating a new value.
		// note: this can only happen if the promise is created by passing a
		// Context value that's never canceled(nil Done) to the Ctx constructor.
		return p
	}

	// attempt to reserve a goroutine for the rescheduling,
	// or return an error if there's no available ones.
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync[T](p.group, errGroupBusyResult[T]{})
	}

	return nil
}

func (p *Promise[T]) Delay(
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if errProm := p.validateState(); errProm != nil {
		return errProm
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

	// note: this will also handle the error, if any, synchronously.
	if errProm := p.validateState(); errProm != nil {
		// TODO: the returned errProm isn't needed, so maybe get rid of it.
		return
	}

	ctx, cancel := callbackCtx(p.group, nil)
	debug(p, startHandler, startPromiseHandler, startPromiseCallbackHandler)
	go followHandler(
		p,
		nil,
		followResFunc[T, T](cb),
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
	debug(p, startHandler, startPromiseHandler, startPromiseFollowHandler)
	go followHandler(
		p,
		nextProm,
		followResNextResFunc[T, T](cb),
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
	debug(p, startHandler, startPromiseHandler, startPromiseThenHandler)
	go followHandler(
		p,
		nextProm,
		followResNextResFunc[T, T](cb),
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
	debug(p, startHandler, startPromiseHandler, startPromiseCatchHandler)
	go followHandler(
		p,
		nextProm,
		followResNextResFunc[T, T](cb),
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
	debug(p, startHandler, startPromiseHandler, startPromiseRecoverHandler)
	go followHandler(
		p,
		nextProm,
		followResNextResFunc[T, T](cb),
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

	if errProm := p.validateState(); errProm != nil {
		return errProm
	}

	nextProm := newPromInter[T](p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
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
		if nextProm == nil {
			// this should not happen, as operations that defines a certain
			// set of target states must create and pass a next promise.
			panic("promise: internal: next promise is unexpectedly nil")
		}

		nextRes := getEffectiveNextRes[NextT, PrevT](prevProm.res, nil)
		nextProm.resolveToRes(nextRes)
		return
	}

	// start with the previous Result.
	res := prevProm.res

	// mark prev Handled and check whether we should resolve next or continue.
	if op.CanHandle() {
		if nextProm == nil {
			// this should not happen, as operations that can handle a target
			// promise must create and pass a next promise.
			panic("promise: internal: next promise is unexpectedly nil")
		}

		var valid bool
		res, valid = handleFollow(prevProm, nextProm, op.ResolveOnInvalidHandle())
		if !valid && op.ResolveOnInvalidHandle() {
			return
		}
	}

	// run the callback with the actual promise result.
	runCallbackHandler(nextProm, cb, res, ctx, cancel)
	debug(prevProm, endDebugEvents...)
}
