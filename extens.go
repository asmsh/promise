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
	"github.com/asmsh/promise/internal/uniquerand"
	"log"
	"os"
)

type logger struct {
	enabled bool
	l       *log.Logger
}

func (l logger) Println(v ...any) {
	if !l.enabled {
		return
	}
	l.l.Println(v...)
}

var logr = logger{
	enabled: false,
	l: func() *log.Logger {
		return log.New(os.Stderr, "", log.Lmicroseconds)
	}(),
}

// Select returns a Promise value that resolves to the first Promise that's
// resolved from the Promise values passed.
// It doesn't wait for all passed Promise values to resolve.
// The resulting IdxRes value holds the Result value of the resolved Promise.
// The original order of the IdxRes's Promise can be retrieved from its Idx field.
func Select[T any](p ...Promise[T]) Promise[IdxRes[T]] {
	return selectCall[T](p...)
}

func selectCall[T any](p ...Promise[T]) Promise[IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[IdxRes[T]](nil)
	}

	nextProm := newPromInter[IdxRes[T]](nil)
	go selectHandler(nextProm, p)
	return nextProm
}

func selectHandler[T any](
	nextProm *genericPromise[IdxRes[T]],
	p []Promise[T],
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
	randIdx.Reset(len(p))

loop:
	for idx, ok := randIdx.Get(); ok; idx, ok = randIdx.Get() {
		currProm := p[idx].impl()
		loopCnt++

		logr.Println("idx", idx, "loopCnt", loopCnt, "of length", len(p))

		// Select with non-blocking or with blocking, based on whether we might be
		// interested to check other promises for potential immediate resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(p)

		if blocking {
			logr.Println("blocking block")

			select {
			case <-currProm.syncChan:
				logr.Println("blocking block syncChan case")

				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
				break loop
			case extQ := <-currProm.extsChan:
				logr.Println("blocking block extQ case")

				// the promise is not resolved yet...
				// create the res chan if it's not already created.
				if resChan == nil {
					resChan = make(chan IdxRes[T])
				}
				// update the queue with this extension call.
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				// send the updated queue back for either another extension call,
				// or the currProm's resolving logic.
				currProm.extsChan <- extQ
			}
		} else {
			logr.Println("non-blocking block")

			select {
			case <-currProm.syncChan:
				logr.Println("non-blocking block syncChan case")

				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
				break loop
			case extQ := <-currProm.extsChan:
				logr.Println("non-blocking block extQ case")

				if resChan == nil {
					resChan = make(chan IdxRes[T])
				}
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				currProm.extsChan <- extQ
			default:
				logr.Println("non-blocking block default case")

				// This would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// Re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		}
	}

	// because this is a Select extension call, only one result is expected
	if res.Result == nil {
		logr.Println("blocking on resChan receive")

		res = <-resChan
	}

	// resolve the next promise, based on the final Result got
	switch res.State() {
	case Panicked:
		logr.Println("idx", res.Idx, "Panicked")

		res := panickedResultSingleIdxRes[T]{res}
		resolveToPanickedRes(nextProm, res)
	case Rejected:
		logr.Println("idx", res.Idx, "Rejected")

		res := rejectedResultSingleIdxRes[T]{res}
		resolveToRejectedRes(nextProm, res)
	case Fulfilled:
		logr.Println("idx", res.Idx, "Fulfilled")

		res := fulfilledResultSingleIdxRes[T]{res}
		resolveToFulfilledRes(nextProm, res)
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
func All[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
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
func AllWait[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
	return allCall(true, p)
}

func allCall[T any](waitAll bool, p []Promise[T]) Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, waitAll, true, false)
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
func Any[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
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
func AnyWait[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
	return anyCall(true, p)
}

func anyCall[T any](waitAll bool, p []Promise[T]) Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, waitAll, false, true)
	return nextProm
}

// Join returns a Promise value that resolves to Fulfilled after all Promise
// values passed resolves.
//
// It waits for all passed Promise values to resolve.
//
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func Join[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
	return joinCall(p)
}

func joinCall[T any](p []Promise[T]) Promise[[]IdxRes[T]] {
	if len(p) == 0 {
		return Wrap[[]IdxRes[T]](nil)
	}

	nextProm := newPromInter[[]IdxRes[T]](nil)
	go joinHandler(nextProm, p, true, false, false)
	return nextProm
}

func joinHandler[T any](
	nextProm *genericPromise[[]IdxRes[T]],
	p []Promise[T],
	waitAll bool,
	allSuccess bool,
	anySuccess bool,
) {
	// resChan is populated lazily, only if it's needed.
	var resChan chan IdxRes[T]

	// resArr and resState, collectively, represent the resolve result.
	resArr := make([]IdxRes[T], 0, len(p))
	resState := unknown

	// loopCnt records how many iterations happened in the loop below
	var loopCnt int

	// randIdx responsible for returning a random, unique, index in the provided
	// list of promises.
	var randIdx uniquerand.Int
	randIdx.Reset(len(p))

	logr.Println(
		"join with: waitAll",
		waitAll,
		"allSuccess",
		allSuccess,
		"anySuccess",
		anySuccess,
		"resState",
		resState,
	)

loop:
	for idx, ok := randIdx.Get(); ok; idx, ok = randIdx.Get() {
		currProm := p[idx].impl()
		loopCnt++

		logr.Println("loop with: idx", idx, "loopCnt", loopCnt, "of length", len(p))

		// Select with non-blocking or with blocking, based on whether we might be
		// interested to check other promises for potential immediate resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(p)

		if !blocking {
			logr.Println("non-blocking block")

			select {
			case <-currProm.syncChan:
				logr.Println("non-blocking block syncChan case")

				res := IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}

				logr.Println(
					"non-blocking block syncChan case with res.state",
					res.State(),
					"resState",
					resState,
				)

				resArr = append(resArr, res)
				resState = Fulfilled
				if allSuccess {
					newResState := getAllResState(res.State(), resState)

					logr.Println(
						"non-blocking block syncChan allSuccess prevResState",
						resState,
						"newResState",
						newResState,
					)

					resState = newResState
					if !waitAll && res.State() == Rejected {
						logr.Println("non-blocking block syncChan allSuccess break")

						break loop
					}
				}
				if anySuccess {
					newResState := getAnyResState(res.State(), resState)

					logr.Println(
						"non-blocking block syncChan anySuccess prevResState",
						resState,
						"newResState",
						newResState,
					)

					resState = newResState
					if !waitAll && res.State() == Fulfilled {
						logr.Println("non-blocking block syncChan anySuccess break")

						break loop
					}
				}
			case extQ := <-currProm.extsChan:
				logr.Println("non-blocking block extQ case")

				if resChan == nil {
					logr.Println("non-blocking block extQ case new chan")

					resChan = make(chan IdxRes[T])
				}

				logr.Println(
					"non-blocking block extQ case prev queue valid",
					extQ.valid,
					"len",
					len(extQ.extra),
				)
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				logr.Println(
					"non-blocking block extQ case prev queue valid",
					extQ.valid,
					"len",
					len(extQ.extra),
				)

				currProm.extsChan <- extQ
			default:
				logr.Println("non-blocking block default case")

				// This would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// Re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		} else {
			logr.Println("blocking block")

			select {
			case <-currProm.syncChan:
				logr.Println("blocking block syncChan case")

				// the promise is resolved...
				// create a result value based on the current promise.
				res := IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}

				logr.Println(
					"blocking block syncChan case with res.state",
					res.State(),
					"resState",
					resState,
				)

				// update the resState to one with the lower priority,
				// and add the new result to the result array.
				resArr = append(resArr, res)
				resState = Fulfilled
				if allSuccess {
					// update the final state, based on the previous and new ones.
					newResState := getAllResState(res.State(), resState)

					logr.Println(
						"blocking block syncChan allSuccess prevResState",
						resState,
						"newResState",
						newResState,
					)

					resState = newResState

					// the new state is Rejected, and it's requested not to wait all, break.
					// note: by default, we won't break on Panicked states, as it will be used
					// only to alter the final state.
					if !waitAll && res.State() == Rejected {
						logr.Println("blocking block syncChan allSuccess break")

						break loop
					}
				}
				if anySuccess {
					// update the final state, based on the previous and new ones.
					newResState := getAnyResState(res.State(), resState)

					logr.Println(
						"blocking block syncChan anySuccess prevResState",
						resState,
						"newResState",
						newResState,
					)

					resState = newResState

					// the new state is Fulfilled, and it's requested not to wait all, break.
					if !waitAll && res.State() == Fulfilled {
						logr.Println("blocking block syncChan anySuccess break")

						break loop
					}
				}
			case extQ := <-currProm.extsChan:
				logr.Println("blocking block extQ case")

				// the promise is not resolved yet...
				// create the res chan if it's not already created.
				if resChan == nil {
					logr.Println("blocking block extQ case new chan")

					resChan = make(chan IdxRes[T])
				}

				// update the queue with this extension call.
				logr.Println(
					"blocking block extQ case prev queue valid",
					extQ.valid,
					"len",
					len(extQ.extra),
				)
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				logr.Println(
					"blocking block extQ case new queue valid",
					extQ.valid,
					"len",
					len(extQ.extra),
				)

				// send the updated queue back for either another extension call,
				// or the currProm's resolving logic.
				currProm.extsChan <- extQ
			}
		}
	}

	if waitAll ||
		(allSuccess && resState == Fulfilled) ||
		(anySuccess && resState != Fulfilled) ||
		resState == unknown {
		pending := len(p) - len(resArr)
		logr.Println("pending promises", pending)

		if pending != 0 {
			logr.Println("waiting for pending promises", pending)

			for i := 0; i < pending; i++ {
				res := <-resChan
				logr.Println(
					"waiting block with res.state",
					res.State(),
					"resState",
					resState,
				)

				resArr = append(resArr, res)
				if allSuccess {
					// if it's requested to return on the first failure, break.
					newResState := getAllResState(res.State(), resState)

					logr.Println(
						"waiting block allSuccess res.state",
						res.State(),
						"prevResState",
						resState,
						"newResState",
						newResState,
					)

					resState = newResState
					if !waitAll && res.State() == Rejected {
						logr.Println("waiting block allSuccess break")

						break
					}
				}
				if anySuccess {
					// if it's requested to return on the first failure, break.
					newResState := getAnyResState(res.State(), resState)

					logr.Println(
						"waiting block anySuccess res.state",
						res.State(),
						"prevResState",
						resState,
						resState,
						"newResState",
						newResState,
					)

					resState = newResState
					if !waitAll && res.State() == Fulfilled {
						logr.Println("waiting block anySuccess break")

						break
					}
				}

				if !allSuccess && !anySuccess {
					resState = Fulfilled
				}
			}
		}
	}

	logr.Println("resolving to", resState)

	// resolve the next promise as expected, based on the resState.
	switch resState {
	case Panicked:
		res := panickedResultMultiIdxRes[T]{resArr}
		resolveToPanickedRes(nextProm, res)
	case Rejected:
		res := rejectedResultMultiIdxRes[T]{resArr}
		resolveToRejectedRes(nextProm, res)
	case Fulfilled:
		res := fulfilledResultMultiIdxRes[T]{resArr}
		resolveToFulfilledRes(nextProm, res)
	}
}

func updateExtQCall[T any](
	q *extQueue[T],
	idx int,
	resChan chan IdxRes[T],
	syncChan chan struct{},
) {
	call := extCall[T]{
		idx:      idx,
		resChan:  resChan,
		syncChan: syncChan,
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
