package promise

import (
	"fmt"

	"github.com/asmsh/promise/internal/uniquerand"
)

// IdxRes is a positional result view, that represents the result of the promise
// at index idx in the original list provided.
type IdxRes[T any] struct {
	Idx int
	Result[T]
}

func (ir IdxRes[T]) String() string {
	return fmt.Sprintf("[%d]%v", ir.Idx, ir.Result)
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

		// Select with non-blocking or with blocking, based on whether we might be
		// interested to check other promises for potential immediate resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(p)

		if blocking {
			select {
			case <-currProm.syncChan:
				// the promise is resolved...
				// create a result value based on the current promise.
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
				break loop
			case extQ := <-currProm.extsChan:
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
			select {
			case <-currProm.syncChan:
				res = IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
				break loop
			case extQ := <-currProm.extsChan:
				if resChan == nil {
					resChan = make(chan IdxRes[T])
				}
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				currProm.extsChan <- extQ
			default:
				// This would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// Re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		}
	}

	// because this is a Select extension call, only one result is expected
	if res.Result == nil {
		res = <-resChan
	}

	// resolve the next promise, based on the final Result got
	switch res.State() {
	case Panicked:
		// TODO: we should pass the error correctly, as IdxErr (??), in ALL panics,
		//  so the result would be that the Recover callback must do a type-cast.
		res := errPromisePanickedResult[IdxRes[T]]{v: getPanicVFromRes(res.Result)}
		resolveToPanickedRes[IdxRes[T]](nextProm, res)
	case Rejected:
		resolveToRejectedRes(nextProm, ValErr[IdxRes[T]](res, res.Err()))
	case Fulfilled:
		resolveToFulfilledRes(nextProm, Val[IdxRes[T]](res))
	}
}

// All returns a Promise value that resolves to Fulfilled if all Promise values
// passed resolved to Fulfilled, resolves to Rejected or to Panicked if at least
// one resolved to Rejected or to Panicked, respectively.
// It doesn't wait for all passed Promise values to resolve.
// The resulting IdxRes slice holds the Result values of the Promise values passed
// up to when the returned Promise was resolved.
// The order of the resulting IdxRes slice's elements is the order of resolving,
// which doesn't have to match the order of the passed Promise values.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
// Between resolving the returned Promise value to either Rejected or Panicked,
// Panicked will always take precedence.
func All[T any](p ...Promise[T]) Promise[[]IdxRes[T]] {
	return allCall(false, p)
}

// AllWait returns a Promise value that resolves to Fulfilled if all Promise values
// passed resolved to Fulfilled, resolves to Rejected or to Panicked if at least
// one resolved to Rejected or to Panicked, respectively.
// It waits for all passed Promise values to resolve.
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The order of the resulting IdxRes slice's elements is the order of resolving,
// which doesn't have to match the order of the passed Promise values.
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

// Any returns a Promise value that resolves to Fulfilled if at least of the
// Promise values passed resolved to Fulfilled, resolves to Rejected if all
// resolved to Rejected, or resolves to Panicked if at least one resolved to
// Panicked.
// It doesn't wait for all passed Promise values to resolve.
// The resulting IdxRes slice holds the Result values of the Promise values passed
// up to when the returned Promise was resolved.
// The order of the resulting IdxRes slice's elements is the order of resolving,
// which doesn't have to match the order of the passed Promise values.
// The original order of any IdxRes's Promise can be retrieved from its Idx field.
func Any[T any](waitAll bool, p ...Promise[T]) Promise[[]IdxRes[T]] {
	return anyCall(waitAll, p)
}

// AnyWait returns a Promise value that resolves to Fulfilled if at least of the
// Promise values passed resolved to Fulfilled, resolves to Rejected if all
// resolved to Rejected, or resolves to Panicked if at least one resolved to
// Panicked.
// It waits for all passed Promise values to resolve.
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The order of the resulting IdxRes slice's elements is the order of resolving,
// which doesn't have to match the order of the passed Promise values.
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
// It waits for all passed Promise values to resolve.
// The resulting IdxRes slice holds the Result values of all Promise values passed.
// The order of the resulting IdxRes slice's elements is the order of resolving,
// which doesn't have to match the order of the passed Promise values.
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
	resState := State(0)

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

		// Select with non-blocking or with blocking, based on whether we might be
		// interested to check other promises for potential immediate resolution.
		// Only do that if we haven't already looped over the list before, otherwise
		// we might end up in an (almost) infinite loop.
		// Non-blocking gives us the benefit of catching other possibly resolved
		// promises, without being stuck(blocked) on the first one we encounter.
		blocking := loopCnt > len(p)

		if blocking {
			select {
			case <-currProm.syncChan:
				// the promise is resolved...
				// create a result value based on the current promise.
				res := IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}

				// update the resState to one with the lower priority,
				// and add the new result to the result array.
				resArr = append(resArr, res)
				resState = Fulfilled
				if allSuccess {
					// if it's requested to return on the first failure, break.
					resState = getAllResState(res.State(), resState)
					if !waitAll && resState != Fulfilled {
						break loop
					}
				}
				if anySuccess {
					// if it's requested to return on the first failure, break.
					resState = getAnyResState(res.State(), resState)
					if !waitAll && resState == Fulfilled {
						break loop
					}
				}
			case extQ := <-currProm.extsChan:
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
			select {
			case <-currProm.syncChan:
				res := IdxRes[T]{
					Idx:    idx,
					Result: getFinalRes(currProm.res),
				}
				resArr = append(resArr, res)
				resState = Fulfilled
				if allSuccess {
					resState = getAllResState(res.State(), resState)
					if !waitAll && resState != Fulfilled {
						break loop
					}
				}
				if anySuccess {
					resState = getAnyResState(res.State(), resState)
					if !waitAll && resState == Fulfilled {
						break loop
					}
				}
			case extQ := <-currProm.extsChan:
				if resChan == nil {
					resChan = make(chan IdxRes[T])
				}
				updateExtQCall(&extQ, idx, resChan, nextProm.syncChan)
				currProm.extsChan <- extQ
			default:
				// This would happen when the promise is not resolved yet, and there's
				// another extension call that owns the extQueue value.
				// Re-put the index in the randIdx source, to re-visit this case later.
				randIdx.Put(idx)
			}
		}
	}

	if waitAll ||
		(allSuccess && resState == Fulfilled) ||
		(anySuccess && resState != Fulfilled) {
		pending := len(p) - len(resArr)
		if pending != 0 {

			for i := 0; i < pending; i++ {
				res := <-resChan
				resArr = append(resArr, res)
				if allSuccess {
					// if it's requested to return on the first failure, break.
					resState = getAllResState(res.State(), resState)
					if !waitAll && resState != Fulfilled {
						break
					}
				}
				if anySuccess {
					// if it's requested to return on the first failure, break.
					resState = getAnyResState(res.State(), resState)
					if !waitAll && resState == Fulfilled {
						break
					}
				}
			}
		}
	}

	// resolve the next promise as expected, based on the resState.
	switch resState {
	case Panicked:
		res := errPromisePanickedIdxResult[T]{resArr}
		resolveToPanickedRes[[]IdxRes[T]](nextProm, res)
	case Rejected:
		res := errPromiseRejectedIdxResult[T]{resArr}
		resolveToRejectedRes[[]IdxRes[T]](nextProm, res)
	case Fulfilled:
		resolveToFulfilledRes(nextProm, Val[[]IdxRes[T]](resArr))
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
func getAllResState(newState, prevState State) State {
	if newState > prevState {
		return newState
	}
	return prevState
}

// getAnyResState returns the resolve state of the promise returned by Any.
// Fulfilled has the highest priority.
// Panicked has the highest priority between Panicked and Rejected.
func getAnyResState(newState, prevState State) State {
	if newState == Fulfilled || prevState == Fulfilled {
		return Fulfilled
	}
	return getAllResState(newState, prevState)
}
