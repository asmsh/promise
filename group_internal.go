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

// the resMu must be locked before entering.
func (g *Group[T]) getSingleCallResSnapshot() GroupRes[T] {
	if g.core.options.IsSaveAllGroupResults() {
		front := g.resQ.Front()
		if front == nil {
			return GroupRes[T]{}
		}
		return front.Value.(GroupRes[T])
	}

	// fetch the call's result from the history.
	return g.resHist.getRes()
}

func (g *Group[T]) selectRes() Result[GroupRes[T]] {
	// init the wait chan of this group.
	groupDone := g.core.sg.WaitChan()

	// check whether this is a sync or an async handling.
	// it will be a sync call iff the [Group] is already done,
	// which happens when the 'groupDone' is closed.
	// this will be used to not add the 'groupCall' to the 'callsQ',
	// and instead just read directly from the 'resQ' or 'resHist'.
	select {
	case <-groupDone:
		// fast-path, for a zero or a done [Group], with no unneeded allocations.
		g.resMu.RLock()
		// for a zero [Group] we return a [Success] and empty [Result],
		// otherwise we return the existing [Result].
		callResState := Success
		callRes := GroupRes[T]{}
		if res := g.getSingleCallResSnapshot(); res.Result != nil {
			callResState = res.State()
			callRes = res
		}
		g.resMu.RUnlock()

		return newSingleRes(callResState, callRes)
	default:
		var callResState State
		var callRes GroupRes[T]
		var targetFound bool

		// if there was a previous [Result], return early with it.
		g.resMu.RLock()
		if res := g.getSingleCallResSnapshot(); res.Result != nil {
			callResState = res.State()
			callRes = res
			targetFound = true
		}
		g.resMu.RUnlock()

		// we found our target, so return the expected [Result] values.
		if targetFound {
			return newSingleRes(callResState, callRes)
		}

		// target is not found, move forward to the async handling...

		return g.selectResSlow(groupDone)
	}
}

func (g *Group[T]) selectResSlow(groupDone <-chan struct{}) Result[GroupRes[T]] {
	// create the needed resChan to receive the [Result] value on it,
	// and the syncChan to handle unblocking promises once the target
	// value is received.
	resChan := make(chan GroupRes[T])
	syncChan := make(chan struct{})

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it.
	g.callsQMu.Lock()
	call := g.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.callsQMu.Unlock()

	// wait for a [Result], or for the [Group] to be done.
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
	// note: this has to be before the removal of the 'groupCall' from
	// the 'callsQ', otherwise we will have a deadlock on 'callsQMu'.Lock().
	close(syncChan)

	// remove this 'groupCall' from the 'callsQ', now that it's no longer active.
	g.callsQMu.Lock()
	g.callsQ.Remove(call)
	g.callsQMu.Unlock()

	return newSingleRes(callResState, callRes)
}

// getMultiCallResSnapshot returns a copy of this [Group]'s results,
// at the time of calling, based on the 'saveAllGroupResults' flag.
//
// the callResState is the result of the respective 'calcInitStateFunc'
// call, given the [Group]'s state history, at the time of calling.
//
// callResLen is the capacity of the returned [GroupRes] slice that's
// requested by the caller.
// the actual capacity will be bigger than this value by a difference
// based on the 'saveAllGroupResults' flag.
// the difference will be the length of 'resQ' if 'saveAllGroupResults'
// is true, and 1 otherwise.
//
// the 'resMu' must be locked before entering.
func (g *Group[T]) getMultiCallResSnapshot(callResState State, callResLen int) []GroupRes[T] {
	if g.core.options.IsSaveAllGroupResults() {
		callResLen += g.resQ.Len()
		callRes := make([]GroupRes[T], 0, callResLen)
		for res := g.resQ.Front(); res != nil; res = res.Next() {
			callRes = append(callRes, res.Value.(GroupRes[T]))
		}
		return callRes
	}

	// fetch the call's result from the history.
	res := g.resHist.getStateRes(callResState)

	// if no matching [Result] is found, either return nil, or return
	// the new callResLen with the expected cap.
	if res.Result == nil && callResLen == 0 {
		return nil
	} else if res.Result == nil {
		return make([]GroupRes[T], 0, callResLen)
	}

	// we found a [Result] that matches the group call...
	// create the callRes with the expected cap, and include it in
	// the returned callRes.
	return append(make([]GroupRes[T], 0, callResLen+1), res)
}

