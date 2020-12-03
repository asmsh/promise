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

package status

import (
	"runtime"
	"sync/atomic"
)

var (
	cas  = atomic.CompareAndSwapUint32
	load = atomic.LoadUint32
	swap = atomic.SwapUint32
)

// PromStatus holds the value that defines and represents the behaviour and the
// status of the promise.
// It's read and written/updated atomically.
//
// The value is split into 5 sections, flags, state, fate, chain, and lock, as
// follows, starting from the right:
// - The lock section takes 4 bits.
// - The chain section takes 6 bits.
// - The fate section takes 2 bits.
// - The state section takes 2 bits.
// - The flags section takes 8 bits.
//
// Description of the section:
// * the lock section.
//   = although it's named 'lock', it doesn't use any Mutexes.
//   = the lock is implemented through atomic writing, reading, and updating of
//     the status value.
//   = although the implementation is lock-free(in the sense of Mutexes), it's
//     not wait-free.
//   = the lock logic used is just a way to tell new comers(that want to update
//     the status) that: "the value is currently being updated by some previous
//     update call so wait here until it finish, then you can get your chance
//     to update the status too".
//   = currently, for a large number of concurrent calls, the lock logic might
//     starve some calls which are waiting to acquire the lock.
//   = currently, there's no fairness in the lock logic, an old call waiting to
//     acquire the lock might wait longer than newer calls.
//   = the whole waiting behaviour is passed to the 'go scheduler'(through a call
//     to runtime.Gosched) to decide which goroutine should run now(and hence
//     acquire the lock first).
//   = despite the fact of the lock fairness problem, the lock will be acquired
//     for only a small period of time by any call, because the operations done
//     while the lock is acquired are very basic(add, and, or, assign, compare,
//     and conditions).
//   = based on the expected small acquire time, when no concurrent acquire calls
//     are executed continuously all the time, no starving acquire calls will run
//     forever.
//   = from testing(and benchmarks), the current lock implementation does just
//     okay, it does the job, without being a bottleneck.
//   = considering the starving issue, it's rather odd to have such big number
//     of concurrent calls on a promise in real examples, so the issue should
//     not be visible at all.
//
// * the chain section describes the promise chain and the chained calls.
//   = 3 mutually exclusive possible modes, represented by 2 bits:
//     - none: no calls are chained or has ever been chained to this promise.
//     - wait: the chain has a call that can read only fulfilled promise results,
//     - read: the chain has a call that can read fulfilled and rejected promise
//             results, such call can prevent UnCaughtErr panics if the promise
//             is about to be rejected.
//     - follow: the chain has a call that can read all types of promise results,
//               such call can prevent panics if the promise is about to be
//               rejected or panicked.
//   = 1 flag(with 3 more reserved):
//     - calledFinally: whether a 'Finally' callback has been called on this
//                      promise or not.
//   = wait calls are 'Wait', 'WaitUntil 'GetRes', and 'GetResUntil'.
//   = read calls are 'Finally', and 'asyncRead'.
//   = follow calls are 'Then', 'Catch', 'Recover', and 'asyncFollow'.
//   = the chain modes decides whether a panic should happen when the promise
//     is about to be rejected or panicked(uncaught error or un-recovered panic).
//   = not all methods have to updated the chain mode with their corresponding
//     chain mode value, only the ones which their logic depends on the panic
//     behaviour.
//   = the chain value is written/updated as long as the fate is unresolved.
//   = the chain value is read just before the promise is resolved.
//   = each chain value covers the logic of the previous values.
//   = the 'wait' chain mode is defined but not used, as currently, no specific
//     logic needs to know about it.
//
// * the fate section describes the fate of the promise.
//   = 4 mutually exclusive possible values, represented by 2 bits:
//     - unresolved: the promise's callback func didn't finish its work, or its
//                   wait chan hasn't been closed nor received the result.
//     - resolving: the promise's result is being passed to the only one resolving
//                  call.
//                  it's an internal fate that denotes that other calls must wait.
//                  it's considered an equivalent to the unresolved fate.
//     - resolved: the promise's final state is know, whether it's still pending,
//                 or is settled to some other state.
//     - handled: the promise is resolved and its result is being passed to a
//                read call, or to any follow call.
//   = the fate value is updated twice, firstly, when resolving, along with the
//     state value(after reading the chain value), and secondly, when executing
//     a follow method's callback or returning from an await call.
//   = a promise which its fate is unresolved, its state must be pending.
//   = a promise which its fate is resolved, and its state is pending, its fate
//     will never be handled.
//   = at transiting from the unresolved fate to the resolved fate, the chain
//     value is read and a panic will occur if the promise is not followed,
//     and its state is panicked or rejected(when running in the safe mode).
//
// * the state section describes the state of this promise.
//   = 4 mutually exclusive possible values, represented by 2 bits:
//     - pending: the promise's callback func didn't finish its work, or its
//                wait chan hasn't been closed nor received the result, yet.
//     - fulfilled: the promise's callback func finished its work and returned
//                  the result, the wait chan received the result, or the wait
//                  chan has been closed.
//                  in any case, where there's a result, the last value must be
//                  either not an error value, or it's a nil error value.
//     - rejected: the promise's callback func finished its work and returned
//                 the result, or the wait chan received the result.
//                 in any case, the last value must be a non-nil error value.
//     - panicked: the promise's callback func produced a panic which it didn't
//                 recover from.
//   = the state value is written once, after reading the chain value.
//   = a promise which its state is pending, its fate must be either unresolved
//     or resolved.
//   = a promise which its state is fulfilled, rejected, or panicked, its fate
//     must be either resolved or handled.
//
// * the flags section describes the behaviour of the promise, it consists of
//   two sub-sections, types, and features.
//   = the types sub-section holds info about the type of the promise, and has
//     two values(with 2 more reserved):
//     - once: whether the promise is a OncePromise or not.
//     - timed: whether the promise is a TimedPromise or not.
//   = the features sub-section holds info about supported features by the promise,
//     and has two values(with 2 more reserved):
//     - notSafe: whether the promise is running in the non-safe mode or not.
//     - external: whether the wait chan is created externally or internally.
//   = the flags are written once, at creation, and never updated.
//
type PromStatus uint32

