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

// Select returns a Promise value that resolves to the first Promise that's
// resolved from the Promise values passed.
// It doesn't wait for all passed Promise values to resolve.
// The resulting IdxRes value holds the Result value of the resolved Promise.
// The original order of the IdxRes's Promise can be retrieved from its Idx field.
func Select[T any](p ...*Promise[T]) *Promise[IdxRes[T]] {
	return selectCall[T](p...)
}

func selectCall[T any](p ...*Promise[T]) *Promise[IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[IdxRes[T]](nil)
	}

	nextProm := newPromInter[IdxRes[T]](nil)
	go selectHandler(nextProm, p)
	return nextProm
}

func selectHandler[T any](
	newProm *Promise[IdxRes[T]],
	ps []*Promise[T],
) {
	// resChan is populated lazily, only if it's needed.
	var resChan chan IdxRes[T]

	// res represent the resolve result.
	res := IdxRes[T]{}

	// loopCnt records how many iterations happened in the loop below
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

		if !blocking {
			select {
			case <-currProm.syncCtx.Done():
				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			default:
				// This would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// Re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		} else {
			extsChan := currProm.extsChan()
			select {
			case <-currProm.syncCtx.Done():
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			case extQ := <-extsChan:
				// make sure to check the state again and get the sync result,
				// in case the promise is resolved...
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
					addExtCallToQ(&extQ, resChan, newProm.syncCtx.Done(), idx)
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

	// because this is a Select extension call, only one result is expected
	if res.Result == nil {
		res = <-resChan
	}

	// resolve the next promise, based on the final Result got
	switch res.State() {
	case Panicked:
		newProm.resolveToPanickedRes(panickedResultSingleRes[T, IdxRes[T]]{res})
	case Rejected:
		newProm.resolveToRejectedRes(rejectedResultSingleRes[T, IdxRes[T]]{res})
	case Fulfilled:
		newProm.resolveToFulfilledRes(fulfilledResultSingleRes[T, IdxRes[T]]{res})
	}
}

// All returns a Promise value that resolves to Fulfilled if all Promise values
// passed resolved to Fulfilled.
// It resolves to Panicked if at least one resolved to Panicked.
// It resolves to Rejected if at least one resolved to Rejected and none resolves
// to Panicked.
//
// It doesn't wait for all passed Promise values to resolve, unless the resolved
// one(s) is Panicked.
//
// The resulting IdxRes slice holds the Result values of the Promise values passed
// up to when the returned Promise was resolved.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func All[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	return allCall(false, p)
}

// AllWait returns a Promise value that resolves to Fulfilled if all Promise values
// passed resolved to Fulfilled.
// It resolves to Panicked if at least one resolved to Panicked.
// It resolves to Rejected if at least one resolved to Rejected and none resolves
// to Panicked.
//
// It waits for all passed Promise values to resolve.
//
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func AllWait[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	return allCall(true, p)
}

func allCall[T any](waitAll bool, ps []*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(ps) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, ps, waitAll, true, false)
	return nextProm
}

// Any returns a Promise value that resolves to Fulfilled if at least one of
// the Promise values passed resolves to Fulfilled and none resolves to Panicked.
// It resolves to Panicked if at least one resolved to Panicked.
// It resolves to Rejected if all resolves to Rejected.
//
// It doesn't wait for all passed Promise values to resolve, unless the resolved
// one(s) is Panicked.
//
// The resulting IdxRes slice holds the Result values of the Promise values passed
// up to when the returned Promise was resolved.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func Any[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	return anyCall(false, p)
}

// AnyWait returns a Promise value that resolves to Fulfilled if at least one of
// the Promise values passed resolves to Fulfilled and none resolves to Panicked.
// It resolves to Panicked if at least one resolved to Panicked.
// It resolves to Rejected if all resolves to Rejected.
//
// It waits for all passed Promise values to resolve.
//
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func AnyWait[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	return anyCall(true, p)
}

func anyCall[T any](waitAll bool, ps []*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(ps) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, ps, waitAll, false, true)
	return nextProm
}

// Join returns a Promise value that resolves to Fulfilled after all Promise
// values passed resolves.
//
// It waits for all passed Promise values to resolve.
//
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func Join[T any](p ...*Promise[T]) *Promise[[]IdxRes[T]] {
	return joinCall(p)
}

func joinCall[T any](ps []*Promise[T]) *Promise[[]IdxRes[T]] {
	if len(ps) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, ps, true, false, false)
	return nextProm
}

