// Copyright 2025 Ahmad Sameh(asmsh)
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
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asmsh/sema"
)

// Group represents a group of [Promise] values that operate on the same
// input and result types, offering methods to start new promises, wait
// for all ongoing ones, and have a centralized handling for their failures.
//
// Promises from the same [Group] share the same goroutines pool, if one
// is set.
//
// A Group is safe for concurrent use by multiple goroutines.
//
// The zero value is a ready to use Group with the [DefaultGroupConfig].
type Group[T any] struct {
	_ noCopy

	core *groupCore
	once sync.Once

	// callsQ is used for registering and tracking the [Group]'s join calls,
	// like [Group.AnyRes], [Group.AllWaitRes], etc.
	callsQMu sync.RWMutex
	callsQ   list.List // of groupCall[T]

	// resQ is used to hold all [Result] values returned from promises
	// that belong to this [Group] when the [GroupConfig.SaveAllGroupResults]
	// option is set.
	// it's a linked-list instead of an array, because the size is unpredictable.
	// resHist holds the history of [Result] values for either first values
	// or the last ones, depending on the [saveLastSingleGroupResult] flag.
	// resMu protects reads and writes to them.
	resMu   sync.RWMutex
	resQ    list.List // of GroupRes[T]
	resHist groupResHistory[T]
}

// NewGroup creates and returns a new [Group] value of type T,
// initialized with the provided options.
//
// When no options are provided, it's equivalent to a zero-value [Group],
// which is ready to use with the [DefaultGroupConfig].
func NewGroup[T any](opts ...GroupOption) *Group[T] {
	g := &Group[T]{}
	g.init()
	for _, opt := range opts {
		opt(g.core)
	}
	return g
}

func (g *Group[T]) init() {
	if g == nil {
		return
	}
	g.once.Do(func() {
		g.core = &groupCore{}
	})
}

// groupCore contains read-only, share-able, or type-agnostic fields only.
type groupCore struct {
	debugCB func([]debugEvent)

	unhandledPanicCB func(any)
	unhandledErrorCB func(error)

	// waiting represents whether this group has entered the waiting
	// mode or not.
	// it can enter the waiting mode via one of the Wait methods.
	waiting atomic.Bool

	// canceled represents whether this Group has been canceled or not,
	// which happens if there's been an unhandled failure and
	// the [GroupConfig.FailuresCancelGroup] flag was set.
	canceled atomic.Bool

	// sg is used for limiting concurrency and implementing
	// the waiting functions.
	sg sema.Group

	// ctx will be non-nil if the Group is meant to close all Context values
	// once any Promise that's created using it panics or returns an error.
	// if neverCancelCBCtx is true, these 2 fields will be unset.
	ctx    context.Context
	cancel context.CancelFunc

	// options is generated from groupOptions.
	options groupOptionsBitFlags
}

//go:generate genflagged -type=groupOptions -outType=groupOptionsBitFlags -outFile=group_options_flagged.go
type groupOptions struct {
	onetimeHandling     bool
	neverCancelCBCtx    bool
	failuresCancelCBCtx bool
	failuresCancelGroup bool
	noNilCtxDoneChan    bool
	noWaitingBusyGroup  bool
	saveAllGroupResults bool
}

func noopRegFunc() {
	// do nothing
}

// Go runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The [Result.State] will either be [Panic], or [Success], based on whether
// cb caused a panic, or returned normally, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) Go(cb func()) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, goFunc[T, T](cb))
}

// GoErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned a non-nil error, or returned a nil error,
// respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) GoErr(cb func() error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, goNextErrFunc[T, T](cb))
}

// GoValErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) GoValErr(cb func() (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, goNextValErrFunc[T, T](cb))
}

// GoCtxErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The cb function receives a [context.Context] value derived from the [Group]'s
// context, if one is configured, and is canceled once cb returns.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned a non-nil error, or returned a nil error,
// respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return nil.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) GoCtxErr(cb func(ctx context.Context) error) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, ctxNextErrFunc[T, T](cb))
}

// GoCtxValErr runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The cb function receives a [context.Context] value derived from the [Group]'s
// context, if one is configured, and is canceled once cb returns.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) GoCtxValErr(cb func(ctx context.Context) (T, error)) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, ctxNextValErrFunc[T, T](cb))
}

