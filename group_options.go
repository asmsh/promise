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

// DefaultGroupConfig is the default [GroupConfig] value, equivalent to
// a zero-value [Group].
var DefaultGroupConfig = GroupConfig{}

// GroupOption configures a [Group].
type GroupOption func(core *groupCore)

// ApplyConfig applies all the settings from the provided [GroupConfig]
// value, conf, to the [Group].
func ApplyConfig(conf GroupConfig) GroupOption {
	return func(core *groupCore) {
		Size(conf.Size)(core)

		if conf.UnhandledPanicCB != nil {
			UnhandledPanicCB(conf.UnhandledPanicCB)(core)
		}

		if conf.UnhandledErrorCB != nil {
			UnhandledErrorCB(conf.UnhandledErrorCB)(core)
		}

		if conf.OnetimeHandling {
			OnetimeHandling()(core)
		}

		// Apply the cancel flags before NeverCancelCBCtx, because
		// NeverCancelCBCtx checks if they're already set on the core.
		if conf.FailuresCancelCBCtx {
			FailuresCancelCBCtx()(core)
		}

		if conf.FailuresCancelGroup {
			FailuresCancelGroup()(core)
		}

		if conf.NeverCancelCBCtx {
			NeverCancelCBCtx()(core)
		}

		if conf.NoNilCtxDoneChan {
			NoNilCtxDoneChan()(core)
		}

		if conf.NoWaitingBusyGroup {
			NoWaitingBusyGroup()(core)
		}

		if conf.SaveAllGroupResults {
			SaveAllGroupResults()(core)
		}
	}
}

// UnhandledPanicCB sets the [GroupConfig.UnhandledPanicCB] to the provided cb value.
func UnhandledPanicCB(cb func(any)) GroupOption {
	return func(core *groupCore) {
		core.unhandledPanicCB = cb
	}
}

// UnhandledErrorCB sets the [GroupConfig.UnhandledErrorCB] to the provided cb value.
func UnhandledErrorCB(cb func(error)) GroupOption {
	return func(core *groupCore) {
		core.unhandledErrorCB = cb
	}
}

// Size sets the [GroupConfig.Size] to the provided size value.
func Size(size int) GroupOption {
	return func(core *groupCore) {
		core.sg.SetSize(size)
	}
}

// OnetimeHandling sets the [GroupConfig.OnetimeHandling] to true,
// or to the set value provided, if any.
func OnetimeHandling(set ...bool) GroupOption {
	return func(core *groupCore) {
		core.options.SetOnetimeHandlingTo(getEffSet(set...))
	}
}

// FailuresCancelCBCtx sets the [GroupConfig.FailuresCancelCBCtx] to true,
// or to the set value provided, if any.
func FailuresCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		if getEffSet(set...) {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetNeverCancelCBCtxTo(false)
			core.options.SetFailuresCancelCBCtxTo(true)
		} else {
			core.ctx, core.cancel = nil, nil
			core.options.SetFailuresCancelCBCtxTo(false)
		}
	}
}

// FailuresCancelGroup sets the [GroupConfig.FailuresCancelGroup] to true,
// or to the set value provided, if any.
func FailuresCancelGroup(set ...bool) GroupOption {
	return func(core *groupCore) {
		if getEffSet(set...) {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.options.SetNeverCancelCBCtxTo(false)
			core.options.SetFailuresCancelGroupTo(true)
		} else {
			core.ctx, core.cancel = nil, nil
			core.options.SetFailuresCancelGroupTo(false)
		}
	}
}

// NeverCancelCBCtx sets the [GroupConfig.NeverCancelCBCtx] to true,
// or to the set value provided, if any.
func NeverCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := getEffSet(set...)

		// only allow set to be effective if the [GroupConfig.FailuresCancelCBCtx]
		// or [GroupConfig.FailuresCancelGroup] flags aren't set.
		if s && !core.options.IsFailuresCancelCBCtx() && !core.options.IsFailuresCancelGroup() {
			core.options.SetNeverCancelCBCtxTo(true)
		} else if !s {
			core.options.SetNeverCancelCBCtxTo(false)
		}
	}
}

