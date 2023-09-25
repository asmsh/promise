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

// PromStatus holds the value that defines and represents the behavior and the
// status of the promise.
// It's read and written/updated atomically.
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
	chainModeNone   uint32 = iota << 4
	chainModeWait   uint32 = iota << 4
	chainModeRead   uint32 = iota << 4
	chainModeFollow uint32 = iota << 4

	// chain flags, and calls flags, using 4 bits
	_ = 1 << 6 // reserved
	_ = 1 << 7 // reserved
	_ = 1 << 8 // reserved
	_ = 1 << 9 // reserved

	// chainModeBitsSetMask and chainModeBitsClrMask are &-ed with the status
	// to get the chain value and clear the chain value, respectively.
	chainModeBitsSetMask = chainModeFollow
	chainModeBitsClrMask = ^chainModeBitsSetMask
)

// the fate's related values and constants, using 2 bits(the [11th : 12th] bits)
const (
	// starting with a shift amount of 10, which is the number of bits used by
	// previous sections.

	// fate modes, using 2 bits
	fateUnresolved uint32 = iota << 10
	fateResolving  uint32 = iota << 10
	fateResolved   uint32 = iota << 10
	fateHandled    uint32 = iota << 10

	// fateBitsSetMask and fateBitsCLrMask are &-ed with the status to get
	// the fate value and clear the fate value, respectively.
	fateBitsSetMask = fateHandled
	fateBitsCLrMask = ^fateBitsSetMask
)

// the state's related values and constants, using 2 bits(the [13th : 14th] bits)
const (
	// starting with a shift amount of 12, which is the number of bits used by
	// previous sections.

	// state modes, using 2 bits
	statePending   uint32 = iota << 12
	stateFulfilled uint32 = iota << 12
	stateRejected  uint32 = iota << 12
	statePanicked  uint32 = iota << 12

	// stateBitsSetMask and stateBitsCLrMask are &-ed with the status to get
	// the state value and clear the state value, respectively.
	stateBitsSetMask = statePanicked
	stateBitsCLrMask = ^stateBitsSetMask
)

// the flags' related values and constants, using 6 bits(the [15th : 20th] bits)
const (
	// starting with a shift amount of 14, which is the number of bits used by
	// previous sections.

	// promise chain types...
	FlagsTypeOnce  uint32 = 1 << (iota + 14)
	FlagsTypeTimed uint32 = 1 << (iota + 14)
	_                     = 1 << (iota + 14) // reserved
	_                     = 1 << (iota + 14) // reserved

	// promise features...
	FlagsIsNotSafe  uint32 = 1 << (iota + 14)
	FlagsIsExternal uint32 = 1 << (iota + 14)
	_                      = 1 << (iota + 14) // reserved
	_                      = 1 << (iota + 14) // reserved

	// 255 = 1111_1111 (the 8 flags above)
	flagsBitsSetMask uint32 = 255 << 14
	flagsBitsClrMask        = ^flagsBitsSetMask
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
		panic("promise: internal: unexpected status change")
	}
}

// Load returns the current status value, if it's not being updated right now,
// and if it's, it waits until it's updated then return the value.
func (s *PromStatus) Load() (currentStatus uint32) {
	// read the current status value, and return it, as long as the
	// read value is not the locked status, otherwise, wait until the
	// read value becomes different than the locked status.
	cs := load((*uint32)(s))
	for cs == lockAcquired {
		cs = load((*uint32)(s))
	}
	return cs
}

// RegRead declares that there's a read call registered on this promise,
// like a 'Finally', 'Wait', or 'asyncRead' calls.
func (s *PromStatus) RegRead() (firstRead bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain mode to read, only if the chain mode is none or wait
	if ns&chainModeBitsSetMask < chainModeRead {
		ns &= chainModeBitsClrMask // clear the chain mode bits
		ns |= chainModeRead        // set the chain mode to read
		firstRead = true           // this is the first set to read
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstRead, ns
}

// RegWait declares that there's a wait call registered on this promise,
// like a 'Wait', or 'asyncWait' calls.
func (s *PromStatus) RegWait() (firstWait bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain mode to wait, only if the chain mode is none
	if ns&chainModeBitsSetMask < chainModeWait {
		ns &= chainModeBitsClrMask // clear the chain mode bits
		ns |= chainModeWait        // set the chain mode to wait
		firstWait = true           // this is the first set to wait
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstWait, ns
}

// RegFollow declares that there's a follow call registered on this promise,
// like a 'Then', 'Catch', or 'Recover' calls.
func (s *PromStatus) RegFollow() (firstFollow bool, status uint32) {
	// read the current status value, and acquire the update lock
	cs := s.readAndAcquireLock()
	// create a new status value from the current one
	ns := cs

	// set the chain mode to follow, only if the chain mode is none, temp, or wait
	if ns&chainModeBitsSetMask < chainModeFollow {
		ns &= chainModeBitsClrMask // clear the chain mode bits
		ns |= chainModeFollow      // set the chain mode to follow
		firstFollow = true         // this is the first set to follow
	}

	// save the new status value, and release the update lock
	s.saveAndReleaseLock(ns)
	return firstFollow, ns
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
// Deprecated:
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

// IsChainEmpty returns true if the chain mode is 'chainModeNone'
func IsChainEmpty(status uint32) bool {
	return status&chainModeBitsSetMask == chainModeNone
}

// IsChainAtLeastRead returns true if the chain mode is either 'chainModeRead'
// or 'chainModeFollow'.
func IsChainAtLeastRead(status uint32) bool {
	return status&chainModeBitsSetMask >= chainModeRead
}

func IsChainModeWait(status uint32) bool {
	return status&chainModeBitsSetMask == chainModeWait
}

func IsChainModeRead(status uint32) bool {
	return status&chainModeBitsSetMask == chainModeRead
}

func IsChainModeFollow(status uint32) bool {
	return status&chainModeBitsSetMask == chainModeFollow
}

func IsFateUnresolved(status uint32) bool {
	return status&fateBitsSetMask == fateUnresolved
}

func IsFateResolving(status uint32) bool {
	return status&fateBitsSetMask == fateResolving
}

func IsFateResolved(status uint32) bool {
	return status&fateBitsSetMask == fateResolved
}

func IsFateHandled(status uint32) bool {
	return status&fateBitsSetMask == fateHandled
}

func IsStatePending(status uint32) bool {
	return status&stateBitsSetMask == statePending
}

func IsStateFulfilled(status uint32) bool {
	return status&stateBitsSetMask == stateFulfilled
}

func IsStateRejected(status uint32) bool {
	return status&stateBitsSetMask == stateRejected
}

func IsStatePanicked(status uint32) bool {
	return status&stateBitsSetMask == statePanicked
}

func IsFlagsOnce(status uint32) bool {
	return status&FlagsTypeOnce == FlagsTypeOnce
}

func IsFlagsTimed(status uint32) bool {
	return status&FlagsTypeTimed == FlagsTypeTimed
}

func IsFlagsNotSafe(status uint32) bool {
	return status&FlagsIsNotSafe == FlagsIsNotSafe
}

func IsFlagsExternal(status uint32) bool {
	return status&FlagsIsExternal == FlagsIsExternal
}

func NewFromFlags(status uint32) PromStatus {
	return PromStatus(status & flagsBitsSetMask)
}