// GoCtxRes runs the provided function, cb, in a separate goroutine, and returns
// a [Promise] value whose [Promise.WaitRes] tracks the execution of cb.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// The cb function receives a [context.Context] value derived from the [Group]'s
// context, if one is configured, and is canceled once cb returns.
//
// The [Result.State] will either be [Panic], [Error], or [Success], based on
// whether cb caused a panic, returned an error, or returned a value, respectively.
//
// If the [Result.State] is [Panic], [Result.Err] will return a [PanicError]
// that wraps the panic value that occurred.
// If the [Result.State] is [Error], [Result.Err] will return a non-nil error
// that wraps the error that cb returned.
// If the [Result.State] is [Success], [Result.Val] will return the value
// that cb returned.
//
// If cb called runtime.Goexit, [Result.State] will be [Error] and [Result.Err]
// will return [ErrPromiseGoexit].
//
// It will panic if cb is nil.
func (g *Group[T]) GoCtxRes(cb func(ctx context.Context) Result[T]) *Promise[T] {
	if cb == nil {
		panic(nilCallbackPanicMsg)
	}

	return goCallback(g, ctxNextResFunc[T, T](cb))
}

func goCallback[NextT, PrevT any](
	g *Group[NextT],
	cb callback[NextT, PrevT],
) *Promise[NextT] {
	// make sure the group and in a valid state, otherwise return
	// the error promise created.
	if errRes := g.initValidateReserve(); errRes != nil {
		return newPromSync(g, errRes)
	}

	// create the new promise that will track the provided callback.
	nextProm := newPromAsync(g)

	// derive the Context used in the callback from the group's, or from
	// the new promise, effectively making the created Context closed
	// once the callback returns.
	ctx, cancel := callbackCtx(g, nextProm.syncChan)

	debug(nextProm, startHandler, startGroupHandler, startGroupGoHandler)
	go goCallbackHandler(nextProm, cb, ctx, cancel)

	return nextProm
}

func goCallbackHandler[NextT, PrevT any](
	nextProm *Promise[NextT],
	cb callback[NextT, PrevT],
	ctx context.Context,
	cancel context.CancelFunc,
) {
	defer nextProm.group.freeGoroutine()
	runCallbackHandler[NextT, PrevT](nextProm, cb, nil, ctx, cancel)
	debug(nextProm, endHandler, endGroupHandler, endGroupGoHandler)
}

// Delay returns a [Promise] that resolves to res after waiting for at least
// duration d in a separate goroutine, according to how the [State] of res
// matches the provided cond.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
func (g *Group[T]) Delay(
	res Result[T],
	d time.Duration,
	cond ...DelayCond,
) *Promise[T] {
	if errRes := g.initValidateReserve(); errRes != nil {
		return newPromSync(g, errRes)
	}

	nextProm := newPromAsync(g)
	flags := getDelayFlags(cond)
	debug(nextProm, startHandler, startGroupHandler, startGroupDelayHandler)
	go delayHandler(nextProm, res, d, flags)
	return nextProm
}

func delayHandler[T any](
	nextProm *Promise[T],
	res Result[T],
	dd time.Duration,
	flags delayFlags,
) {
	defer nextProm.group.freeGoroutine()
	if shouldDelayRes(res, flags) {
		time.Sleep(dd)
	}
	nextProm.resolveToRes(res)
	debug(nextProm, endHandler, endGroupHandler, endGroupDelayHandler)
}

// Chan returns a [Promise] that wraps the provided [Result] channel, resChan,
// waiting in a separate goroutine for the first [Result] value sent to it,
// which will be used to resolve the returned [Promise].
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
//
// Only one [Result] value is received from resChan, and any later values
// will not be received (and will block if the channel becomes full).
//
// Closing the resChan will have the same effect as sending nil to it.
//
// Sending nil to resChan will make the [Result.State] return [Success],
// [Result.Err] return nil, and [Result.Val] return nil.
//
// It will panic if resChan is nil.
func (g *Group[T]) Chan(resChan <-chan Result[T]) *Promise[T] {
	if resChan == nil {
		panic(nilResChanPanicMsg)
	}

	if errRes := g.initValidateReserve(); errRes != nil {
		return newPromSync(g, errRes)
	}

	nextProm := newPromAsync(g)
	debug(nextProm, startHandler, startGroupHandler, startGroupChanHandler)
	go chanHandler(nextProm, resChan)
	return nextProm
}

func chanHandler[T any](nextProm *Promise[T], resChan <-chan Result[T]) {
	defer nextProm.group.freeGoroutine()
	res := <-resChan
	nextProm.resolveToRes(res)
	debug(nextProm, endHandler, endGroupHandler, endGroupChanHandler)
}

