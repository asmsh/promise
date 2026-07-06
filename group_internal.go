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

import "container/list"

// groupCall describes a Group call and how to communicate back to it.
type groupCall[T any] struct {
	// resChan is used to send the Result back to the groupCall's goroutine.
	// this is a new, per call, unbuffered and never closed channel.
	resChan chan<- GroupRes[T]

	// syncChan is used to communicate that the groupCall has been resolved,
	// so that the sending promise(s) can return without blocking on resChan.
	// this is a new, per call, unbuffered channel.
	syncChan <-chan struct{}
}

type groupResHistory[T any] struct {
	// vals holds first 1 [Result] of each [State].
	// the first most or last most [Result] will be saved at index 0.
	vals [3]GroupRes[T]
}

// the 'resMu' must be write-locked before entering.
func (h *groupResHistory[T]) insertRes(res GroupRes[T]) {
	res.Result = getFinalRes(res.Result)

	// save the first unique [Result] in the first empty place.
	for i := range h.vals {
		// found an empty place?
		if h.vals[i].Result == nil {
			h.vals[i] = res
			break
		}

		// found another [Result] with the same [State]?
		if h.vals[i].State() == res.State() {
			break
		}
	}
}

// getRes returns the first most or last most [Result], according to how
// the values have been saved.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getRes() (res GroupRes[T]) {
	// the [Result] we're interested in is always at index 0.
	return h.vals[0]
}

// getStateHist returns the Bitwise-OR of all different states that have
// been sent on this group.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getStateHist() (state State) {
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		state |= h.vals[i].State()
	}
	return state
}

// getLenForState returns the number of [Result] values matching
// the [State] value, state.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) getLenForState(
	includeAll bool,
	state State,
) (l int) {
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		if !includeAll && h.vals[i].Result.State() != state {
			continue
		}
		l++
	}

	return l
}

// appendResForState adds the one or more [Result] values matching
// the [State] value, state, to dst and return it.
//
// the 'resMu' must be read-locked before entering.
func (h *groupResHistory[T]) appendResForState(
	includeAll bool,
	state State,
	dst []GroupRes[T],
) []GroupRes[T] {
	for i := range h.vals {
		if h.vals[i].Result == nil {
			continue
		}
		if !includeAll && h.vals[i].Result.State() != state {
			continue
		}
		dst = append(dst, h.vals[i])
	}

	return dst
}

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
		// check for an existing [Result], and if there's none, register this
		// 'groupCall' on the 'callsQ' -- both atomically under a single read-lock
		// on 'resMu'.
		//
		// this pairs with 'handleGroupCalls', which records a promise's result
		// and snapshots the 'callsQ' under a write-lock on the same 'resMu'. so a
		// concurrently-resolving promise is serialized relative to this section:
		// it is either already recorded and observed in the snapshot here (and
		// returned early), or it observes this 'groupCall' and sends its result
		// on 'resChan' (caught by 'selectResSlow'). it can never be missed, which
		// would otherwise leave the call resolving with an unknown [State] (a
		// panic in 'newSingleRes').
		//
		// note: the lock order is always 'resMu' -> 'callsQMu'.
		g.resMu.RLock()

		// if there was a previous [Result], return early with it, without
		// allocating the channels or registering the 'groupCall'.
		if res := g.getSingleCallResSnapshot(); res.Result != nil {
			g.resMu.RUnlock()
			return newSingleRes(res.State(), res)
		}

		// no [Result] yet: allocate the resChan to receive the [Result] value
		// on, and the syncChan to unblock promises once it's received, then
		// register the 'groupCall'.
		resChan := make(chan GroupRes[T])
		syncChan := make(chan struct{})
		g.callsQMu.Lock()
		call := g.callsQ.PushBack(groupCall[T]{
			resChan:  resChan,
			syncChan: syncChan,
		})
		g.callsQMu.Unlock()

		g.resMu.RUnlock()

		return g.selectResSlow(groupDone, resChan, syncChan, call)
	}
}

