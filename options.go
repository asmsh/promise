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

		if conf.FailuresCancelCBCtx {
			core.ctx, core.cancel = context.WithCancel(context.Background())
		}

		if conf.NeverCancelCBCtx && !conf.FailuresCancelCBCtx {
			core.neverCancelCBCtx = true
		}

		if conf.OnetimeHandling {
			core.onetimeHandling = true
		}

		if conf.NoNilCtxDoneChan {
			core.noNilCtxDoneChan = true
		}

		if conf.NoWaitingBusyGroup {
			core.noWaitingBusyGroup = true
		}

		if conf.SaveAllGroupResults {
			core.saveAllGroupResults = true
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

func SetFailuresCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		if s {
			core.ctx, core.cancel = context.WithCancel(context.Background())
			core.neverCancelCBCtx = false
		} else {
			core.ctx, core.cancel = nil, nil
		}
	}
}

func SetNeverCancelCBCtx(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		if s && core.ctx == nil {
			core.neverCancelCBCtx = true
		} else if !s {
			core.neverCancelCBCtx = false
		}
	}
}

func SetOnetimeHandling(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.onetimeHandling = s
	}
}

func SetNoNilCtxDoneChan(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.noNilCtxDoneChan = s
	}
}

func SetNoWaitingBusyGroup(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.noWaitingBusyGroup = s
	}
}

func SetSaveAllGroupResult(set ...bool) GroupOption {
	return func(core *groupCore) {
		s := true
		if len(set) > 0 {
			s = set[0]
		}
		core.saveAllGroupResults = s
	}
}

type GroupConfig struct {
	UnhandledPanicCB func(v any)
	UnhandledErrorCB func(err error)

	// Size is the number of goroutines which this group is allowed to run.
	// This includes goroutines created for both, constructor calls(Go, GoRes, etc.),
	// and follow calls([Promise.Then], [Promise.Catch], etc.).
	// If it's 0 or less, then the group size is unlimited.
	Size int

	// FailuresCancelCBCtx, if true, will result in canceling all [context.Context] values
	// passed to all callbacks, once any callback returns an error or cause a panic that's
	// not caught or recovered, through [Promise.Catch] or [Promise.Recover], respectively.
	// The default behavior is never canceling a callback's [context.Context] value on any
	// failures from other callbacks.
	FailuresCancelCBCtx bool

	// NeverCancelCBCtx, if true, will result in passing a never canceled [context.Context]
	// value to all callbacks.
	// If [GroupConfig.FailuresCancelCBCtx] is true, this will be set to false.
	// The default behavior is always canceling the callback's [context.Context] value
	// when its [Promise] resolves.
	NeverCancelCBCtx bool

	// OnetimeHandling, if true, will enforce that the [Result] value returned from
	// any callback is only used one-time (via [Promise.Res] or as an argument to
	// any other callbacks).
	// And this will cause any further attempt to use that [Result] value to return
	// a [Error] [Result] with the [ErrPromiseConsumed] error.
	OnetimeHandling bool

	// NoNilCtxDoneChan, if true, will cause new calls to [Group.Ctx] to return a
	// [Error] [Result] with [ErrPromiseNilCtxDone], when the [context.Context]
	// value passed has a nil Done channel ([context.Context.Done]).
	// Otherwise, it will allow the creation of the [Promise], which will be a never
	// resolved (blocked) promise, causing all follow promises to be blocked as well.
	NoNilCtxDoneChan bool

	// NoWaitingBusyGroup, if true, will cause new calls to [Group.Chan], [Group.Delay],
	// and all [Group.Go] functions, to return a [Error] [Result] with [ErrGroupBusy]
	// synchronously, when the [Group] is currently handling promises that equals the [Size]
	// value (if set).
	// Otherwise, it will wait until there's a place for the new promise.
	NoWaitingBusyGroup bool

	// SaveAllGroupResults causes the [Group.AllWaitRes], [Group.AnyWaitRes]
	// and [Group.JoinRes] to return all [Result] values that got returned
	// from the [Promise] values created from this [Group] since it got created.
	//
	// By default, we save a single Result value, the first Result.
	SaveAllGroupResults bool
}