// Ctx returns a [Promise] that wraps the provided [context.Context] value, ctx,
// that's resolved in a separate goroutine once the ctx is canceled.
// The goroutine is drawn from this [Group]'s pool, if one is set, and the
// returned [Promise] is tracked by this [Group]'s wait and join operations.
// The [Promise.WaitRes] returns a [Result] value that allows knowing the [State]
// of ctx (via [Result.State]), and the error returned from ctx (via [Result.Err]).
//
// Once the [context.Context.Done] channel is closed, the [Result.State] will
// either be [Error], or [Success], based on whether [context.Context.Err]
// returned an error, or nil, respectively.
//
// If the [context.Context.Done] channel is nil or never closed, the returned
// [Promise] value will never resolve, meaning that all its methods will block.
// This behavior can be changed by the [GroupConfig.NoNilCtxDoneChan] option.
//
// It will panic if ctx is nil.
func (g *Group[T]) Ctx(ctx context.Context) *Promise[T] {
	if ctx == nil {
		panic(nilCtxPanicMsg)
	}

	g.init()

	if errRes := g.validateActive(); errRes != nil {
		return newPromSync(g, errRes)
	}

	// handle contexts with nil done channel, as a nil channel is never closed,
	// and if used with this package it will block follow calls on it forever.
	if ctx.Done() == nil {
		if g != nil && g.core.options.IsNoNilCtxDoneChan() {
			return newPromSync(g, errCtxNilDoneResult[T]{})
		}

		// since this ctx value will never be closed, the equivalent
		// outcome would be a Promise that's never resolved.
		// so, return that equivalent value without creating any unneeded
		// resources.
		return newPromBlocked[T]()
	}

	// TODO: handle already closed Done chan, so that we don't create
	//  a new goroutine if the chan is already closed.

	if !g.reserveGoroutine(noopRegFunc) {
		return newPromSync(g, errGroupBusyResult[T]{})
	}

	nextProm := newPromCtx(g, ctx)
	debug(nextProm, startHandler, startGroupHandler, startGroupCtxHandler)
	go ctxHandler(nextProm)
	return nextProm
}

func ctxHandler[T any](nextProm *Promise[T]) {
	defer nextProm.group.freeGoroutine()
	// block on receiving from the wait chan, which will be a Context.Done
	// channel already saved in the nextProm.res and closed by the Context.
	<-nextProm.WaitChan()
	// passing the nextProm.res again is redundant, as it's already saved,
	// but just done to keep reusing the same helper methods.
	nextProm.resolveToRes(nextProm.res)
	debug(nextProm, endHandler, endGroupHandler, endGroupCtxHandler)
}

// Wrap returns a [Promise] that wraps the provided [Result] value, res,
// synchronously, without creating any new goroutines.
// The returned [Promise] belongs to this [Group].
// The [Promise.WaitRes] will return the provided res.
func (g *Group[T]) Wrap(res Result[T]) *Promise[T] {
	g.init()

	if errRes := g.validateActive(); errRes != nil {
		return newPromSync(g, errRes)
	}

	if res != nil && res.State() == Panic {
		if _, ok := res.Err().(resultPanicV); !ok {
			res = PanicRes[T](res)
		}
	}

	return newPromSync(g, res)
}

// Wait enters the wait mode, blocking new calls from starting,
// then waits for all ongoing promises on this [Group] to return.
// If there are any triggers provided, it executes them first.
//
// Example:
//
//	g.Wait(func() { <-signalChan })
func (g *Group[T]) Wait(triggers ...func()) {
	g.init()
	// execute the trigger functions to block the wait logic until they return.
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true) // blocks new calls from being started.
	g.core.sg.Wait()
}

// SelectRes returns the first [Result] value of the first resolved [Promise]
// that started on this [Group].
// It always returns the same [Result] value.
func (g *Group[T]) SelectRes(triggers ...func()) Result[GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.selectRes()
}

// AllRes returns a [Result] of [Success] after all the promises
// of this [Group] are resolved to [Success].
// Otherwise, it returns early a [Result] of [Panic] or [Error],
// once a [Promise] is resolved to [Panic] or [Error], respectively.
//
// It will return a [Result] of [Success] if it's called on a zero [Group].
//
// It returns the [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the [GroupOption]
// [SaveAllGroupResults].
//
// The [Panic] and [Error] values can be return via [Result.Err],
// which is a [MultiError] wrapping an [GroupError] errors.
// And each [GroupError] is wrapping a [PanicError] or other error
// values, for [Panic] or [Error], respectively.
func (g *Group[T]) AllRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.joinRes(allOp)
}

// AllWaitRes causes the [Group] to enter the wait mode, blocking new promises
// from being created, then waits for all ongoing promises on it to return.
//
// It returns a [Result] of [Success] if all the promises were resolved
// to [Success].
// Otherwise, it returns a [Result] of [Panic] or [Error], if at least one
// promise was resolved to [Panic] or [Error], respectively.
//
// It will return a [Result] of [Success] if it's called on a zero [Group].
//
// It returns the [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the [GroupOption]
// [SaveAllGroupResults].
//
// The [Panic] and [Error] values can be return via [Result.Err],
// which is a [MultiError] wrapping an [GroupError] errors.
// And each [GroupError] is wrapping a [PanicError] or other error
// values, for [Panic] or [Error], respectively.
func (g *Group[T]) AllWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(allWaitOp)
	g.core.sg.Wait()
	return res
}