func (g *Group[T]) selectResSlow(
	groupDone <-chan struct{},
	resChan <-chan GroupRes[T],
	syncChan chan struct{},
	call *list.Element,
) Result[GroupRes[T]] {
	// wait for a [Result], or for the [Group] to be done.
	var callResState State
	var callRes GroupRes[T]
	select {
	case <-groupDone:
		// the [Group] finished without sending a result to this call. with the
		// atomic register+snapshot in selectRes this is unreachable while any
		// promise resolves (it would observe this call and send), but fall back
		// to the [Group]'s recorded [Result] -- or a zero-[Group] [Success] --
		// rather than resolving to an unknown [State].
		g.resMu.RLock()
		if res := g.getSingleCallResSnapshot(); res.Result != nil {
			callResState = res.State()
			callRes = res
		} else {
			callResState = Success
		}
		g.resMu.RUnlock()
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
// at the time of calling, based on the 'SaveAllGroupResults' flag.
//
// the callResState is the result of the respective [joinOperationLogic.InitState]
// call, given the [Group]'s state history, at the time of calling.
//
// extraResLen is the capacity of the returned [GroupRes] slice that's
// requested by the caller.
// the actual capacity will be bigger than this value by a difference
// based on the 'SaveAllGroupResults' flag.
// the difference will be the length of 'resQ' if 'SaveAllGroupResults'
// is true, and [0:3] otherwise, based on the found matching [Result]
// values in the history.
//
// the 'resMu' must be locked before entering.
func (g *Group[T]) getMultiCallResSnapshot(
	includeAll bool,
	callResState State,
	extraResLen int,
) []GroupRes[T] {
	// if we are saving all group results, return a snapshot of them.
	if g.core.options.IsSaveAllGroupResults() {
		callResLen := extraResLen + g.resQ.Len()
		callRes := make([]GroupRes[T], 0, callResLen)
		for res := g.resQ.Front(); res != nil; res = res.Next() {
			callRes = append(callRes, res.Value.(GroupRes[T]))
		}
		return callRes
	}

	// update the wanted res length with addition required for this callResState.
	callResLen := g.resHist.getLenForState(includeAll, callResState)

	// if there's nothing to be returned, return nil.
	if extraResLen == 0 && callResLen == 0 {
		return nil
	}

	// create the call result.
	callRes := make([]GroupRes[T], 0, extraResLen+callResLen)

	// if there are no results to be fetched from teh history, return early.
	if callResLen == 0 {
		return callRes
	}

	// populate the call result from the history and return it.
	return g.resHist.appendResForState(includeAll, callResState, callRes)
}

func (g *Group[T]) joinRes(op joinOperationLogic) Result[[]GroupRes[T]] {
	groupDone := g.core.sg.WaitChan()

	select {
	case <-groupDone:
		g.resMu.RLock()
		resStateHist := g.resHist.getStateHist()
		// get the 'callResState' from the [Group]'s state history.
		// for a zero [Group] we return a [Success] and empty [Result],
		// otherwise we return the existing [Result].
		callResState := op.initState(resStateHist)

		// if this is a zero [Group], or the call ignores the current
		// [Group] state, use the [Group]'s highest [State] instead,
		// as there are no other promises running and the current state
		// history is all we get.
		if callResState == unknown {
			callResState = calcGroupResState(resStateHist)
		}

		// get the 'callRes' from this [Group] based on the 'callResState'.
		callRes := g.getMultiCallResSnapshot(op == joinOp, callResState, 0)
		g.resMu.RUnlock()

		return newMultiRes(op, callResState, callRes)
	default:
		// if we want to return early, check if the current [Group]
		// state is the target [State] for this call.
		// note: if the check failed, we will need to check the history
		// fields again, because the target [State] might be sent later.
		if op.returnOnTargetState() {
			var callRes []GroupRes[T]
			var targetFound bool

			g.resMu.RLock()
			resStateHist := g.resHist.getStateHist()
			callResState := op.initState(resStateHist)
			if op.isTargetState(callResState) {
				callRes = g.getMultiCallResSnapshot(op == joinOp, callResState, 0)
				targetFound = true
			}
			g.resMu.RUnlock()

			if targetFound {
				return newMultiRes(op, callResState, callRes)
			}

			// target is not found, move forward to the async (slow) handling...
		}

		return g.joinResSlow(groupDone, op)
	}
}

func (g *Group[T]) joinResSlow(
	groupDone <-chan struct{},
	op joinOperationLogic,
) Result[[]GroupRes[T]] {
	var resChan chan GroupRes[T]
	var syncChan chan struct{}

	// register this 'groupCall' on the 'callsQ' and take the [Group]'s state
	// and [Result] snapshot atomically, under a single read-lock on 'resMu'.
	//
	// this pairs with 'handleGroupCalls', which records a promise's result and
	// snapshots the 'callsQ' under a write-lock on the same 'resMu'. so any
	// concurrently-resolving promise is serialized relative to this section: it
	// either registered-then-visible here first (in which case it observes this
	// 'groupCall' and sends its result on 'resChan', caught by the 'resultLoop'
	// below), or it recorded its result before this snapshot (in which case it
	// is included in 'callRes'). it can never be both (a duplicate) nor neither
	// (a lost result).
	//
	// note: the lock order is always 'resMu' -> 'callsQMu', so there's no
	// deadlock, and the blocking 'resChan' sends happen outside of both locks.
	var call *list.Element
	var callResState State
	var callRes []GroupRes[T]
	var targetFound bool

	g.resMu.RLock()

	resStateHist := g.resHist.getStateHist()
	callResState = op.initState(resStateHist)
	if op.returnOnTargetState() && op.isTargetState(callResState) {
		// we want an early return once the target [Result] is found, and it's
		// already present in the [Group]'s recorded results.
		callRes = g.getMultiCallResSnapshot(op == joinOp, callResState, 0)
		targetFound = true
	} else if op.returnOnTargetState() {
		// early-return call, but the target isn't found yet: expect at least
		// all active promises to be included.
		extraResLen := g.core.sg.ActiveCount()
		callRes = g.getMultiCallResSnapshot(op == joinOp, callResState, extraResLen)
		// create the syncChan to handle unblocking promises once the target
		// value is received, but only if we are expecting an early return.
		syncChan = make(chan struct{})
	} else {
		// we want to wait for all promises, active and pending, so init the
		// result array with the expected results number, accounting for all
		// ongoing promises, active and pending.
		//
		// to return the results of all promises that ever ran on this [Group],
		// done and ongoing, we start with a snapshot of the 'resQ', which
		// includes all results already recorded on this [Group]. any promise
		// not yet recorded is guaranteed (by the atomic registration below) to
		// observe this 'groupCall' and send its result to the 'resultLoop'.
		extraResLen := g.core.sg.ActiveCount() + g.core.sg.PendingCount()
		callRes = g.getMultiCallResSnapshot(op == joinOp, callResState, extraResLen)
	}

	// if the target isn't already found, register this 'groupCall' (while still
	// holding 'resMu') so the 'resultLoop' can collect the remaining results.
	if !targetFound {
		resChan = make(chan GroupRes[T])
		g.callsQMu.Lock()
		call = g.callsQ.PushBack(groupCall[T]{
			resChan:  resChan,
			syncChan: syncChan,
		})
		g.callsQMu.Unlock()
	}

	g.resMu.RUnlock()

	// loop over the 'resChan', if the target [State] is not found yet,
	// recording each [Result], and updating the final [State].
resultLoop:
	for !targetFound {
		// the 2 select cases below can't be ready at the same time, because the
		// 'groupDone' channel is closed once all Promise goroutines exit, which
		// happens after 'handleGroupCall' returns, and 'handleGroupCall' is
		// responsible for sending on the 'resChan'.
		select {
		case res := <-resChan:
			res.Result = getFinalRes(res.Result)
			callResState = op.nextState(res.State(), callResState)
			callRes = append(callRes, res)
			if op.returnOnTargetState() && op.isTargetState(res.State()) {
				break resultLoop
			}
		case <-groupDone:
			break resultLoop
		}
	}

	// clean up the 'groupCall', but only if we actually registered it. when the
	// target was already found in the initial snapshot, 'call' is nil and no
	// promise ever learned about our 'resChan'/'syncChan', so there's nothing
	// to unblock or remove.
	if !targetFound {
		// if we are expected to return early, make sure no promises will block
		// on sending to the resChan.
		if op.returnOnTargetState() {
			close(syncChan)
		}

		// remove this 'groupCall' from the 'callsQ', now that it's no longer active.
		g.callsQMu.Lock()
		g.callsQ.Remove(call)
		g.callsQMu.Unlock()
	}

	// if the 'callResState' is still unknown, it means 'initState' deferred the
	// state computation to the results, but all results were captured in the
	// initial snapshot instead of arriving on the 'resChan' (so the loop above
	// never ran 'nextState'). fall back to the [Group]'s state history, mirroring
	// the fast path in 'joinRes'. this happens when promises finish quickly,
	// before this call registers its 'groupCall' on the 'callsQ'.
	if callResState == unknown {
		g.resMu.RLock()
		callResState = calcGroupResState(g.resHist.getStateHist())
		g.resMu.RUnlock()
	}

	return newMultiRes(op, callResState, callRes)
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
	// initState takes history [State] and returns the [Result] [State]
	// that should be fetched from the [Group] history and be included
	// in the [Result] returned by the current call.
	// It returns [unknown] if no fetching from the history should be done.
	initState(stateHist State) State

	// nextState takes the current and previous [State] values and returns
	// the next [State], which will be the resolve state, if there are no
	// more promises to process.
	nextState(currState State, prevState State) (nextState State)

	// returnOnTargetState is an identity method telling whether the current
	// operation should return once the target [State] is found or it should
	// keep going until all promises are processed.
	returnOnTargetState() bool

	// isTargetState takes the current [State] value and returns whether
	// it's the target for this call, and we should resolve/return.
	isTargetState(currState State) bool
}

type allOperation struct{}

// allOperation can only fetch [Panic] or [Error] from the history.
func (allOperation) initState(stateHist State) State {
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

// allOperation gives higher priority to [Panic], then [Error] and [Success].
func (allOperation) nextState(currState State, prevState State) (nextState State) {
	// the order here matters.
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

// allOperation returns once it finds its target [State].
func (allOperation) returnOnTargetState() bool {
	return true
}

func (allOperation) isTargetState(currState State) bool {
	return currState == Panic || currState == Error
}

type allWaitOperation struct{ allOperation }

// allWaitOperation returns only after processing all promises.
func (a allWaitOperation) returnOnTargetState() bool {
	return false
}

type anyOperation struct{}

// anyOperation can only fetch [Success] from the history.
func (anyOperation) initState(stateHist State) State {
	// the order here matters.
	switch {
	case stateHist == unknown:
		return unknown
	case stateHist&Success == Success:
		return Success
	case stateHist&Panic == Panic:
		return unknown
	case stateHist&Error == Error:
		return unknown
	}

	// unexpected state.
	return stateHist
}

// anyOperation gives higher priority to [Success], then [Panic] and [Error].
func (anyOperation) nextState(currState State, prevState State) (nextState State) {
	// the order here matters.
	switch {
	case currState == Success || prevState == Success:
		return Success
	case currState == Panic:
		return Panic
	case prevState == unknown:
		return currState
	default:
		return prevState
	}
}

func (anyOperation) returnOnTargetState() bool {
	return true
}

func (anyOperation) isTargetState(currState State) bool {
	return currState == Success
}

type anyWaitOperation struct{ anyOperation }

// anyWaitOperation returns only after processing all promises.
func (a anyWaitOperation) returnOnTargetState() bool {
	return false
}

type joinOperation struct{}

// joinOperation includes all [Result] values from the history.
func (joinOperation) initState(State) State {
	// value doesn't matter, as it's handled by comparing against [joinOp].
	return Success
}

// joinOperation always resolved to [Success].
func (joinOperation) nextState(State, State) State {
	return Success
}

// joinOperation returns only after processing all promises.
func (joinOperation) returnOnTargetState() bool {
	return false
}

func (joinOperation) isTargetState(State) bool {
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
	case stateHist&Error == Error:
		return Error
	case stateHist&Success == Success:
		return Success
	}

	// unexpected state.
	return stateHist
}
