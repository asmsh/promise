// Copyright 2023 Ahmad Sameh(asmsh)
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

import (
	"github.com/asmsh/uniquerand"
)

// Wait blocks until all the passed [Promise] values, p, resolves.
func Wait[T any](p ...*Promise[T]) {
	for _, pp := range p {
		<-pp.WaitChan()
	}
}

// Select returns a [Promise] that resolves to the first [Promise]
// resolved from p.
//
// The resulting [IdxRes] value holds the [Result] value of the resolved
// [Promise], and the original order is saved in the [IdxRes.Idx] field.
func Select[T any](p ...*Promise[T]) *Promise[IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[IdxRes[T]](nil)
	}

	nextProm := newPromInter[IdxRes[T]](nil)
	go selectHandler(nextProm, p)
	return nextProm
}

func selectHandler[T any](
	nextProm *Promise[IdxRes[T]],
	ps []*Promise[T],
) {
	// resChan is populated lazily, only if it's needed.
	var resChan chan IdxRes[T]

	// res represent the resolve result.
	res := IdxRes[T]{}

	// loopCnt records how many iterations happened in the loop below.
	var loopCnt int

	// randIdx responsible for returning a random, unique, index in the provided
	// list of promises.
	var randIdx uniquerand.Int
	randIdx.Reset(len(ps))

loop:
	for idx, ok := randIdx.Get(); ok; idx, ok = randIdx.Get() {
		currProm := ps[idx]
		loopCnt++

		// Select with non-blocking or with blocking, based on whether we might be
		// interested to check other promises for potential immediate resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(ps)

		// extract the current promise's waitChan since it's used in both flows.
		waitChan := currProm.WaitChan()

		if !blocking {
			select {
			case <-waitChan:
				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			default:
				// this would happen when the promise is not resolved yet.
				// re-put the index, to re-visit this case later.
				randIdx.Put(idx)
			}
		} else {
			extsChan := currProm.extsChan()
			select {
			case <-waitChan:
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			case extQ := <-extsChan:
				// make sure to check the state again and get the sync result,
				// in case the promise got resolved while calling extsChan...
				if extQ.initState != unknown {
					res = IdxRes[T]{
						Idx:    idx,
						Result: getFinalRes(currProm.res),
					}
				} else {
					// the promise is not resolved yet...
					// create the res chan if it's not already created.
					if resChan == nil {
						resChan = make(chan IdxRes[T])
					}

					// update the queue with this extension call.
					addExtCallToQ(&extQ, resChan, nextProm.syncChan, idx)
				}

				// send the updated queue back for either another extension call,
				// or to be included in the currProm's resolving logic.
				extsChan <- extQ
			}
		}

		// if the promise was resolved synchronously, break and use its result.
		if res.Result != nil {
			break loop
		}
	}

	// because this is a Select extension call, only one result is expected.
	if res.Result == nil {
		res = <-resChan
	}

	// resolve the next promise as expected, based on the final callResState.
	nextProm.resolveToRes(newSingleRes(res.State(), res))
}

// All returns a [Promise] that resolves to [Success] if all the passed
// [Promise] values, p, resolved to [Success].
// It resolves to [Panic] if at least one resolved to [Panic].
// It resolves to [Error] if at least one resolved to [Error] and none resolved
// to [Panic].
//
// The returned [Promise] resolves on the first [Error], otherwise it waits
// all the passed [Promise] values to resolve.
// It doesn't resolve on [Panic], despite the fact that the returned [Promise]
// will be resolved to [Panic], if there's any.
//
// The resulting [IdxRes] slice holds the [Result] values of the [Promise]
// values passed, up to when the returned [Promise] was resolved, and their
// original order is saved in the [IdxRes.Idx] field.
func All[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, allOp)
	return nextProm
}

// AllWait returns a [Promise] that resolves to [Success] if all the passed
// [Promise] values, p, resolved to [Success].
// It resolves to [Panic] if at least one resolved to [Panic].
// It resolves to [Error] if at least one resolved to [Error].
//
// The returned [Promise] waits for all the passed [Promise] values to resolve.
//
// The resulting [IdxRes] slice holds the [Result] values of all [Promise]
// values passed, and their original order is saved in the [IdxRes.Idx] field.
func AllWait[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, allWaitOp)
	return nextProm
}

// Any returns a [Promise] that resolves to [Success] if at least one of
// the passed [Promise] values, p, resolved to [Success] and none resolved
// to [Panic].
// It resolves to [Panic] if at least one resolved to [Panic].
// It resolves to [Error] if all resolved to [Error].
//
// The returned [Promise] resolves on the first [Success], otherwise it waits
// all the passed [Promise] values to resolve.
// It doesn't resolve on [Panic], despite the fact that the returned [Promise]
// will be resolved to [Panic], if there's any.
//
// The resulting [IdxRes] slice holds the [Result] values of the [Promise]
// values passed, up to when the returned [Promise] was resolved, and their
// original order is saved in the [IdxRes.Idx] field.
func Any[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, anyOp)
	return nextProm
}

