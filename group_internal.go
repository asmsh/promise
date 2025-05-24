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

func (g *Group[T]) selectRes() Result[GroupRes[T]] {
	// init the wait chan of this group.
	groupDone := g.core.sg.WaitChan()

	// check whether this is a sync call or an async call.
	// it will be a sync call iff the [Group] is already done,
	// which happens when the 'groupDone' is closed.
	// this will be used to not add the 'groupCall' to the 'callsQ',
	// and instead just read directly from the 'resQ'.
	select {
	case <-groupDone:
		// TODO: how to know which saved Result is the first, to return here,
		// like what we do with the AllRes, etc.
		// fast-path, with no unneeded allocations in 'selectResSlow'.
		// return a [Success] [GroupRes] with empty [Result].
		// this could happen either on a zero [Group], or when
		// the [Group] has no ongoing promises, and in both cases
		// the [Select] can't observe any [Result] values.
		return newSingleRes[T, GroupRes[T]](Success, GroupRes[T]{})
	default:
		return g.selectResSlow(groupDone)
	}
}

func (g *Group[T]) selectResSlow(groupDone <-chan struct{}) Result[GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])
	syncChan := make(chan struct{})

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// wait for a [Result] or for the [Group] to be done.
	var callResState State
	var callRes GroupRes[T]
	select {
	case <-groupDone:
	case res := <-resChan:
		callResState = res.State()
		callRes = res
	}

	// close the 'syncChan' to prevent other promises from blocking on
	// sending to the 'resChan', once the wanted [Result] is received.
	close(syncChan)

	// remove this groupCall from the callsQ, now that it's no longer active.
	// note: this is just a cleanup action, as this groupCall will not be used
	// by anything (there are no running promises to use anymore).
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	// handle not receiving any [Result], or calls on a zero [Group].
	// it could still happen that we don't receive any [Result] if we
	// propagated this 'groupCall' after the ongoing promises passed
	// the 'handleGroupCall' loop in the 'handleGroupCalls'.
	// so, in this case, it will be treated as a sync call.
	if callResState == unknown {
		return newSingleRes[T, GroupRes[T]](Success, GroupRes[T]{})
	}

	return newSingleRes(callResState, callRes)
}

// checkTargetStateFunc is optional and only  needed if we want to return early.
func (g *Group[T]) joinRes(op joinOperationLogic) Result[[]GroupRes[T]] {
	// init the wait chan of this group.
	groupDone := g.core.sg.WaitChan()

	// check whether this is a sync call or an async call.
	// it will be a sync call iff the [Group] is already done,
	// which happens when the 'groupDone' is closed.
	// this will be used to not add the 'groupCall' to the 'callsQ',
	// and instead just read directly from the 'resQ'.
	select {
	case <-groupDone:
		// get the 'callResState' and 'callRes' from this [Group].
		g.core.resMu.RLock()
		// if the state history is 0, then this is a zero [Group],
		// so return a [Success] [GroupRes] with empty [Result].
		callResState := op.InitState(g.core.resStateHist)
		if callResState == unknown {
			callResState = calcGroupResState(g.core.resStateHist)
		}
		callRes := g.getCallResSnapshot(callResState, 0)
		g.core.resMu.RUnlock()

		return newMultiRes(callResState, callRes)
	default:
		// if we want to return early, check if the current [Group]
		// state is the target [State] for this call.
		// note: if the check failed, we will need to check the history
		// fields again, because the target [State] might be sent later
		// on the resQ.
		// note: the history fields set happens-before the resQ send,
		// so, to be sure that we didn't miss anything, we need to check
		// the history fields _after_ the resQ send.
		if op.ReturnOnTargetState() {
			var callResState State
			var callRes []GroupRes[T]
			var targetFound bool

			g.core.resMu.RLock()
			callResState = op.InitState(g.core.resStateHist)
			if op.IsTargetState(callResState) {
				// we found our target, so return the expected [Result] values.
				targetFound = true
				callRes = g.getCallResSnapshot(callResState, 0)
			}
			g.core.resMu.RUnlock()

			if targetFound {
				return newMultiRes(callResState, callRes)
			}

			// target is not found, move forward to joinResSlow...
		}

		return g.joinResSlow(groupDone, op)
	}
}