// the lock's related values and constants, using 4 bits(the [1st : 4th] bits)
const (
	// lockAcquired is the value of the status when some update call is
	// running(reg, inc, or dec methods).
	lockAcquired uint32 = 1 << iota
	_                   // reserved
	_                   // reserved
	_                   // reserved
)

// the chain's related values and constants, using 6 bits(the [5th : 10th] bits)
const (
	// starting with a shift amount of 4, which is the number of bits used by
	// previous sections.

	// chain modes, using 2 bits
	chainModeNone uint32 = iota << 4
	chainModeWait
	chainModeRead
	chainModeFollow

	// chain flags, and calls flags, using 4 bits
	chainCalledFinally = 1 << 6
	_                  = 1 << 7 // reserved
	_                  = 1 << 8 // reserved
	_                  = 1 << 9 // reserved

	// chainModeBitsSetMask and chainModeBitsClrMask are &-ed with the status
	// to get the chain value and clear the chain value, respectively.
	chainModeBitsSetMask = chainModeFollow
	chainModeBitsClrMask = ^chainModeBitsSetMask
)

// the fate's related values and constants, using 2 bits(the [11th : 12th] bits)
const (
	// starting with a shift amount of 10, which is the number of bits used by
	// previous sections.

	fateUnresolved uint32 = iota << 10
	fateResolving
	fateResolved
	fateHandled

	// fateBitsSetMask and fateBitsCLrMask are &-ed with the status to get
	// the fate value and clear the fate value, respectively.
	fateBitsSetMask = fateHandled
	fateBitsCLrMask = ^fateBitsSetMask
)

// the state's related values and constants, using 2 bits(the [13th : 14th] bits)
const (
	// starting with a shift amount of 12, which is the number of bits used by
	// previous sections.

	statePending uint32 = iota << 12
	stateFulfilled
	stateRejected
	statePanicked

	// stateBitsSetMask and stateBitsCLrMask are &-ed with the status to get
	// the state value and clear the state value, respectively.
	stateBitsSetMask = statePanicked
	stateBitsCLrMask = ^stateBitsSetMask
)

// the flags' related values and constants, using 6 bits(the [15th : 20th] bits)
const (
	// starting with a shift amount of 14, which is the number of bits used by
	// previous sections.

	// promise types
	FlagsTypeOnce uint32 = 1 << (iota + 14)
	FlagsTypeTimed
	_ // reserved
	_ // reserved

	// promise features
	FlagsIsNotSafe
	FlagsIsExternal
	_ // reserved
	_ // reserved
)

func (s *PromStatus) readAndAcquireLock() (currentStatus uint32) {
	// read the current status value, and acquire the update lock,
	// by checking if there's any other, previous, update call is
	// still processing, and wait for it to finish.
	cs := swap((*uint32)(s), lockAcquired)
	for cs == lockAcquired {
		// don't actively wait for concurrent update calls, instead,
		// tell the go scheduler to run other goroutines(including the
		// one which has the lock) instead of the current(waiting) one.
		runtime.Gosched()
		cs = swap((*uint32)(s), lockAcquired)
	}
	// at this point, the value of the current status, cs, here is
	// only available to this method and its caller.
	return cs
}