// NoNilCtxDoneChan sets the [GroupConfig.NoNilCtxDoneChan] to true,
// or to the set value provided, if any.
func NoNilCtxDoneChan(set ...bool) GroupOption {
	return func(core *groupCore) {
		core.options.SetNoNilCtxDoneChanTo(getEffSet(set...))
	}
}

// NoWaitingBusyGroup sets the [GroupConfig.NoWaitingBusyGroup] to true,
// or to the set value provided, if any.
func NoWaitingBusyGroup(set ...bool) GroupOption {
	return func(core *groupCore) {
		core.options.SetNoWaitingBusyGroupTo(getEffSet(set...))
	}
}

// SaveAllGroupResults sets the [GroupConfig.SaveAllGroupResults] to true,
// or to the set value provided, if any.
func SaveAllGroupResults(set ...bool) GroupOption {
	return func(core *groupCore) {
		core.options.SetSaveAllGroupResultsTo(getEffSet(set...))
	}
}

// getEffSet returns the effective set value, which is the first provided
// value, if any, or true otherwise.
func getEffSet(set ...bool) bool {
	s := true
	if len(set) > 0 {
		s = set[0]
	}
	return s
}

// GroupConfig holds the configuration settings for a [Group].
// It can be applied to a [Group] by passing [ApplyConfig] to [NewGroup],
// or by using the individual option functions directly.
type GroupConfig struct {
	// Size is the number of goroutines that this group is allowed to run.
	// This includes goroutines created for both, constructor calls (Go, GoCtxRes, etc.),
	// and follow calls ([Promise.Follow], [Promise.Callback], etc.).
	// If it's 0 or less, then the group size is unlimited.
	//
	// If the group is full, then new calls might block or return the [ErrGroupBusy]
	// error, based on the [GroupConfig.NoWaitingBusyGroup] flag.
	Size int

	// UnhandledPanicCB is called with the panic value once any [Promise] in
	// this [Group] resolves to [Panic] and it's not handled by any follow
	// calls, like [Promise.Follow] or [Promise.WaitRes].
	UnhandledPanicCB func(v any)

	// UnhandledErrorCB is called with the error value once any [Promise] in
	// this [Group] resolves to [Error] and it's not handled by any follow
	// calls, like [Promise.Follow] or [Promise.WaitRes].
	UnhandledErrorCB func(err error)

	// OnetimeHandling enforces that the [Result] value returned from any
	// callback is only used one-time (via [Promise.WaitRes] or as an argument
	// to any other callback that accepts a [Result] value).
	// This will cause any further attempt to use that [Result] value to
	// return an [Error] [Result] with the [ErrPromiseConsumed] error.
	OnetimeHandling bool

	// FailuresCancelCBCtx will result in canceling all [context.Context] values
	// passed to all callbacks, once any callback returns an [Error] or a [Panic]
	// [Result] that's not handled.
	//
	// Handling an [Error] [Result] is done via one of [Promise.Follow], [Promise.WaitRes],
	// or [Promise.Delay].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.Follow], [Promise.WaitRes],
	// or [Promise.Delay].
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
	// Handling an [Error] [Result] is done via one of [Promise.Follow], [Promise.WaitRes],
	// or [Promise.Delay].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.Follow], [Promise.WaitRes],
	// or [Promise.Delay].
	//
	// The default behavior is always allowing new calls that return a [Promise] value,
	// unless the [Group] is closed by one of the wait methods ([Group.Wait],
	// [Group.AllWaitRes], [Group.AnyWaitRes], [Group.JoinRes]).
	//
	// This also sets the [GroupConfig.FailuresCancelCBCtx] flag.
	//
	// Inspired from: https://github.com/golang/go/issues/54045
	FailuresCancelGroup bool

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

	// NoNilCtxDoneChan causes new calls to [Group.Ctx] to return an [Error] [Result]
	// with the [ErrNilCtxDone] err, when the [context.Context] value passed
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