// getCallResSnapshot returns a copy of this [Group]'s results,
// at the time of calling, based on the 'saveAllGroupResults' flag.
//
// the callResState is the result of the respective 'calcInitStateFunc'
// call, given the [Group]'s state history, at the time of calling.
//
// callResLen is the capacity of the returned [GroupRes] slice that's
// requested by the caller.
// the actual capacity will be bigger than this value by a difference
// based on the 'saveAllGroupResults' flag.
// the difference will be the length of resQ if 'saveAllGroupResults'
// is true, and 1 otherwise.
//
// the resMu must be locked before entering.
func (g *Group[T]) getCallResSnapshot(callResState State, callResLen int) []GroupRes[T] {
	if g.core.saveAllGroupResults {
		callResLen += g.core.resQ.Len()
		callRes := make([]GroupRes[T], 0, callResLen)
		for res := g.core.resQ.Front(); res != nil; res = res.Next() {
			callRes = append(callRes, res.Value.(GroupRes[T]))
		}
		return callRes
	}

	// fetch the result from the history based on the callResState.
	var val any
	switch callResState {
	case Panic:
		if res := g.core.resHist.panic; res != nil {
			val = res
			callResLen += 1
		}
	case Error:
		if res := g.core.resHist.error; res != nil {
			val = res
			callResLen += 1
		}
	case Success:
		if res := g.core.resHist.success; res != nil {
			val = res
			callResLen += 1
		}
	default:
		// do nothing, as it means the call doesn't expect a result
		// to be added from the history.
	}

	// create the callRes with the final expected cap, only if there
	// are results expected, otherwise leave it nil.
	var callRes []GroupRes[T]
	if callResLen > 0 {
		callRes = make([]GroupRes[T], 0, callResLen)
	}

	// if we found a result that matches the group call, include it.
	if val != nil {
		callRes = append(callRes, val.(GroupRes[T]))
	}

	return callRes
}

// checkTargetStateFunc is optional and only needed if we want to return early.
func (g *Group[T]) joinResSlow(
	groupDone <-chan struct{},
	op joinOperationLogic,
) Result[[]GroupRes[T]] {
	// create the needed res channel to receive the result on it.
	resChan := make(chan GroupRes[T])

	// only create the syncChan if we are expecting an early return.
	var syncChan chan struct{}
	if op.ReturnOnTargetState() {
		syncChan = make(chan struct{})
	}

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it
	g.core.callsQMu.Lock()
	call := g.core.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.core.callsQMu.Unlock()

	// init the result array, and account for the [Group] state only
	// if we are waiting for all promises.
	var callResState State
	var callRes []GroupRes[T]
	var targetFound bool
	if op.ReturnOnTargetState() {
		g.core.resMu.RLock()
		callResState = op.InitState(g.core.resStateHist)
		if op.IsTargetState(callResState) {
			// we found our target, so return the expected [Result] values.
			targetFound = true
			callRes = g.getCallResSnapshot(callResState, 0)
		} else {
			// expect at least all active promises to be included.
			callResLen := g.core.sg.ActiveCount()
			callRes = g.getCallResSnapshot(callResState, int(callResLen))
		}
		g.core.resMu.RUnlock()
	} else {
		// we want to wait for all promises, active and pending, so
		// init the result array with the expected results number.
		//
		// note: we account for ALL ongoing promises (active and pending),
		// because at this point, we propagated this 'groupCall' to all
		// ongoing promises, and since they block on either a sending
		// to this 'groupCall''s resChan, or receiving from its syncChan,
		// and as both operations will only unblock in the resultLoop
		// below, then all results will be seen by the resultLoop.
		//
		// note: to return the results of all promises that ever ran on
		// this [Group], done and ongoing, we start with a snapshot of
		// the 'resQ', which at this point, will include all results
		// that have been sent on this [Group] before and already
		// consumed by other 'groupCall', except this one.
		//
		// this is guaranteed by how the 'handleGroupCalls' is implemented,
		// which blocks once this 'groupCall' is propagated to all promises,
		// and inserts to the 'resQ' after this 'groupCall' unblocks them.
		// (see the note above for details).
		//
		// since the inserts to the 'resQ' are protected by a write lock
		// (Lock) on the resMu.
		// and the copy below is protected by a read lock (RLock);
		// we know that once we RLock the 'resMu', NO ongoing calls
		// will update the 'resQ' until this 'groupCall' unblocks in the
		// 'resultLoop' below.
		//
		// this also means that all further inserts to the 'resQ' will
		// also be sent to the 'resChan' of this 'groupCall'.
		//
		// after starting with the snapshot, we will move forward
		// to the 'resultLoop' below, which will unblock all ongoing
		// promises and append the got [Result] to the 'callRes'.
		// the 'callRes' now includes all results so far.

		// get the [State] of this [Group], and a snapshot of its [Result],
		// expecting at least all ongoing promises to be included.
		g.core.resMu.RLock()
		callResState = op.InitState(g.core.resStateHist)
		callResLen := g.core.sg.ActiveCount() + g.core.sg.PendingCount()
		callRes = g.getCallResSnapshot(callResState, int(callResLen))
		g.core.resMu.RUnlock()
	}

	// loop over the 'resChan', if the target [State] is not found yet,
	// recording each [Result], and updating the final [State] based on
	// the provided 'calcNextStateFunc'.
resultLoop:
	for !targetFound {
		// the 2 select cases below can't be ready at the same time.
		// because the 'groupDone' channel is closed once all Promise values
		// exits, which happens after the 'handleGroupCall' method returns,
		// and the 'handleGroupCall' method is responsible for sending on
		// the 'resChan'.
		select {
		case res := <-resChan:
			callResState = op.NextState(res.State(), callResState)
			callRes = append(callRes, res)
			if op.ReturnOnTargetState() && op.IsTargetState(res.State()) {
				break resultLoop
			}
		case <-groupDone:
			break resultLoop
		}
	}

	// if we are expected to return early, make sure no promises will block
	// on sending to the resChan.
	if op.ReturnOnTargetState() {
		close(syncChan)
	}

	// remove this 'groupCall' from the 'callsQ', now that it's no longer active.
	g.core.callsQMu.Lock()
	g.core.callsQ.Remove(call)
	g.core.callsQMu.Unlock()

	return newMultiRes(callResState, callRes)
}