// AnyRes returns a [Result] of [Success] once one of the promises
// of this [Group] is resolved to [Success].
// Otherwise, it waits and returns a [Result] of [Panic] or [Error],
// if at least one is resolved to [Panic] or [Error], respectively.
//
// It will return a [Result] of [Success] if it's called on a zero [Group].
//
// It returns the [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the [GroupOption]
// [SaveAllGroupResults].
//
// The [Panic] and [Error] values can be return via [Result.Err],
// which is a [MultiError] wrapping an [GroupError] errors.
// And each [GroupError] is wrapping a [PanicError] or other error
// values, for [Panic] or [Error], respectively.
func (g *Group[T]) AnyRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	return g.joinRes(anyOp)
}

// AnyWaitRes causes the [Group] to enter the wait mode, blocking new promises
// from being created, then waits for all ongoing promises on it to return.
//
// It returns a [Result] of [Success] if at least one of the promises was
// resolved to [Success].
// Otherwise, it returns a [Result] of [Panic] or [Error], if at least one
// was resolved to [Panic] or [Error], respectively.
//
// It will return a [Result] of [Success] if it's called on a zero [Group].
//
// It returns the [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the [GroupOption]
// [SaveAllGroupResults].
//
// The [Panic] and [Error] values can be return via [Result.Err],
// which is a [MultiError] wrapping an [GroupError] errors.
// And each [GroupError] is wrapping a [PanicError] or other error
// values, for [Panic] or [Error], respectively.
func (g *Group[T]) AnyWaitRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(anyWaitOp)
	g.core.sg.Wait()
	return res
}

// JoinRes causes the [Group] to enter the wait mode, blocking new promises
// from being created, then waits for all ongoing promises on it to return.
//
// It returns a [Result] of [Success] with all promises' [Result] values.
//
// It returns the [Result] values either from the start of this [Group],
// or after the provided triggers have been called, based on the [GroupOption]
// [SaveAllGroupResults].
func (g *Group[T]) JoinRes(triggers ...func()) Result[[]GroupRes[T]] {
	g.init()
	for _, f := range triggers {
		f()
	}
	g.core.waiting.Store(true)
	res := g.joinRes(joinOp)
	g.core.sg.Wait()
	return res
}

func (g *Group[T]) initValidateReserve() Result[T] {
	// make sure the group is initiated.
	g.init()

	// return an error if the group is in the waiting mode,
	// disallowing the initiation of any new work.
	if errRes := g.validateActive(); errRes != nil {
		return errRes
	}

	// attempt to reserve a goroutine for the callback,
	// or return an error if there's no available ones.
	if !g.reserveGoroutine(noopRegFunc) {
		return errGroupBusyResult[T]{}
	}

	return nil
}

// validateActive will return an [Error] [Result] if the [Group] has been closed
// because of either it is in Waiting mode, or it has been Canceled due to a failure.
func (g *Group[T]) validateActive() Result[T] {
	if g == nil {
		return nil
	}

	if g.isWaiting() {
		return errGroupWaitingResult[T]{}
	}
	if g.isCanceled() {
		return errGroupCanceledResult[T]{}
	}

	return nil
}

// isWaiting should be only used through validateActive.
func (g *Group[T]) isWaiting() bool {
	return g.core.waiting.Load()
}

// isCanceled should be only used through validateActive.
func (g *Group[T]) isCanceled() bool {
	if !g.core.options.IsFailuresCancelGroup() {
		// since we only write to the canceled field if that option is set,
		// then there's no point in attempting to read it if not set.
		return false
	}
	return g.core.canceled.Load()
}

// note: once it returns true, an arrangement must be made for calling freeGoroutine.
func (g *Group[T]) reserveGoroutine(chainRegFunc func()) bool {
	if g == nil {
		// execute the register func and mark this reservation successful.
		chainRegFunc()
		return true
	}

	// either block until a place is available, or return an error with no waiting.
	if g.core.options.IsNoWaitingBusyGroup() {
		if g.core.sg.TryReserveN(1) {
			// since we entered this case, and this is a non-blocking select,
			// this case will happen immediately.
			chainRegFunc()
			return true
		}
		return false
	}

	// it's guaranteed to make a successful reservation in this flow (after waiting),
	// execute the register func before waiting.
	chainRegFunc()
	g.core.sg.Reserve()
	return true
}

// note: if the program is exiting, this call might not be executed,
// so no important clean up or logic should be included here.
func (g *Group[T]) freeGoroutine() {
	if g == nil {
		return
	}
	g.core.sg.Free()
}