func (s *PromStatus) saveAndReleaseLock(newStatus uint32) {
	// save the new status value, and release the update lock
	if !cas((*uint32)(s), lockAcquired, newStatus) {
		// panic if the status value has been changed unexpectedly
		panic("promise: internal: the promise' status has been changed unexpectedly")
	}
}

// Read returns the current status value, if it's not being updated rightnow,
// and if it's, it waits until it's updated then return the value.
func (s *PromStatus) Read() (currentStatus uint32) {
	// read the current status value, and return it, as long as the
	// read value is not the locked status, otherwise, wait until the
	// read value becomes different than the locked status.
	cs := load((*uint32)(s))
	for cs == lockAcquired {
		cs = load((*uint32)(s))
	}
	return cs
}

// RegFollow declares that there's a follow call registered on this promise,
// like a 'Then', 'Catch', or 'Recover' calls.
func (s *PromStatus) RegFollow() (firstFollow bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain mode to follow, only if the fate is unresolved or
	// resolving, and the chain mode is either none, temp, or wait.
	if ns&fateBitsSetMask < fateResolved {
		if ns&chainModeBitsSetMask < chainModeFollow {
			ns &= chainModeBitsClrMask // clear the chain mode bits
			ns |= chainModeFollow      // set the chain mode to follow
			firstFollow = true         // this is the first set to follow
		}
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstFollow, ns
}

// RegRead declares that there's a read call registered on this promise,
// like a 'Finally', or 'asyncRead' calls.
func (s *PromStatus) RegRead() (firstRead bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain mode to read, only if the fate is unresolved or
	// resolving, and the chain mode is none.
	if ns&fateBitsSetMask < fateResolved {
		if ns&chainModeBitsSetMask < chainModeRead {
			ns &= chainModeBitsClrMask // clear the chain mode bits
			ns |= chainModeRead        // set the chain mode to read
			firstRead = true           // this is the first set to read
		}
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstRead, ns
}

// SetCalledFinally declares that 'Finally' has been called on this promise.
// 'Finally' got its own flag, cause it can't set the fate to 'Handled', so
// it need another approach to detect calls, for the once implementation.
func (s *PromStatus) SetCalledFinally() (firstCall bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain calls flags at any state & fate
	if ns&chainCalledFinally != chainCalledFinally {
		ns |= chainCalledFinally // set the corresponding flag
		firstCall = true         // this is the first time it's set
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstCall, ns
}

// SetResolving set the fate to Resolving, only if it's Unresolved.
// This method(and the Resolving fate) is unique to the resolving logic,
// and should be used only for that purpose.
func (s *PromStatus) SetResolving() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the fate to resolving, only if the fate is unresolved
	if ns&fateBitsSetMask == fateUnresolved {
		ns &= fateBitsCLrMask // clear the fate section
		ns |= fateResolving   // set the fate to resolving
		set = true            // this is the first set to resolving
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

// ClearResolving set the fate to Unresolved, only if it's Resolving.
// This method(and the Resolving fate) is unique to the resolving logic,
// and should be used only for that purpose.
// It's required for the JointPromise implementation.
func (s *PromStatus) ClearResolving() (cleared bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the fate to resolving, only if the fate is unresolved
	if ns&fateBitsSetMask == fateResolving {
		ns &= fateBitsCLrMask // clear the fate section
		ns |= fateUnresolved  // set the fate to resolving
		cleared = true        // this is the first set to resolving
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return cleared, ns
}

// SetPendingResolved is required for the TimedPromise
func (s *PromStatus) SetPendingResolved() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the fate to resolved without changing the state(because it will
	// be pending), only if the fate is unresolved or resolving.
	if ns&fateBitsSetMask < fateResolved {
		ns &= fateBitsCLrMask // clear the fate section
		ns |= fateResolved    // set the fate to resolved
		set = true            // this is the first set to resolved
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

func (s *PromStatus) SetFulfilledResolved() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the state to fulfilled and the fate to resolved, only if the fate
	// is unresolved or resolving.
	if ns&fateBitsSetMask < fateResolved {
		ns &= stateBitsCLrMask // clear the state section
		ns &= fateBitsCLrMask  // clear the fate section
		ns |= stateFulfilled   // set the state to fulfilled
		ns |= fateResolved     // set the fate to resolved
		set = true
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

// SetFulfilledResolvedSync should be used only from the 'fulfillSync' method.
// it updates the status value directly, as in the 'fulfillSync' method it's
// guaranteed that the promise will be accessible from this goroutine only,
// because the promise hasn't been returned to the caller yet.
func (s *PromStatus) SetFulfilledResolvedSync() (status uint32) {
	ns := uint32(0)
	ns |= stateFulfilled // set the state to fulfilled
	ns |= fateResolved   // set the fate to resolved
	*s = PromStatus(ns)  // update the status value
	return ns
}

func (s *PromStatus) SetRejectedResolved() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the state to rejected and the fate to resolved, only if the fate
	// is unresolved or resolving.
	if ns&fateBitsSetMask < fateResolved {
		ns &= stateBitsCLrMask // clear the state section
		ns &= fateBitsCLrMask  // clear the fate section
		ns |= stateRejected    // set the state to rejected
		ns |= fateResolved     // set the fate to resolved
		set = true
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

// SetRejectedResolvedSync should be used only from the 'rejectSync' method.
// it updates the status value directly, as in the 'rejectSync' method it's
// guaranteed that the promise will be accessible from this goroutine only,
// because the promise hasn't been returned to the caller yet.
func (s *PromStatus) SetRejectedResolvedSync() (status uint32) {
	ns := uint32(0)
	ns |= stateRejected // set the state to rejected
	ns |= fateResolved  // set the fate to resolved
	*s = PromStatus(ns) // update the status value
	return ns
}

func (s *PromStatus) SetPanickedResolved() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the state to panicked and the fate to resolved, only if the fate
	// is unresolved or resolving.
	if ns&fateBitsSetMask < fateResolved {
		ns &= stateBitsCLrMask // clear the state section
		ns &= fateBitsCLrMask  // clear the fate section
		ns |= statePanicked    // set the state to panicked
		ns |= fateResolved     // set the fate to resolved
		set = true
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

// SetPanickedResolvedSync should be used only from the 'panicSync' method.
// it updates the status value directly, as in the 'panicSync' method it's
// guaranteed that the promise will be accessible from this goroutine only,
// because the promise hasn't been returned to the caller yet.
func (s *PromStatus) SetPanickedResolvedSync() (status uint32) {
	ns := uint32(0)
	ns |= statePanicked // set the state to panicked
	ns |= fateResolved  // set the fate to resolved
	*s = PromStatus(ns) // update the status value
	return ns
}

func (s *PromStatus) SetHandled() (set bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// panic if the state is 'pending', or the fate is neither 'resolved' nor 'handled'
	if ns&stateBitsSetMask == statePending || ns&fateBitsSetMask < fateResolved {
		// release the lock, so if the panic is recovered, the status is unlocked
		s.saveAndReleaseLock(ns)
		panic("promise: internal: unexpected call to SetHandled")
	}

	// set the fate to handled, without changing the state, only if the fate
	// is not handled already.
	if ns&fateBitsSetMask < fateHandled {
		ns &= fateBitsCLrMask // clear the fate section
		ns |= fateHandled     // set the fate to handled
		set = true
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return set, ns
}

func IsChainModeFollow(status uint32) bool {
	chainMode := status & chainModeBitsSetMask
	return chainMode == chainModeFollow
}

// IsChainEmpty returns true if the chain mode is less than 'chainModeRead'
func IsChainEmpty(status uint32) bool {
	chainMode := status & chainModeBitsSetMask
	return chainMode < chainModeRead
}

func IsFateUnresolved(status uint32) bool {
	return status&fateBitsSetMask == fateUnresolved
}

func IsFateResolving(status uint32) bool {
	return status&fateBitsSetMask == fateResolving
}

func IsFateResolved(status uint32) bool {
	fate := status & fateBitsSetMask
	return fate == fateResolved
}

func IsFateHandled(status uint32) bool {
	fate := status & fateBitsSetMask
	return fate == fateHandled
}

func IsStatePending(status uint32) bool {
	state := status & stateBitsSetMask
	return state == statePending
}

func IsStateFulfilled(status uint32) bool {
	state := status & stateBitsSetMask
	return state == stateFulfilled
}

func IsStateRejected(status uint32) bool {
	state := status & stateBitsSetMask
	return state == stateRejected
}

func IsStatePanicked(status uint32) bool {
	state := status & stateBitsSetMask
	return state == statePanicked
}

func IsFlagsExternal(status uint32) bool {
	flags := status & FlagsIsExternal
	return flags == FlagsIsExternal
}

func IsFlagsNotSafe(status uint32) bool {
	flags := status & FlagsIsNotSafe
	return flags == FlagsIsNotSafe
}
