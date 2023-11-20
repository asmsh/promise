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

// NewFrom creates a new PromStatus from the passed status, only carrying
// the 'flags' bits of the status value to the newly created PromStatus.
func NewFrom(status uint32) PromStatus {
	return PromStatus(status & flagsBitsSetMask)
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