func joinHandler[T any](
	newProm *Promise[[]IdxRes[T]],
	ps []*Promise[T],
	waitAll bool,
	allSuccess bool,
	anySuccess bool,
) {
	// resChan is populated lazily, only if it's needed.
	var resChan chan IdxRes[T]

	// resArr and resState, collectively, represent the resolve result.
	resArr := make([]IdxRes[T], 0, len(ps))
	resState := unknown

	// loopCnt records how many iterations happened in the loop below
	var loopCnt int

	// randIdx responsible for returning a random, unique, index in the provided
	// list of promises.
	var randIdx uniquerand.Int
	randIdx.Reset(len(ps))

	// try to find a suitable resolved promise, based on the provided flags,
	// or arrange for a notification once a promise is resolved.
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

		var res IdxRes[T]
		if !blocking {
			select {
			case <-currProm.syncCtx.Done():
				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			default:
				// this would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		} else {
			extsChan := currProm.extsChan()
			select {
			case <-currProm.syncCtx.Done():
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
			case extQ := <-extsChan:
				// make sure to check the state again and get the sync result,
				// in case the promise is resolved...
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
					addExtCallToQ(&extQ, resChan, newProm.syncCtx.Done(), idx)
				}

				// send the updated queue back for either another extension call,
				// or to be included in the currProm's resolving logic.
				extsChan <- extQ
			}
		}

		// if the promise was resolved synchronously, update the result fields.
		if res.Result != nil {
			// add it to the result array.
			resArr = append(resArr, res)

			// get the final promise's state, based on the previous resState and
			// the recent resolved promise's state, using the selected mode rules.
			if allSuccess {
				resState = getAllResState(res.State(), resState)

				// stop, if we found the target break state based on the current flags.
				// note: for the allSuccess case, we can only continue if the waitAll
				// flag is not set, or the recent resolved promise got resolved to
				// anything but Rejected.
				// note: by default, we won't break on Panicked states, as it will be
				// used only to alter the final state.
				if !waitAll && res.State() == Rejected {
					break loop
				}
			}
			if anySuccess {
				resState = getAnyResState(res.State(), resState)

				if !waitAll && res.State() == Fulfilled {
					break loop
				}
			}
		}
	}

	// no resolved promises, or the resolved promise(s) didn't meet the requirements
	// set by the provided flags.
	if waitAll || resState == unknown ||
		(allSuccess && resState == Fulfilled) ||
		(anySuccess && resState != Fulfilled) {
		// the waitAll flag is set, no promise got resolved by the wait logic above,
		// or the resolved promise(s) didn't meet the requirements set by the flags...
		// get the number of pending promises against the initially provided list.
		pending := len(ps) - len(resArr)

		// if there are no pending promises and no result state computed, then it
		// must be a Join call, which means the result state expected is Fulfilled.
		if pending == 0 && resState == unknown {
			resState = Fulfilled
		}

		// otherwise, wait until a matching result is received.
		if pending != 0 {
			for i := 0; i < pending; i++ {
				res := <-resChan
				resArr = append(resArr, res)

				// get the final promise's state, based on the previous resState and
				// the recent resolved promise's state, using the selected mode rules.
				if allSuccess {
					resState = getAllResState(res.State(), resState)

					// stop, if we found the target break state based on the current flags.
					// note: for the allSuccess case, we can only continue if the waitAll
					// flag is not set, or the recent resolved promise got resolved to
					// anything but Rejected.
					// note: by default, we won't break on Panicked states, as it will be
					// used only to alter the final state.
					if !waitAll && res.State() == Rejected {
						break
					}
				}
				if anySuccess {
					resState = getAnyResState(res.State(), resState)

					if !waitAll && res.State() == Fulfilled {
						break
					}
				}

				if !allSuccess && !anySuccess {
					resState = Fulfilled
				}
			}
		}
	}

	// resolve the next promise as expected, based on the final resState.
	switch resState {
	case Panicked:
		newProm.resolveToPanickedRes(panickedResultMultiRes[T, IdxRes[T]]{resArr})
	case Rejected:
		newProm.resolveToRejectedRes(rejectedResultMultiRes[T, IdxRes[T]]{resArr})
	case Fulfilled:
		newProm.resolveToFulfilledRes(fulfilledResultMultiRes[T, IdxRes[T]]{resArr})
	}
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

// getAllResState returns the resolve state of the promise returned by All.
// Panicked has the highest priority.
// Rejected has the highest priority between Fulfilled and Rejected.
func getAllResState(newState, prevState State) State {
	switch {
	case newState == Panicked || prevState == Panicked:
		return Panicked
	case newState == Rejected:
		return Rejected
	case prevState == unknown:
		return newState
	default:
		return prevState
	}
}

// getAnyResState returns the resolve state of the promise returned by Any.
// Panicked has the highest priority.
// Fulfilled has the highest priority between Fulfilled and Rejected.
func getAnyResState(newState, prevState State) State {
	switch {
	case newState == Panicked || prevState == Panicked:
		return Panicked
	case newState == Fulfilled:
		return Fulfilled
	case prevState == unknown:
		return newState
	default:
		return prevState
	}
}
