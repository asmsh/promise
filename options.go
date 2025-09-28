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

package promise

import "context"

type GroupOption func(core *groupCore)

func ApplyConfig(conf GroupConfig) GroupOption {
	return func(core *groupCore) {
		if cb := conf.UnhandledPanicCB; cb != nil {
			core.unhandledPanicCB = cb
		}

		if cb := conf.UnhandledErrorCB; cb != nil {
			core.unhandledErrorCB = cb
		}

		core.sg.SetSize(conf.Size)

		if conf.OnetimeHandling {
			core.options.SetOnetimeHandling()
		}

		if conf.NeverCancelCBCtx && !conf.FailuresCancelCBCtx && !conf.FailuresCancelGroup {
			core.options.SetNeverCancelCBCtx()
		}

		if conf.FailuresCancelCBCtx {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetFailuresCancelCBCtx()
			core.options.ResetNeverCancelCBCtx()
		}

		if conf.FailuresCancelGroup {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetFailuresCancelGroup()
			core.options.ResetNeverCancelCBCtx()
		}

		if conf.NoNilCtxDoneChan {
			core.options.SetNoNilCtxDoneChan()
		}

		if conf.NoWaitingBusyGroup {
			core.options.SetNoWaitingBusyGroup()
		}

		if conf.SaveAllGroupResults {
			core.options.SetSaveAllGroupResults()
		}
	}
}

func UnhandledPanicCB(cb func(any)) GroupOption {
	return func(core *groupCore) {
		core.unhandledPanicCB = cb
	}
}

func UnhandledErrorCB(cb func(error)) GroupOption {
	return func(core *groupCore) {
		core.unhandledErrorCB = cb
	}
}

func Size(size int) GroupOption {
	return func(core *groupCore) {
		core.sg.SetSize(size)
	}
}

// SetOnetimeHandling sets the [GroupConfig.OnetimeHandling] value to true,
// or to the set value provided, if any.
func SetOnetimeHandling(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.options.SetOnetimeHandlingTo(s)
	}
}

// SetNeverCancelCBCtx sets the [GroupConfig.NeverCancelCBCtx] value to true,
// or to the set value provided, if any.
func SetNeverCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}

		// only allow set to be effective if the [GroupConfig.FailuresCancelCBCtx]
		// or [GroupConfig.FailuresCancelGroup] flags aren't set.
		if s && !core.options.IsFailuresCancelCBCtx() && !core.options.IsFailuresCancelGroup() {
			core.options.SetNeverCancelCBCtxTo(true)
		} else if !s {
			core.options.SetNeverCancelCBCtxTo(false)
		}
	}
}

// SetFailuresCancelCBCtx sets the [GroupConfig.FailuresCancelCBCtx] value to true,
// or to the set value provided, if any.
func SetFailuresCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}

		if s {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetNeverCancelCBCtxTo(false)
			core.options.SetFailuresCancelCBCtxTo(true)
		} else {
			core.ctx, core.cancel = nil, nil
			core.options.SetFailuresCancelCBCtxTo(false)
		}
	}
}

// SetFailuresCancelGroup sets the [GroupConfig.FailuresCancelGroup] value to true,
// or to the set value provided, if any.
func SetFailuresCancelGroup(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}

		if s {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetNeverCancelCBCtxTo(false)
			core.options.SetFailuresCancelGroupTo(true)
		} else {
			core.ctx, core.cancel = nil, nil
			core.options.SetFailuresCancelGroupTo(false)
		}
	}
}

// SetNoNilCtxDoneChan sets the [GroupConfig.NoNilCtxDoneChan] value to true,
// or to the set value provided, if any.
func SetNoNilCtxDoneChan(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.options.SetNoNilCtxDoneChanTo(s)
	}
}

// SetNoWaitingBusyGroup sets the [GroupConfig.NoWaitingBusyGroup] value to true,
// or to the set value provided, if any.
func SetNoWaitingBusyGroup(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.options.SetNoWaitingBusyGroupTo(s)
	}
}

