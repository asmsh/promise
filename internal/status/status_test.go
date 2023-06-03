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
	"testing"
)

// the benchmarks calls the SetFulfilledResolved method, as all methods
// use the same technique, but only sets different variables.

func BenchmarkPromStatus_Setters(b *testing.B) {
	s := PromStatus(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SetFulfilledResolved()
	}
}

func BenchmarkPromStatus_Setters_Parallel(b *testing.B) {
	b.Run("normal", func(b *testing.B) {
		s := PromStatus(0)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				s.SetFulfilledResolved()
			}
		})
	})
	b.Run("stressed", func(b *testing.B) {
		s := PromStatus(0)
		b.ReportAllocs()
		b.SetParallelism(100)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				s.SetFulfilledResolved()
			}
		})
	})
}

func BenchmarkPromStatus_Load(b *testing.B) {
	b.Run("unlocked status", func(b *testing.B) {
		s := PromStatus(0)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s.Load()
		}
	})
	b.Run("locked status", func(b *testing.B) {
		s := PromStatus(0)

		go func() {
			for {
				s.SetFulfilledResolved()
			}
		}()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s.Load()
		}
	})
}

// isChainOnlyChanged returns true if all other bits, other than chain's,
// are 0, otherwise it returns false.
func isChainOnlyChanged(s uint32) bool {
	sOtherBits := s & chainModeBitsClrMask
	return sOtherBits == 0
}

func TestPromStatus_Chain(t *testing.T) {
	s := PromStatus(0)

	if !IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: none")
	}

	// first set to read should succeed
	set, ns := s.RegRead()
	if !set {
		t.Errorf("PromStatus.Chain value 'read' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("RegRead returned unexpected status value")
	}
	if !IsChainModeRead(ns) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if !isChainOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than chain's, have changed, unexpectedly")
	}

	// second set to read should fail
	set, ns = s.RegRead()
	if set {
		t.Errorf("PromStatus.Chain value 'read' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("RegRead returned unexpected status value")
	}
	if !IsChainModeRead(ns) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if !isChainOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than chain's, have changed, unexpectedly")
	}

	// first set to Follow should succeed
	set, ns = s.RegFollow()
	if !set {
		t.Errorf("PromStatus.Chain value 'follow' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("RegFollow returned unexpected status value")
	}
	if !IsChainModeFollow(ns) {
		t.Errorf("unexpected PromStatus.Chain value, expected: follow")
	}
	if IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if !isChainOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than chain's, have changed, unexpectedly")
	}

	// second set to Follow should fail
	set, ns = s.RegFollow()
	if set {
		t.Errorf("PromStatus.Chain value 'follow' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("RegFollow returned unexpected status value")
	}
	if !IsChainModeFollow(ns) {
		t.Errorf("unexpected PromStatus.Chain value, expected: follow")
	}
	if IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if !isChainOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than chain's, have changed, unexpectedly")
	}

	// any set to Finally after set to Follow should fail
	set, ns = s.RegRead()
	if set {
		t.Errorf("PromStatus.Chain value 'read' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("RegRead returned unexpected status value")
	}
	if !IsChainModeFollow(ns) {
		t.Errorf("unexpected PromStatus.Chain value, expected: follow")
	}
	if IsChainEmpty(uint32(s)) {
		t.Errorf("unexpected PromStatus.Chain value, expected: read")
	}
	if !isChainOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than chain's, have changed, unexpectedly")
	}
}

// isFateOnlyChanged returns true if all other bits, other than fate's,
// are 0, otherwise it returns false.
func isFateOnlyChanged(s uint32) bool {
	sOtherBits := s & fateBitsCLrMask
	return sOtherBits == 0
}

