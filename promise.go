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
	// or nil, if it's part of the default group.
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

	// resolveState holds the state of this promise.
	// once the promise is resolved, it will be non-zero.
	// written once, before the syncCtx channel is closed.
	resolveState atomic.Uint32

	// chainStatus holds the status of this promise's chain.
	// once there are any calls registered on this promise, or
	// the promise has been already handled, it will be non-zero.
	chainStatus atomic.Uint32
}

// extQueue wil be owned, at any time, by a single goroutine.
type extQueue[T any] struct {
	// initState is the state value at the time this queue was created.
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
	// wait the promise to be resolved
	s := p.wait()

	// if the chain has a read call or is already handled, return early
	if p.chainStatus.Load() >= chainStatusRead {
		return
	}

	// at this point, the promise is not handled and doesn't have a read call,
	// so call the unhandled handlers if the state is one of a failure.
	switch s {
	case Rejected:
		p.uncaughtErrorHandler()
	case Panicked:
		p.uncaughtPanicHandler()
	}
}

func (p *Promise[T]) Res() Result[T] {
	p.regChainRead()
	return p.resCall()
}

func (p *Promise[T]) resCall() Result[T] {
	// wait the promise to be resolved
	p.wait()

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

func (p *Promise[T]) Callback(
	cb func(ctx context.Context, res Result[T]),
) {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.syncCtx == nil {
		return
	}

	p.regChainRead()
	p.group.reserveGoroutine()
	ctx, cancel := p.group.callbackCtx(nil)
	debug(p, startHandler, startFollowHandler, startCallbackFollowHandler)
	go callbackFollowHandler(p, cb, ctx, cancel)
}

func callbackFollowHandler[T any](
	prevProm *Promise[T],
	cb callbackCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	// make sure we free this goroutine reservation
	defer prevProm.group.freeGoroutine()

	// wait the previous promise to be resolved
	prevProm.wait()

	// run the callback with the actual promise result
	runCallbackHandler[T, T](nil, cb, prevProm.res, false, false, ctx, cancel)
	debug(prevProm, endHandler, endFollowHandler, endCallbackFollowHandler)
}

func (p *Promise[T]) Delay(
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if p.syncCtx == nil {
		return newPromBlocking[T]()
	}

	p.regChainRead()
	flags := getDelayFlags(cond)
	p.group.reserveGoroutine()
	nextProm := newPromInter[T](p.group)
	debug(p, startHandler, startFollowHandler, startDelayFollowHandler)
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
	prevProm.wait()

	// mark prevProm as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be used.
	res, ok := handleFollow(prevProm, nextProm, false)
	if !ok {
		// it's not a valid handle. it's considered a failure.
		if flags.onError {
			time.Sleep(dd)
		}
		resolveToRejectedRes(nextProm, res)
		return
	}

	resolveToResWithDelay(nextProm, res, dd, flags)
	debug(prevProm, endHandler, endFollowHandler, endDelayFollowHandler)
}

func (p *Promise[T]) Then(
	thenCb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if thenCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.syncCtx == nil {
		return newPromBlocking[T]()
	}

	p.regChainRead()
	p.group.reserveGoroutine()
	nextProm := newPromInter[T](p.group)
	ctx, cancel := p.group.callbackCtx(nextProm.syncCtx)
	debug(p, startHandler, startFollowHandler, startThenFollowHandler)
	go thenFollowHandler(p, nextProm, thenCb, ctx, cancel)
	return nextProm
}

func thenFollowHandler[T any](
	prevProm *Promise[T],
	nextProm *Promise[T],
	cb followCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer prevProm.group.freeGoroutine()
	s := prevProm.wait()

	// 'Then' can handle only the 'Fulfilled' state, so return otherwise
	if s != Fulfilled {
		resolveToRes(nextProm, prevProm.res)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prevProm, nextProm, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallbackHandler[T, T](nextProm, cb, res, true, true, ctx, cancel)
	debug(prevProm, endHandler, endFollowHandler, endThenFollowHandler)
}

func (p *Promise[T]) Catch(
	catchCb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if catchCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.syncCtx == nil {
		return newPromBlocking[T]()
	}

	p.regChainRead()
	p.group.reserveGoroutine()
	nextProm := newPromInter[T](p.group)
	ctx, cancel := p.group.callbackCtx(nextProm.syncCtx)
	debug(p, startHandler, startFollowHandler, startCatchFollowHandler)
	go catchFollowHandler(p, nextProm, catchCb, ctx, cancel)
	return nextProm
}

func catchFollowHandler[T any](
	prevProm *Promise[T],
	nextProm *Promise[T],
	cb followCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer prevProm.group.freeGoroutine()
	s := prevProm.wait()

	// 'Catch' can handle only the 'Rejected' state, so return otherwise
	if s != Rejected {
		resolveToRes(nextProm, prevProm.res)
		return
	}

	// mark prev as 'Handled'.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, _ := handleFollow(prevProm, nextProm, false)

	// run the callback with the actual promise result
	runCallbackHandler[T, T](nextProm, cb, res, true, true, ctx, cancel)
	debug(prevProm, endHandler, endFollowHandler, endCatchFollowHandler)
}

func (p *Promise[T]) Recover(
	recoverCb func(ctx context.Context, res Result[T]) Result[T],
) *Promise[T] {
	if recoverCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.syncCtx == nil {
		return newPromBlocking[T]()
	}

	p.regChainRead()
	p.group.reserveGoroutine()
	nextProm := newPromInter[T](p.group)
	ctx, cancel := p.group.callbackCtx(nextProm.syncCtx)
	debug(p, startHandler, startFollowHandler, startRecoverFollowHandler)
	go recoverFollowHandler(p, nextProm, recoverCb, ctx, cancel)
	return nextProm
}

func recoverFollowHandler[T any](
	prevProm *Promise[T],
	nextProm *Promise[T],
	cb followCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer prevProm.group.freeGoroutine()
	s := prevProm.wait()

	// 'Recover' can handle only the 'Panicked' state, so return otherwise
	if s != Panicked {
		resolveToRes(nextProm, prevProm.res)
		return
	}

	// mark prev as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be handled
	// by the callback.
	res, ok := handleFollow(prevProm, nextProm, true)
	if !ok {
		// return, since the promise is now resolved
		return
	}

	// run the callback with the actual promise result
	runCallbackHandler[T, T](nextProm, cb, res, true, true, ctx, cancel)
	debug(prevProm, endHandler, endFollowHandler, endRecoverFollowHandler)
}

func (p *Promise[T]) Finally(
	finallyCb func(ctx context.Context),
) *Promise[T] {
	if finallyCb == nil {
		panic(nilCallbackPanicMsg)
	}
	if p.syncCtx == nil {
		return newPromBlocking[T]()
	}

	p.regChainRead()
	p.group.reserveGoroutine()
	nextProm := newPromInter[T](p.group)
	ctx, cancel := p.group.callbackCtx(nextProm.syncCtx)
	debug(p, startHandler, startFollowHandler, startFinallyFollowHandler)
	go finallyFollowHandler(p, nextProm, finallyCb, ctx, cancel)
	return nextProm
}

// finallyFollowHandler is like an asyncReadCall, except that it can't set the 'Handled'
// flag(handle the promise), and it can return new promise with new result.
// if we made the finally a normal 'follow' method(like then,..), it will be
// possible to call it on a panicked promise and return a fulfilled promise,
// and the panic will be dismissed implicitly, which is something we don't want.
func finallyFollowHandler[T any](
	prevProm *Promise[T],
	nextProm *Promise[T],
	cb finallyCallback[T, T],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer prevProm.group.freeGoroutine()
	prevProm.wait()
	runCallbackHandler[T, T](nextProm, cb, prevProm.res, false, true, ctx, cancel)
	debug(prevProm, endHandler, endFollowHandler, endFinallyFollowHandler)
}