func (g *Group[T]) joinRes(op joinOperationLogic) Result[[]GroupRes[T]] {
	groupDone := g.core.sg.WaitChan()

	select {
	case <-groupDone:
		g.resMu.RLock()
		// get the 'callResState' from the [Group]'s state history.
		// for a zero [Group] we return a [Success] and empty [Result],
		// otherwise we return the existing [Result].
		callResState := op.InitState(g.resStateHist)

		// if this is a zero [Group], or the call ignores the current
		// [Group] state, use the [Group]'s highest [State] instead,
		// as there are no other promises running and the current state
		// history is all we get.
		if callResState == unknown {
			callResState = calcGroupResState(g.resStateHist)
		}

		// get the 'callRes' from this [Group] based on the 'callResState'.
		callRes := g.getMultiCallResSnapshot(callResState, 0)
		g.resMu.RUnlock()

		return newMultiRes(callResState, callRes)
	default:
		// if we want to return early, check if the current [Group]
		// state is the target [State] for this call.
		// note: if the check failed, we will need to check the history
		// fields again, because the target [State] might be sent later.
		if op.ReturnOnTargetState() {
			var callRes []GroupRes[T]
			var targetFound bool

			g.resMu.RLock()
			callResState := op.InitState(g.resStateHist)
			if op.IsTargetState(callResState) {
				callRes = g.getMultiCallResSnapshot(callResState, 0)
				targetFound = true
			}
			g.resMu.RUnlock()

			if targetFound {
				return newMultiRes(callResState, callRes)
			}

			// target is not found, move forward to the async handling...
		}

		return g.joinResSlow(groupDone, op)
	}
}

func (g *Group[T]) joinResSlow(
	groupDone <-chan struct{},
	op joinOperationLogic,
) Result[[]GroupRes[T]] {
	resChan := make(chan GroupRes[T])

	// create the syncChan to handle unblocking promises once the target
	// value is received, but onl if we are expecting an early return.
	var syncChan chan struct{}
	if op.ReturnOnTargetState() {
		syncChan = make(chan struct{})
	}

	// record this 'groupCall' in the 'callsQ' for this [Group], enforcing
	// all promises that observe it to wait and send the result to it.
	g.callsQMu.Lock()
	call := g.callsQ.PushBack(groupCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
	})
	g.callsQMu.Unlock()

	// init the result fields, and account for the [Group] state.
	var callResState State
	var callRes []GroupRes[T]
	var targetFound bool
	if op.ReturnOnTargetState() {
		// we want an early return, once the target [Result] is found.
		g.resMu.RLock()
		callResState = op.InitState(g.resStateHist)
		if op.IsTargetState(callResState) {
			// we found our target, so return the expected [Result] values.
			callRes = g.getMultiCallResSnapshot(callResState, 0)
			targetFound = true
		} else {
			// expect at least all active promises to be included.
			callResLen := g.core.sg.ActiveCount()
			callRes = g.getMultiCallResSnapshot(callResState, int(callResLen))
		}
		g.resMu.RUnlock()
	} else {
		// we want to wait for all promises, active and pending, so
		// init the result array with the expected results number.
		//
		// note: we account for all ongoing promises, active and pending,
		// because at this point, we propagated this 'groupCall' to all
		// ongoing promises, and since they block on either a sending
		// to the 'resChan', or receiving from the 'syncChan', and as both
		// operations will only unblock in the 'resultLoop' below, then
		// all results will be seen by the 'resultLoop'.
		//
		// note: to return the results of all promises that ever ran on
		// this [Group], done and ongoing, we start with a snapshot of
		// the 'resQ', which at this point, will include all results
		// that have been already sent on this [Group] and consumed by
		// other 'groupCall's, except this one.
		//
		// this is guaranteed by how the 'handleGroupCalls' is implemented,
		// which blocks on each 'groupCall', then inserts to the 'resQ'
		// only after it's unblocked in the 'resultLoop'.
		// (see the note above for details).
		//
		// given that the inserts to the 'resQ' are protected by a write
		// lock (Lock) on the 'resMu'.
		// and that the copy below is protected by a read lock (RLock).
		// we know that once we RLock the 'resMu', the 'resQ' will have
		// the up-to-date [Result] values, and any future inserts to the
		// 'resQ' will also be sent to the 'resChan' and caught in the
		// 'resultLoop' (as part of unblocking 'handleGroupCalls').

		// get the [State] of this [Group], and a snapshot of its [Result],
		// expecting at least all ongoing promises to be included.
		g.resMu.RLock()
		callResState = op.InitState(g.resStateHist)
		callResLen := g.core.sg.ActiveCount() + g.core.sg.PendingCount()
		callRes = g.getMultiCallResSnapshot(callResState, int(callResLen))
		g.resMu.RUnlock()
	}

	// loop over the 'resChan', if the target [State] is not found yet,
	// recording each [Result], and updating the final [State].
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
	// note: this has to be before the removal of the 'groupCall' from
	// the 'callsQ', otherwise we will have a deadlock on 'callsQMu'.Lock().
	if op.ReturnOnTargetState() {
		close(syncChan)
	}

	// remove this 'groupCall' from the 'callsQ', now that it's no longer active.
	g.callsQMu.Lock()
	g.callsQ.Remove(call)
	g.callsQMu.Unlock()

	return newMultiRes(callResState, callRes)
}

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
// [Panic] has the highest priority.
// [Success] has the highest priority between [Success] and [Error].
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

func (joinOperation) InitState(State) State {
	return Success
}
func (joinOperation) NextState(State, State) State {
	return Success
}
func (joinOperation) ReturnOnTargetState() bool {
	return false
}
func (joinOperation) IsTargetState(State) bool {
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