type allOperation struct{}

// InitState returns the [Result] [State] that should be fetched from
// the [Group] history and be included in the [Result] returned by
// [All], or [AllWait].
// It returns 0 ([unknown]) if no fetching from the history should be done.
// [Panic] has the highest priority.
// [Success] is ignored, as no [Success] should be fetched from history.
// It returns either [Panic], [Error], or 0 ([unknown]).
func (allOperation) InitState(stateHist State) State {
	// the order here matters.
	switch {
	case stateHist == unknown:
		return unknown
	case stateHist&Panic == Panic:
		return Panic
	case stateHist&Error == Error:
		return Error
	case stateHist&Success == Success:
		return unknown
	}

	// unexpected state.
	return stateHist
}

// NextState returns the resolve state of the promise returned by [All],
// or [AllWait].
// [Panic] has the highest priority.
// [Error] has the highest priority between [Success] and [Error].
func (allOperation) NextState(currState State, prevState State) (nextState State) {
	switch {
	case currState == Panic || prevState == Panic:
		return Panic
	case currState == Error:
		return Error
	case prevState == unknown:
		return currState
	default:
		return prevState
	}
}

func (allOperation) ReturnOnTargetState() bool {
	return true
}

// IsTargetState breaks on [Error] only, as it ignores [Panic] and is
// okay with [Success].
func (allOperation) IsTargetState(currState State) bool {
	return currState == Error
}

type allWaitOperation struct{ allOperation }

func (a allWaitOperation) ReturnOnTargetState() bool {
	return false
}

type anyOperation struct{}

// InitState returns the [Result] [State] that should be fetched
// from the [Group] history and be included in the [Result] returned by
// [Any], or [AnyWait].
// It returns 0 ([unknown]) if no fetching from the history should be done.
// [Panic] has the highest priority.
// [Error] is ignored, as no [Error] should be fetched from history.
// It returns either [Panic], [Error], or 0 ([unknown]).
func (anyOperation) InitState(stateHist State) State {
	// the order here matters.
	switch {
	case stateHist == unknown:
		return unknown
	case stateHist&Panic == Panic:
		return Panic
	case stateHist&Success == Success:
		return Success
	case stateHist&Error == Error:
		return unknown
	}

	// unexpected state.
	return stateHist
}

// NextState returns the resolve state of the promise returned by [Any],
// [AnyWait].
// Panic has the highest priority.
// Success has the highest priority between Success and Error.
func (anyOperation) NextState(currState State, prevState State) (nextState State) {
	switch {
	case currState == Panic || prevState == Panic:
		return Panic
	case currState == Success:
		return Success
	case prevState == unknown:
		return currState
	default:
		return prevState
	}
}

func (anyOperation) ReturnOnTargetState() bool {
	return true
}

// IsTargetState breaks on [Success] only, as it ignores [Panic] and is
// okay with [Error].
func (anyOperation) IsTargetState(currState State) bool {
	return currState == Success
}

type anyWaitOperation struct{ anyOperation }

func (a anyWaitOperation) ReturnOnTargetState() bool {
	return false
}

type joinOperation struct{}

func (joinOperation) InitState(stateHist State) State {
	return Success
}
func (joinOperation) NextState(currState State, prevState State) (nextState State) {
	return Success
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
