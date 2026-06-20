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
	_ noCopy

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
	// note: it should be accessed directly only if it's not be nil,
	// otherwise it will be saved in res and should be accessed via WaitChan.
	syncChan chan struct{}

	// holds the result of the promise.
	// written once, before the syncChan is closed.
	//
	// don't read it unless the syncChan is closed or nil.
	res Result[T]

	// chainStatus holds the status of this promise's chain.
	// once there are any calls registered on this promise, or
	// the promise has been already handled, it will be non-zero.
	chainStatus atomic.Uint32
}

// extQueue will be owned, at any time, by a single goroutine.
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
	// this is typically an extension call's promise's syncChan.
	syncChan <-chan struct{}

	// idx is the index of this result's promise within the list of promises
	// passed to the extension call.
	idx int
}

// Val returns the value resulting from the promise's execution.
// It blocks until the promise is resolved.
//
// Subsequent calls return the same value, unless [GroupConfig.OnetimeHandling]
// is enabled, in which the zero value of T is returned.
func (p *Promise[T]) Val() T {
	return p.WaitRes().Val()
}

// Err returns the error resulting from the promise's execution.
// It blocks until the promise is resolved.
//
// Subsequent calls return the same value, unless [GroupConfig.OnetimeHandling]
// is enabled, in which case [ErrPromiseConsumed] is returned.
func (p *Promise[T]) Err() error {
	return p.WaitRes().Err()
}

// State returns the final state of the promise's execution.
// It blocks until the promise is resolved.
//
// Subsequent calls return the same value, unless [GroupConfig.OnetimeHandling]
// is enabled, in which case [Error] is returned.
func (p *Promise[T]) State() State {
	return p.WaitRes().State()
}

// String will block until the promise is resolved.
func (p *Promise[T]) String() string {
	return fmt.Sprintf("%v", p.WaitRes())
}

// Wait blocks until the promise is resolved.
func (p *Promise[T]) Wait() {
	p.regChainWait()

	// wait the promise to be resolved.
	<-p.WaitChan()

	// execute the side effects if the chain doesn't have a read call nor
	// is already handled.
	p.chainSideEffects(true)
}

// WaitChan returns a channel that will be closed once this [Promise]
// is resolved and its [Result] is available.
//
// Once the channel is closed, all calls blocked on this [Promise]
// are unblocked, and any subsequent calls complete without blocking.
//
// Subsequent calls return the same value.
func (p *Promise[T]) WaitChan() <-chan struct{} {
	// note: this method is the entry point to using the syncChan,
	// as it handles the syncChan being nil.
	// the syncChan will be nil in the case of the Ctx constructors,
	// which saves the syncChan in res as a Result value that implements:
	// WaitChan() <-chan struct{}.
	// this is only needed because the Context.Done chan is a receive-only,
	// and this implementation expects a bi-direction syncChan to be able
	// to close it.
	// however, since the Context.Done chan is closed by an external actor,
	// we can't close it, and we only need to care about waiting on it.

	// if the syncChan is already set, return it.
	if p.syncChan != nil {
		return p.syncChan
	}

	// otherwise, try to read it from the res.
	return p.waitChanSlow()
}

// waitChanSlow is outlined to allow WaitChan to be inlined.
func (p *Promise[T]) waitChanSlow() <-chan struct{} {
	// the syncChan is nil, so if the current res is set,
	// the syncChan will be saved in it.
	// note: this is needed for the Ctx constructors.
	if p.res != nil {
		if sr, ok := p.res.(interface{ WaitChan() <-chan struct{} }); ok {
			return sr.WaitChan()
		}

		// this will never happen.
		panic("promise: internal: unexpected syncChan value")
	}

	// otherwise, this is a zero Promise, return the default sync chan.
	return neverClosedSyncChan
}

// WaitRes return the [Result] of the promise's execution.
// It blocks until the promise is resolved.
//
// Subsequent calls return the same value, unless [GroupConfig.OnetimeHandling]
// is enabled, in which case a [Result] with the [ErrPromiseConsumed] error
// is returned.
func (p *Promise[T]) WaitRes() Result[T] {
	p.regChainRead()

	// wait the promise to be resolved.
	<-p.WaitChan()

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
	// if no result was set, then it's implicitly the zero result
	if res == nil {
		return zeroResult[T]{}
	}
	return res
}

func (p *Promise[T]) validateState() *Promise[T] {
	// return an error if its group is in the waiting mode,
	// disallowing the initiation of any new work.
	if errRes := p.group.validateActive(); errRes != nil {
		return newPromSync(p.group, errRes)
	}

	if p.syncChan == neverClosedSyncChan {
		// since the syncChan is the never closed chan value, this promise
		// will never be resolved, so no point in allocating a new value.
		// note: this can only happen if the promise is created by passing a
		// Context value that's never canceled(nil Done) to the Ctx constructor.
		return p
	}

	// attempt to reserve a goroutine for the rescheduling
	// or return an error if there are no available ones.
	if !p.group.reserveGoroutine(p.regChainRead) {
		return newPromSync(p.group, errGroupBusyResult[T]{})
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

	nextProm := newPromAsync(p.group)
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

	<-prevProm.WaitChan()

	// mark prevProm as 'Handled', and check whether we should continue or not.
	// the res value returned will hold the correct value that should be used.
	res, ok := handleFollow(prevProm, nextProm, false)
	if !ok {
		// it's not a valid handle. it's considered a failure.
		if flags.onError {
			time.Sleep(dd)
		}
		nextProm.resolveToRes(res)
		return
	}

	if shouldDelayRes(res, flags) {
		time.Sleep(dd)
	}
	nextProm.resolveToRes(res)

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
		// TODO: the returned errProm isn't needed, so avoid creating it.
		//  we only need the chainSideEffects call.
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
		false,
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

	nextProm := newPromAsync(p.group)
	ctx, cancel := callbackCtx(p.group, nextProm.syncChan)
	debug(p, startHandler, startPromiseHandler, startPromiseFollowHandler)
	go followHandler(
		p,
		nextProm,
		followResNextResFunc[T, T](cb),
		ctx,
		cancel,
		true,
		endHandler,
		endPromiseHandler,
		endPromiseFollowHandler,
	)
	return nextProm
}

func followHandler[NextT, PrevT any](
	prevProm *Promise[PrevT],
	nextProm *Promise[NextT],
	cb callback[NextT, PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
	canHandle bool,
	endDebugEvents ...debugEvent,
) {
	defer prevProm.group.freeGoroutine()

	<-prevProm.WaitChan()

	// start with the previous Result.
	res := prevProm.res

	// mark prev Handled and check whether we should resolve next or continue.
	if canHandle {
		if nextProm == nil {
			// this should not happen, as operations that can handle a target
			// promise must create and pass a next promise.
			panic("promise: internal: next promise is unexpectedly nil")
		}

		var valid bool
		res, valid = handleFollow(prevProm, nextProm, canHandle)
		if !valid {
			return
		}
	}

	// run the callback with the actual promise result.
	runCallbackHandler(nextProm, cb, res, ctx, cancel)
	debug(prevProm, endDebugEvents...)
}