// SetSaveAllGroupResult sets the [GroupConfig.SaveAllGroupResults] value to true,
// or to the set value provided, if any.
func SetSaveAllGroupResult(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.options.SetSaveAllGroupResultsTo(s)
	}
}

type GroupConfig struct {
	UnhandledPanicCB func(v any)
	UnhandledErrorCB func(err error)

	// Size is the number of goroutines which this group is allowed to run.
	// This includes goroutines created for both, constructor calls(Go, GoCtxRes, etc.),
	// and follow calls([Promise.Then], [Promise.Catch], etc.).
	// If it's 0 or less, then the group size is unlimited.
	Size int

	// OnetimeHandling enforces that the [Result] value returned from any
	// callback is only used one-time (via [Promise.Res] or as an argument
	// to any other callbacks).
	// This will cause any further attempt to use that [Result] value to
	// return an [Error] [Result] with the [ErrPromiseConsumed] error.
	OnetimeHandling bool

	// NeverCancelCBCtx will result in passing a never canceled [context.Context]
	// value to all callbacks.
	//
	// If [GroupConfig.FailuresCancelCBCtx] or [GroupConfig.FailuresCancelGroup]
	// is true, this will be ignored.
	//
	// The default behavior is always canceling the callback's [context.Context]
	// value when its respective [Promise] returns, that is when the callback
	// function itself returns.
	NeverCancelCBCtx bool

	// FailuresCancelCBCtx will result in canceling all [context.Context] values
	// passed to all callbacks, once any callback returns an [Error] or a [Panic]
	// [Result] that's not handled.
	//
	// Handling an [Error] [Result] is done via one of [Promise.Catch], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.Recover], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	//
	// The default behavior is never canceling a callback's [context.Context] value
	// on any unhandled [Error] or [Panic] [Result] caused by other callbacks, and
	// only canceling it once the respective [Promise] returns.
	FailuresCancelCBCtx bool

	// FailuresCancelGroup will result in cancelling the [Group] once any callback
	// returns an [Error] or a [Panic] [Result] that's not handled.
	//
	// Cancelling the [Group] will cause new calls made to all the [Group.Go] methods,
	// the [Group.Delay], [Group.Chan], and [Group.Ctx] methods to return a [Promise]
	// with an [Error] [Result] with the [ErrGroupCanceled] error.
	//
	// Handling an [Error] [Result] is done via one of [Promise.Catch], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.Recover], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	//
	// The default behavior is always allowing new calls that return a [Promise] value,
	// unless the [Group] is closed by one of the wait methods ([Group.Wait],
	// [Group.AllWaitRes], [Group.AnyWaitRes], [Group.JoinRes]).
	//
	// This also sets the [GroupConfig.FailuresCancelCBCtx] flag.
	//
	// Inspired from: https://github.com/golang/go/issues/54045
	FailuresCancelGroup bool

	// NoNilCtxDoneChan causes new calls to [Group.Ctx] to return an [Error] [Result]
	// with the [ErrPromiseNilCtxDone] err, when the [context.Context] value passed
	// has a nil Done channel ([context.Context.Done]).
	// Otherwise, it will allow the creation of the [Promise], which will be a never
	// resolved promise, causing all follow calls to be blocked as well.
	NoNilCtxDoneChan bool

	// NoWaitingBusyGroup causes new calls to all the [Group.Go] methods,
	// the [Group.Delay], [Group.Chan], and [Group.Ctx] methods to return a [Promise]
	// with an [Error] [Result] with the [ErrGroupBusy] error, when the [Group] is
	// currently handling promises that equals the [Size] value (if set).
	// Otherwise, it will wait until one of the running promises returns and
	// there's a place for the new promise.
	NoWaitingBusyGroup bool

	// SaveAllGroupResults causes the [Group.AllWaitRes], [Group.AnyWaitRes]
	// and [Group.JoinRes] to return all [Result] values that got returned
	// from the [Promise] values created from this [Group] since it got created.
	//
	// By default, we save a single Result value, the first Result.
	SaveAllGroupResults bool
}
