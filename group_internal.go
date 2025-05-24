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

var (
	allOp     joinOperationLogic = allOperation{}
	allWaitOp joinOperationLogic = allWaitOperation{}
	anyOp     joinOperationLogic = anyOperation{}
	anyWaitOp joinOperationLogic = anyWaitOperation{}
	joinOp    joinOperationLogic = joinOperation{}
)

// joinOperationLogic encapsulates the logic for each of the join calls.
type joinOperationLogic interface {
	// InitState takes history [State] and returns the [Result] [State]
	// that should be fetched from the [Group] history and be included
	// in the [Result] returned by the current call.
	// It returns [unknown] if no fetching from the history should be done.
	InitState(stateHist State) State

	// NextState takes the current and previous [State] values and returns
	// the next [State], which will be the resolve state, if there are no
	// more promises to process.
	NextState(currState State, prevState State) (nextState State)

	// ReturnOnTargetState is an identity method telling whether the current
	// operation should return once the target [State] is found or it should
	// keep going until all promises are processed.
	ReturnOnTargetState() bool

	// IsTargetState takes the current [State] value and returns whether
	// it's the target for this call, and we should resolve/return.
	IsTargetState(currState State) bool
}

type allOperation struct{}

func (allOperation) InitState(stateHist State) State {
	return calcAllInitState(stateHist)
}
func (allOperation) NextState(currState State, prevState State) (nextState State) {
	return calcAllNextState(currState, prevState)
}
func (allOperation) ReturnOnTargetState() bool {
	return true
}
func (allOperation) IsTargetState(currState State) bool {
	return checkTargetAllState(currState)
}

type allWaitOperation struct{ allOperation }

func (a allWaitOperation) ReturnOnTargetState() bool {
	return false
}

type anyOperation struct{}

func (anyOperation) InitState(stateHist State) State {
	return calcAnyInitState(stateHist)
}
func (anyOperation) NextState(currState State, prevState State) (nextState State) {
	return calcAnyNextState(prevState, currState)
}
func (anyOperation) ReturnOnTargetState() bool {
	return true
}
func (anyOperation) IsTargetState(currState State) bool {
	return checkTargetAnyState(currState)
}

type anyWaitOperation struct{ anyOperation }

func (a anyWaitOperation) ReturnOnTargetState() bool {
	return false
}

type joinOperation struct{}

func (joinOperation) InitState(stateHist State) State {
	return calcJoinInitState(stateHist)
}
func (joinOperation) NextState(currState State, prevState State) (nextState State) {
	return calcJoinNextState(prevState, currState)
}
func (joinOperation) ReturnOnTargetState() bool {
	return false
}
func (joinOperation) IsTargetState(_ State) bool {
	// value doesn't matter, as [ReturnOnTargetState] returns false.
	return false
}

// calcGroupResState returns the [Result] [State] that should be returned
// from a [Group] if the respective InitState function returned [unknown],
// given that [Group]'s state history, stateHist.
func calcGroupResState(stateHist State) State {
	// the order here matters.
	switch {
	case stateHist == unknown: // a zero [Group] is a [Success] one.
		return Success
	case stateHist&Panic == Panic:
		return Panic
	case stateHist&Success == Success:
		return Success
	case stateHist&Error == Error:
		return Error
	}

	// unexpected state.
	return stateHist
}