func TestPromStatus_Fate_State_Resolving(t *testing.T) {
	s := PromStatus(0)

	if !IsFateUnresolved(uint32(s)) {
		t.Errorf("unexpected PromStatus.Fate value, expected: unresolved")
	}
	if !IsStatePending(uint32(s)) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}

	// first set to Resolving should succeed
	set, ns := s.SetResolving()
	if !set {
		t.Errorf("PromStatus.Fate value 'resolving' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetResolving returned unexpected status value")
	}
	if !IsFateResolving(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolving")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// second set to Resolving should fail
	set, ns = s.SetResolving()
	if set {
		t.Errorf("PromStatus.Fate value 'resolving' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetResolving returned unexpected status value")
	}
	if !IsFateResolving(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolving")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// first Resolving clear should succeed
	cleared, ns := s.ClearResolving()
	if !cleared {
		t.Errorf("PromStatus.Fate value 'resolving' not cleared, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("ClearResolving returned unexpected status value")
	}
	if IsFateResolving(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: unresolved")
	}
	if !IsFateUnresolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: unresolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// second Resolving clear should fail
	cleared, ns = s.ClearResolving()
	if cleared {
		t.Errorf("PromStatus.Fate value 'resolving' cleared, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("ClearResolving returned unexpected status value")
	}
	if IsFateResolving(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: unresolved")
	}
	if !IsFateUnresolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: unresolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// first set to Resolved should succeed
	set, ns = s.SetPendingResolved()
	if !set {
		t.Errorf("PromStatus.Fate value 'resolved' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetPendingResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// second set to Resolved should fail
	set, ns = s.SetPendingResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetPendingResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// any set to Resolving after Resolved should fail
	set, ns = s.SetResolving()
	if set {
		t.Errorf("PromStatus.Fate value 'resolving' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetResolving returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}

	// any Resolving clear after Resolved should fail
	cleared, ns = s.ClearResolving()
	if cleared {
		t.Errorf("PromStatus.Fate value 'resolving' cleared, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("ClearResolving returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePending(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: pending")
	}
	if !isFateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's, have changed, unexpectedly")
	}
}

// isFateAndStateOnlyChanged returns true if all other bits, other than fate's,
// and state's are 0, otherwise it returns false.
func isFateAndStateOnlyChanged(s uint32) bool {
	sOtherBits := s & fateBitsCLrMask & stateBitsCLrMask
	return sOtherBits == 0
}

func TestPromStatus_Fate_State_Fulfilled(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	set, ns := s.SetFulfilledResolved()
	if !set {
		t.Errorf("PromStatus.Fate value 'resolved' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetFulfilledResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Resolved should fail
	set, ns = s.SetFulfilledResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetFulfilledResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// first set to Handled should succeed
	set, ns = s.SetHandled()
	if !set {
		t.Errorf("PromStatus.Fate value 'handled' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Handled should fail
	set, ns = s.SetHandled()
	if set {
		t.Errorf("PromStatus.Fate value 'handled' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// any set to Resolved after set to Handled should fail
	set, ns = s.SetFulfilledResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetFulfilledResolved returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func TestPromStatus_Fate_State_Fulfilled_Sync(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	ns := s.SetFulfilledResolvedSync()
	if uint32(s) != ns {
		t.Errorf("Resolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateFulfilled(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: fulfilled")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func TestPromStatus_Fate_State_Rejected(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	set, ns := s.SetRejectedResolved()
	if !set {
		t.Errorf("PromStatus.Fate value 'resolved' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetRejectedResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Resolved should fail
	set, ns = s.SetRejectedResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetRejectedResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// first set to Handled should succeed
	set, ns = s.SetHandled()
	if !set {
		t.Errorf("PromStatus.Fate value 'handled' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Handled should fail
	set, ns = s.SetHandled()
	if set {
		t.Errorf("PromStatus.Fate value 'handled' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// any set to Resolved after set to Handled should fail
	set, ns = s.SetRejectedResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetRejectedResolved returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func TestPromStatus_Fate_State_Rejected_Sync(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	ns := s.SetRejectedResolvedSync()
	if uint32(s) != ns {
		t.Errorf("SetRejectedResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStateRejected(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: rejected")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func TestPromStatus_Fate_State_Panicked(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	set, ns := s.SetPanickedResolved()
	if !set {
		t.Errorf("PromStatus.Fate value 'resolved' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetPanickedResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Resolved should fail
	set, ns = s.SetPanickedResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetPanickedResolved returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// first set to Handled should succeed
	set, ns = s.SetHandled()
	if !set {
		t.Errorf("PromStatus.Fate value 'handled' not set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// second set to Handled should fail
	set, ns = s.SetHandled()
	if set {
		t.Errorf("PromStatus.Fate value 'handled' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetHandled returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}

	// any set to Resolved after set to Handled should fail
	set, ns = s.SetPanickedResolved()
	if set {
		t.Errorf("PromStatus.Fate value 'resolved' set, unexpectedly")
	}
	if uint32(s) != ns {
		t.Errorf("SetPanickedResolved returned unexpected status value")
	}
	if !IsFateHandled(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: handled")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func TestPromStatus_Fate_State_Panicked_Sync(t *testing.T) {
	s := PromStatus(0)

	// first set to Resolved should succeed
	ns := s.SetPanickedResolvedSync()
	if uint32(s) != ns {
		t.Errorf("SetPanickedResolvedSync returned unexpected status value")
	}
	if !IsFateResolved(ns) {
		t.Errorf("unexpected PromStatus.Fate value, expected: resolved")
	}
	if !IsStatePanicked(ns) {
		t.Errorf("unexpected PromStatus.State value, expected: panicked")
	}
	if !isFateAndStateOnlyChanged(uint32(s)) {
		t.Errorf("PromStatus bits, other than fate's and state's, have changed, unexpectedly")
	}
}

func equalsStatusFlags(s1, s2 uint32) bool {
	s1Flags := s1 & flagsBitsSetMask
	s2Flags := s2 & flagsBitsSetMask
	return s1Flags == s2Flags
}

func TestPromStatus_Flags(t *testing.T) {
	tests := []struct {
		name       string
		initStatus uint32
		wantFlags  uint32
	}{
		{
			name:       "no flags",
			initStatus: 0,
			wantFlags:  0,
		},
		{
			name:       "1 flag",
			initStatus: FlagsTypeOnce,
			wantFlags:  FlagsTypeOnce,
		},
		{
			name:       "multiple flags",
			initStatus: FlagsTypeOnce | FlagsTypeTimed | FlagsIsNotSafe | FlagsIsExternal,
			wantFlags:  FlagsTypeOnce | FlagsTypeTimed | FlagsIsNotSafe | FlagsIsExternal,
		},
	}

	for _, tt := range tests {
		// init the status value
		s := PromStatus(tt.initStatus)

		// execute some calls on the status value
		s.RegRead()
		s.RegWait()
		s.RegFollow()
		s.SetCalledFinally()
		s.SetResolving()
		s.ClearResolving()
		s.SetFulfilledResolved()
		s.SetHandled()

		// check that the flags section equals the expected value
		if ns := s.Load(); !equalsStatusFlags(ns, tt.wantFlags) {
			t.Errorf("unexpected PromStatus.Flags value. got: '%v', want: '%v'", ns, tt.wantFlags)
		}
	}
}