// AnyWait returns a [Promise] that resolves to [Success] if at least one of
// the passed [Promise] values, p, resolved to [Success] and none resolved
// to [Panic].
// It resolves to [Panic] if at least one resolved to [Panic].
// It resolves to [Error] if all resolved to [Error].
//
// The returned [Promise] waits for all the passed [Promise] values to resolve.
//
// The resulting [IdxRes] slice holds the [Result] values of all [Promise]
// values passed, and their original order is saved in the [IdxRes.Idx] field.
func AnyWait[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, anyWaitOp)
	return nextProm
}

// Join returns a [Promise] that resolves to [Success] after all the passed
// [Promise] values, p, resolves.
//
// The returned [Promise] waits for all the passed [Promise] values to resolve.
//
// The resulting [IdxRes] slice holds the [Result] values of all [Promise]
// values passed, and their original order is saved in the [IdxRes.Idx] field.
func Join[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, joinOp)
	return nextProm
}

func joinHandler[T any](
	nextProm *Promise[[]IdxRes[T]],
	ps []*Promise[T],
	op joinOperationLogic,
) {
	// resChan is populated lazily, only if it's needed.
	var resChan chan IdxRes[T]

	// callResState and callRes, collectively, represent the final [Result].
	callResState := unknown
	callRes := make([]IdxRes[T], 0, len(ps))

	// loopCnt records how many iterations happened in the loop below
	var loopCnt int

	// randIdx responsible for returning a random, unique, index in the provided
	// list of promises.
	var randIdx uniquerand.Int
	randIdx.Reset(len(ps))

	// try to find a suitable resolved promise, based on the provided flags,
	// or arrange for a notification once a promise is resolved.
	for idx, ok := randIdx.Get(); ok; idx, ok = randIdx.Get() {
		currProm := ps[idx]
		loopCnt++

		// Select with non-blocking or with blocking, based on whether we might be
		// interested in checking other promises for potential sync resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(ps)

		// extract the current promise's waitChan since it's used in both flows.
		waitChan := currProm.WaitChan()

		var res IdxRes[T]
		if !blocking {
			select {
			case <-waitChan:
				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			default:
				// this would happen when the promise is not resolved yet.
				// re-put the index, to re-visit this case later.
				randIdx.Put(idx)
			}
		} else {
			extsChan := currProm.extsChan()
			select {
			case <-waitChan:
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			case extQ := <-extsChan:
				// make sure to check the state again and get the sync result,
				// in case the promise got resolved while calling extsChan...
				if extQ.initState != unknown {
					res = IdxRes[T]{
						Idx:    idx,
						Result: getFinalRes(currProm.res),
					}
				} else {
					// the promise is not resolved yet...
					// create the res chan if it's not already created.
					if resChan == nil {
						resChan = make(chan IdxRes[T])
					}

					// update the queue with this extension call.
					addExtCallToQ(&extQ, resChan, nextProm.syncChan, idx)
				}

				// send the updated queue back for either another extension call,
				// or to be included in the currProm's resolving logic.
				extsChan <- extQ
			}
		}

		// if no promise was resolved synchronously, retry with another one.
		if res.Result == nil {
			continue
		}

		// a promise was resolved synchronously, update the result fields..
		// add the result to the final promise's [Result].
		callRes = append(callRes, res)

		// get the final promise's [State] based on  the current call.
		callResState = op.NextState(res.State(), callResState)

		// stop, if we found the target [State] based on the current call.
		if op.ReturnOnTargetState() && op.IsTargetState(res.State()) {
			break
		}
	}

	// no resolved promises, or the resolved promise(s) didn't meet the requirements
	// set by the provided flags.
	if callResState == unknown || !op.ReturnOnTargetState() ||
		!op.IsTargetState(callResState) {
		// no early return is requested, no promises got resolved by the wait
		// logic above, or the resolved promise(s) didn't meet the requirements
		// set for this call...
		// get the number of pending promises against the initially provided list.
		pending := len(ps) - len(callRes)

		// if there are no pending promises and no result state computed, then it
		// must be a Join call, which means the result state expected is Success.
		if pending == 0 && callResState == unknown {
			callResState = Success
		}

		// otherwise, wait until a matching result from the pending promises.
		for i := 0; i < pending; i++ {
			res := <-resChan
			callRes = append(callRes, res)
			callResState = op.NextState(res.State(), callResState)
			if op.ReturnOnTargetState() && op.IsTargetState(res.State()) {
				break
			}
		}
	}

	// resolve the next promise as expected, based on the final callResState.
	nextProm.resolveToRes(newMultiRes(callResState, callRes))
}

func addExtCallToQ[T any](
	q *extQueue[T],
	resChan chan IdxRes[T],
	syncChan <-chan struct{},
	idx int,
) {
	call := extCall[T]{
		resChan:  resChan,
		syncChan: syncChan,
		idx:      idx,
	}
	if !q.valid {
		q.valid = true
		q.call = call
	} else {
		q.extra = append(q.extra, call)
	}
}
