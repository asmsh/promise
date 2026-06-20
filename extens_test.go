// Copyright 2024 Ahmad Sameh(asmsh)
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
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asmsh/uniquerand/v2"
)

// used as a randomizer in compileTestCase.
var closedChan chan struct{}

func init() {
	closedChan = make(chan struct{})
	close(closedChan)
}

func getBenchmarkAsyncPromise(
	g *Group[string],
	d time.Duration,
	cnt *atomic.Int64,
	wg *sync.WaitGroup,
) *Promise[string] {
	return g.Go(func() {
		wg.Done()
		defer cnt.Add(1)
		time.Sleep(d)
	})
}

func getBenchmarkSyncPromise(g *Group[string]) *Promise[string] {
	return g.Wrap(nil)
}

func createTestAsyncResString(id int, state State, d time.Duration) string {
	return fmt.Sprintf("id#%d test async %s: %s", id, state, d)
}

func createTestSyncResString(id int, state State) string {
	return fmt.Sprintf("id#%d test sync %s", id, state)
}

type generatedResult struct {
	idx   int // local index to the respective list.
	id    int
	state State
	delay time.Duration // will be 0 for sync Result.
}

func (gr generatedResult) String() string {
	if gr.delay == 0 {
		return createTestSyncResString(gr.id, gr.state)
	}
	return createTestAsyncResString(gr.id, gr.state, gr.delay)
}

func isEqualResult[TRes extRes](got, want Result[TRes]) bool {
	eqState := got.State() == want.State()

	// compare the err and val using their string representation...
	eqErr := true
	if gotErr, wantErr := got.Err(), want.Err(); gotErr != nil || wantErr != nil {
		gotErrStr := fmt.Sprintf("%s", got.Err())
		wantErrStr := fmt.Sprintf("%s", want.Err())
		eqErr = gotErrStr == wantErrStr
	}

	gotValStr := fmt.Sprintf("%s", got.Val())
	wantValStr := fmt.Sprintf("%s", want.Val())
	eqVal := gotValStr == wantValStr

	return eqState && eqErr && eqVal
}

func getTestAsyncSuccessPromise(
	g *Group[string],
	id int,
	d time.Duration,
	cnt *atomic.Int64,
	wg *sync.WaitGroup,
) *Promise[string] {
	return g.GoValErr(func() (string, error) {
		wg.Done()
		defer cnt.Add(1)
		time.Sleep(d)
		return createTestAsyncResString(id, Success, d), nil
	})
}

func getTestAsyncErrorPromise(
	g *Group[string],
	id int,
	d time.Duration,
	cnt *atomic.Int64,
	wg *sync.WaitGroup,
) *Promise[string] {
	return g.GoValErr(func() (string, error) {
		wg.Done()
		defer cnt.Add(1)
		time.Sleep(d)
		return "", errors.New(createTestAsyncResString(id, Error, d))
	})
}

func getTestAsyncPanicPromise(
	g *Group[string],
	id int,
	d time.Duration,
	cnt *atomic.Int64,
	wg *sync.WaitGroup,
) *Promise[string] {
	return g.GoValErr(func() (string, error) {
		wg.Done()
		defer cnt.Add(1)
		time.Sleep(d)
		panic(errors.New(createTestAsyncResString(id, Panic, d)))
	})
}

func getTestSyncSuccessPromise(g *Group[string], id int) *Promise[string] {
	return g.Wrap(ValRes(createTestSyncResString(id, Success)))
}

func getTestSyncErrorPromise(g *Group[string], id int) *Promise[string] {
	return g.Wrap(ErrRes[string](errors.New(createTestSyncResString(id, Error))))
}

func getTestSyncPanicPromise(g *Group[string], id int) *Promise[string] {
	return g.Wrap(PanicRes[string](errors.New(createTestSyncResString(id, Panic))))
}

// helper types for tests and benchmarks...
type singleIdxRes = IdxRes[string]
type multiIdxRes = []IdxRes[string]

type extRes interface {
	singleIdxRes | multiIdxRes
}

type waitFunc func(...*Promise[string])
type idxFunc[TRes extRes] func(...*Promise[string]) *Promise[TRes]

type singleIdxFunc = idxFunc[singleIdxRes]
type multiIdxFunc = idxFunc[multiIdxRes]

type extFuncs[TRes extRes] interface {
	waitFunc | idxFunc[TRes]
}

// caseConfig is the definition of a test or benchmark case.
// it needs to be compiled to a [testCase] using [compileTestCase].
type caseConfig struct {
	name string

	async generateAsyncPromisesConfig
	sync  generateSyncPromisesConfig

	extRes extensionsResult
}

func (c caseConfig) totalResultSize() int {
	return c.async.totalResultSize() + c.sync.totalResultSize()
}

type generateAsyncPromisesConfig struct {
	// totalNum is the total number of randomly generated async promises
	totalNum int

	// if any of the below is provided, the totalNum is ignored.
	successNum      int
	successDelayNum int
	errorNum        int
	errorDelayNum   int
	panicNum        int
	panicDelayNum   int

	// simulated work (time.Sleep) params, for normal and delayed promises.
	// the actual number will be a random value in [minWait, maxWait).
	// if both number are specified and non-zero, it will be an exact wait.
	minWait        time.Duration
	maxWait        time.Duration
	minDelayedWait time.Duration
	maxDelayedWait time.Duration

	// useUniqueDur will be set if we want to track which time.Duration
	// random values have been used, so that the values are unique, which
	// allows ordering by resolution delay.
	useUniqueDur     bool
	uniqueDur        *uniquerand.N[time.Duration]
	delayedUniqueDur *uniquerand.N[time.Duration]
}

func (g generateAsyncPromisesConfig) totalResultSize() int {
	resultSize := g.resultSize()
	delayedResultSize := g.delayedResultSize()

	if totalResult := resultSize + delayedResultSize; totalResult != 0 {
		return totalResult
	}

	return g.totalNum
}

func (g generateAsyncPromisesConfig) resultSize() int {
	return g.successNum + g.errorNum + g.panicNum
}

func (g generateAsyncPromisesConfig) delayedResultSize() int {
	return g.successDelayNum + g.errorDelayNum + g.panicDelayNum
}

func (g *generateAsyncPromisesConfig) validate() {
	// if any of the specific values is provided, the totalNum must be 0
	if g.resultSize() != 0 || g.delayedResultSize() != 0 {
		if g.totalNum != 0 {
			panic(fmt.Sprintf("bad generateAsyncPromisesConfig: unexpectedly totalNum=%d", g.totalNum))
		}
	}

	// if the there's wait params, validate their values.
	if g.minWait != 0 || g.maxWait != 0 {
		if g.minWait > g.maxWait {
			panic(fmt.Sprintf("bad wait params: minWait=%s, maxWait=%s", g.minWait, g.maxWait))
		}
	}

	// if the there's delayed wait params, validate their values.
	if g.minDelayedWait != 0 || g.maxDelayedWait != 0 {
		if g.minDelayedWait > g.maxDelayedWait {
			panic(fmt.Sprintf("bad delayed wait params: minDelayedWait=%s, maxDelayedWait=%s", g.minDelayedWait, g.maxDelayedWait))
		}
	}

	if g.useUniqueDur {
		g.uniqueDur = uniquerand.NewN(uniquerand.Config[time.Duration]{
			// default range of 200ms, with 5ms step.
			Min:  cmp.Or(g.minWait, 1*time.Millisecond),
			Max:  cmp.Or(g.maxWait, 201*time.Millisecond),
			Step: 5 * time.Millisecond,
		})

		g.delayedUniqueDur = uniquerand.NewN(uniquerand.Config[time.Duration]{
			// default range of 200ms, with 5ms step.
			Min:  cmp.Or(g.minDelayedWait, 1000*time.Millisecond),
			Max:  cmp.Or(g.maxDelayedWait, 1200*time.Millisecond),
			Step: 5 * time.Millisecond,
		})

		if g.totalNum != 0 {
			if g.uniqueDur.Cap() < g.totalNum {
				panic(fmt.Sprintf("bad unique wait params: minWait=%s, maxWait=%s", g.minWait, g.maxWait))
			}
			if g.delayedUniqueDur.Cap() < g.totalNum {
				panic(fmt.Sprintf("bad unique delayed wait params: minDelayedWait=%s, maxDelayedWait=%s", g.minDelayedWait, g.maxDelayedWait))
			}
		} else {
			if g.uniqueDur.Cap() < g.resultSize() {
				panic(fmt.Sprintf("bad unique wait params: minWait=%s, maxWait=%s", g.minWait, g.maxWait))
			}
			if g.delayedUniqueDur.Cap() < g.delayedResultSize() {
				panic(fmt.Sprintf("bad unique delayed wait params: minDelayedWait=%s, maxDelayedWait=%s", g.minDelayedWait, g.maxDelayedWait))
			}
		}
	}
}

type generateSyncPromisesConfig struct {
	// totalNum is the total number of randomly generated sync promises
	totalNum int

	// if any of the below is provided, the totalNum is ignored.
	successNum int
	errorNum   int
	panicNum   int
}

func (g generateSyncPromisesConfig) totalResultSize() int {
	if g.successNum != 0 || g.errorNum != 0 || g.panicNum != 0 {
		return g.successNum + g.errorNum + g.panicNum
	}
	return g.totalNum
}

func (g generateSyncPromisesConfig) validate() {
	// if any of the specific values is provided, the totalNum must be 0
	if g.successNum != 0 || g.errorNum != 0 || g.panicNum != 0 {
		if g.totalNum != 0 {
			panic(fmt.Sprintf("bad generateSyncPromisesConfig: unexpectedly totalNum=%d", g.totalNum))
		}
	}
}

type extensionsCall struct {
	// resState is the [State] of the returned [Result] by this call.
	resState State

	// targetState is the [State] value which this call returns early on.
	// if not set (unknown), it means the call doesn't return early.
	targetState State
}

type extensionsResult struct {
	selectCall  extensionsCall
	allCall     extensionsCall
	allWaitCall extensionsCall
	anyCall     extensionsCall
	anyWaitCall extensionsCall
	joinCall    extensionsCall
}

func (r extensionsResult) isStateUnknown() bool {
	set := cmp.Or(
		r.selectCall.resState,
		r.allCall.resState,
		r.allWaitCall.resState,
		r.anyCall.resState,
		r.anyWaitCall.resState,
		r.joinCall.resState,
	)

	// will be false if at least one of the above values is set.
	return set == unknown
}

// testCase is compiled from a [caseConfig] value, which corresponds
// to a test or benchmark case.
type testCase struct {
	name string
	ps   []*Promise[string]

	minWait time.Duration
	maxWait time.Duration

	counter *atomic.Int64

	extRes extensionsResult

	// generatedResults are the list of IDs used in the Promise.Result
	// values generated for sync and async Promises.
	// the order of the values is the order of their Promise's generation.
	// Note: will not be populated for generated benchmark cases.
	generatedResults []generatedResult
}

// getDelayVal returns a random [time.Duration] value in the range [min,max).
// if both min and max are 0, it returns the def value.
func getDelayVal(min, max, defMin, defMax time.Duration) time.Duration {
	if min == 0 && max == 0 {
		min, max = defMin, defMax
	}

	diff := max - min
	if diff == 0 {
		return min // same as max
	}

	r := time.Duration(rand.Int64N(int64(diff)))

	return r + min
}

func compileTestCase(
	t testing.TB,
	config *caseConfig,
	g *Group[string],
	forBenchmark bool,
) *testCase {
	t.Helper()

	// validate the configs.
	config.async.validate()
	config.sync.validate()

	// record the min and max wait used in the generated promises.
	mind := time.Duration(0)
	maxd := time.Duration(0)

	// create helper functions to generate the delay values.
	d := func() time.Duration {
		var dv time.Duration
		if config.async.useUniqueDur {
			v, ok := config.async.uniqueDur.Get()
			if !ok {
				// this will happen if the range is too narrow.
				panic("bad unique wait params")
			}
			dv = v
		} else {
			dv = getDelayVal(
				config.async.minWait,
				config.async.maxWait,
				// default range of 200ms.
				1*time.Millisecond,
				201*time.Millisecond,
			)
		}

		if mind == 0 {
			mind = dv
		}

		mind = min(dv, mind)
		maxd = max(dv, maxd)

		return dv
	}
	dd := func() time.Duration {
		var dv time.Duration
		if config.async.useUniqueDur {
			v, ok := config.async.delayedUniqueDur.Get()
			if !ok {
				// this will happen if the range is too narrow.
				panic("bad unique wait params")
			}
			dv = v
		} else {
			dv = getDelayVal(
				config.async.minDelayedWait,
				config.async.maxDelayedWait,
				// default range of 200ms.
				1000*time.Millisecond,
				1200*time.Millisecond,
			)
		}

		if mind == 0 {
			mind = dv
		}

		mind = min(dv, mind)
		maxd = max(dv, maxd)

		return dv
	}

	// create the result promise slice.
	genPs := make([]*Promise[string], 0, config.totalResultSize())

	// only generate the result IDs lists for normal tests.s
	var genRess []generatedResult
	if !forBenchmark {
		genRess = make([]generatedResult, 0, config.totalResultSize())
	}

	// tracks how many promises executed fully.
	cnt := &atomic.Int64{}

	// generate the required promises...
	genID := 0

	// generate sync promises.
	genSync := 0
	if config.sync.totalNum != 0 {
		for i := 0; i < config.sync.totalNum; i++ {
			var gp *Promise[string]

			if forBenchmark {
				gp = getBenchmarkSyncPromise(g)
			} else {
				var state State
				select {
				case <-closedChan:
					gp = getTestSyncSuccessPromise(g, genID)
					state = Success
				case <-closedChan:
					gp = getTestSyncErrorPromise(g, genID)
					state = Error
				case <-closedChan:
					gp = getTestSyncPanicPromise(g, genID)
					state = Panic
				}

				genRess = append(genRess, generatedResult{
					idx:   genSync,
					id:    genID,
					state: state,
				})
			}

			genPs = append(genPs, gp)
			genID++
			genSync++
		}
	} else {
		for i := 0; i < config.sync.successNum; i++ {
			genPs = append(genPs, getTestSyncSuccessPromise(g, genID))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genSync,
					id:    genID,
					state: Success,
				})
			}

			genID++
			genSync++
		}
		for i := 0; i < config.sync.errorNum; i++ {
			genPs = append(genPs, getTestSyncErrorPromise(g, genID))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genSync,
					id:    genID,
					state: Error,
				})
			}

			genID++
			genSync++
		}
		for i := 0; i < config.sync.panicNum; i++ {
			genPs = append(genPs, getTestSyncPanicPromise(g, genID))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genSync,
					id:    genID,
					state: Panic,
				})
			}

			genID++
			genSync++
		}
	}
	if want := config.sync.totalResultSize(); want != genSync {
		panic(fmt.Sprintf("invalid generated number of sync promises: want=%d, got=%d", want, genSync))
	}

	// account for the generated sync promises.
	cnt.Add(int64(genSync))

	// generate async promises.
	asyncWg := &sync.WaitGroup{}
	asyncWg.Add(config.async.totalResultSize())
	genAsync := 0
	if config.async.totalNum != 0 {
		for i := 0; i < config.async.totalNum; i++ {
			var gp *Promise[string]

			if forBenchmark {
				gd := d()
				gp = getBenchmarkAsyncPromise(g, gd, cnt, asyncWg)
			} else {
				var gd time.Duration
				var state State

				select {
				case <-closedChan:
					gd = d()
					gp = getTestAsyncSuccessPromise(g, genID, gd, cnt, asyncWg)
					state = Success
				case <-closedChan:
					gd = dd()
					gp = getTestAsyncSuccessPromise(g, genID, gd, cnt, asyncWg)
					state = Success
				case <-closedChan:
					gd = d()
					gp = getTestAsyncErrorPromise(g, genID, gd, cnt, asyncWg)
					state = Error
				case <-closedChan:
					gd = dd()
					gp = getTestAsyncErrorPromise(g, genID, gd, cnt, asyncWg)
					state = Error
				case <-closedChan:
					gd = d()
					gp = getTestAsyncPanicPromise(g, genID, gd, cnt, asyncWg)
					state = Panic
				case <-closedChan:
					gd = dd()
					gp = getTestAsyncPanicPromise(g, genID, gd, cnt, asyncWg)
					state = Panic
				}

				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: state,
					delay: gd,
				})
			}

			genPs = append(genPs, gp)
			genID++
			genAsync++
		}
	} else {
		for i := 0; i < config.async.successNum; i++ {
			gd := d()
			genPs = append(genPs, getTestAsyncSuccessPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Success,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
		for i := 0; i < config.async.successDelayNum; i++ {
			gd := dd()
			genPs = append(genPs, getTestAsyncSuccessPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Success,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
		for i := 0; i < config.async.errorNum; i++ {
			gd := d()
			genPs = append(genPs, getTestAsyncErrorPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Error,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
		for i := 0; i < config.async.errorDelayNum; i++ {
			gd := dd()
			genPs = append(genPs, getTestAsyncErrorPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Error,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
		for i := 0; i < config.async.panicNum; i++ {
			gd := d()
			genPs = append(genPs, getTestAsyncPanicPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Panic,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
		for i := 0; i < config.async.panicDelayNum; i++ {
			gd := dd()
			genPs = append(genPs, getTestAsyncPanicPromise(g, genID, gd, cnt, asyncWg))

			if !forBenchmark {
				genRess = append(genRess, generatedResult{
					idx:   genAsync,
					id:    genID,
					state: Panic,
					delay: gd,
				})
			}

			genID++
			genAsync++
		}
	}
	if want := config.async.totalResultSize(); want != genAsync {
		panic(fmt.Sprintf("invalid generated number of async promises: want=%d, got=%d", want, genAsync))
	}

	// Create the compiled test case.
	tc := &testCase{
		name:             config.name,
		ps:               genPs,
		minWait:          mind,
		maxWait:          maxd,
		counter:          cnt,
		extRes:           config.extRes,
		generatedResults: genRess,
	}

	// Wait for all async promises to start.
	asyncWg.Wait()

	return tc
}

func TestPanics(t *testing.T) {
	tests := []struct {
		name          string
		noPanic       bool
		externalPanic bool
		p             func() *Promise[any]
		ip            func() *Promise[IdxRes[any]]
		ips           func() *Promise[[]IdxRes[any]]
	}{
		{
			name:          "nil",
			externalPanic: true,
			p: func() *Promise[any] {
				return GoFunc[any, any](func() error {
					panic(nil)
				})
			},
		},
		{
			name: "GoFunc[any,any]",
			p: func() *Promise[any] {
				return GoFunc[any, any](func() error {
					panic(newTestStrError())
				})
			},
		},
		{
			name: "Select",
			ip: func() *Promise[IdxRes[any]] {
				return Select(
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
				)
			},
		},
		{
			name: "All",
			ips: func() *Promise[[]IdxRes[any]] {
				return All(
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						return nil
					}),
				)
			},
		},
		{
			name: "AllWait",
			ips: func() *Promise[[]IdxRes[any]] {
				return AllWait(
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						return nil
					}),
				)
			},
		},
		{
			name: "Any",
			ips: func() *Promise[[]IdxRes[any]] {
				return Any(
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						return newTestStrError()
					}),
					GoFunc[any, any](func() error {
						return newTestStrError()
					}),
				)
			},
		},
		{
			name: "AnyWait",
			ips: func() *Promise[[]IdxRes[any]] {
				return AnyWait(
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						return newTestStrError()
					}),
					GoFunc[any, any](func() error {
						return newTestStrError()
					}),
				)
			},
		},
		{
			name:    "Join",
			noPanic: true,
			ips: func() *Promise[[]IdxRes[any]] {
				return Join(
					GoFunc[any, any](func() error {
						panic(newTestStrError())
					}),
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						return nil
					}),
				)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error

			if test.p != nil {
				p := test.p()
				r := p.WaitRes()

				if testEnableLogs {
					t.Logf("p res: %v {%T}\n", r, r)
					t.Logf("p val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("p err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("p state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newP := p.Follow(func(ctx context.Context, res Result[any]) Result[any] {
					if res.State() != Panic {
						return res
					}
					if testEnableLogs {
						t.Logf("p recover res: %v {%T}\n", res, res)
						t.Logf("p recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("p recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("p recover state: %v {%T}\n", res.State(), res.State())
					}

					if !errors.Is(res.Err(), ErrPromisePanicked) {
						t.Errorf("p Err(): should be ErrPromisePanicked")
					}
					if perr := new(PanicError); !errors.As(res.Err(), perr) {
						t.Fatalf("p Res() got unexpected error: %v", res.Err())
					}

					return nil
				})

				rNew := newP.WaitRes()

				if testEnableLogs {
					t.Logf("newP res: %v {%T}\n", rNew, rNew)
					t.Logf("newP val: %v {%T}\n", rNew.Val(), rNew.Val())
					t.Logf("newP err: %v {%T}\n", rNew.Err(), rNew.Err())
					t.Logf("newP state: %v {%T}\n", rNew.State(), rNew.State())
				}

				// make sure the Panic is handled in the new promise.
				if expected := Success; rNew.State() != expected {
					t.Errorf("newP state: %v, expected: %v\n", rNew.State(), expected)
				}
			} else if test.ip != nil {
				ip := test.ip()
				r := ip.WaitRes()

				if testEnableLogs {
					t.Logf("ip res: %v {%T}\n", r, r)
					t.Logf("ip val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ip err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ip state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newIp := ip.Follow(func(ctx context.Context, res Result[IdxRes[any]]) Result[IdxRes[any]] {
					if res.State() != Panic {
						return res
					}
					if testEnableLogs {
						t.Logf("ip recover res: %v {%T}\n", res, res)
						t.Logf("ip recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ip recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ip recover state: %v {%T}\n", res.State(), res.State())
					}

					if !errors.Is(res.Err(), ErrPromisePanicked) {
						t.Errorf("ip Err(): should be ErrPromisePanicked")
					}
					if perr := new(PanicError); !errors.As(res.Err(), perr) {
						t.Fatalf("ip Res() got unexpected error: %v", res.Err())
					}

					return nil
				})

				rNew := newIp.WaitRes()

				if testEnableLogs {
					t.Logf("newIp res: %v {%T}\n", rNew, rNew)
					t.Logf("newIp val: %v {%T}\n", rNew.Val(), rNew.Val())
					t.Logf("newIp err: %v {%T}\n", rNew.Err(), rNew.Err())
					t.Logf("newIp state: %v {%T}\n", rNew.State(), rNew.State())
				}

				// make sure the Panic is handled in the new promise.
				if expected := Success; rNew.State() != expected {
					t.Errorf("newIp state: %v, expected: %v\n", rNew.State(), expected)
				}
			} else {
				ips := test.ips()
				r := ips.WaitRes()

				if testEnableLogs {
					t.Logf("ips res: %v {%T}\n", r, r)
					t.Logf("ips val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ips err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ips state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newIps := ips.Follow(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
					if res.State() != Panic {
						return res
					}
					if testEnableLogs {
						t.Logf("ips recover res: %v {%T}\n", res, res)
						t.Logf("ips recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ips recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ips recover state: %v {%T}\n", res.State(), res.State())
					}

					if !errors.Is(res.Err(), ErrPromisePanicked) {
						t.Errorf("ips Err(): should be ErrPromisePanicked")
					}
					if perr := new(PanicError); !errors.As(res.Err(), perr) {
						t.Fatalf("ips Res() got unexpected error: %v", res.Err())
					}

					// clone the received [Result] value before filtering it.
					// note: this is needed as the returned slice (from [Result.Val])
					// is the same value returned from all [Result.Val] calls.
					newResVal := make([]IdxRes[any], len(res.Val()))
					copy(newResVal, res.Val())

					// filter the new [Result] values, to remove the handled values.
					newResVal = slices.DeleteFunc(newResVal, func(i IdxRes[any]) bool {
						return i.State() == Panic
					})

					// create a new [Result] with the needed [State] and [Result] values.
					newRes := MultiRes(Success, newResVal...)

					if testEnableLogs {
						t.Logf("ips new res: %v {%T}\n", newRes, newRes)
						t.Logf("ips new val: %v {%T}\n", newRes.Val(), newRes.Val())
						t.Logf("ips new err: %v {%T}\n", newRes.Err(), newRes.Err())
						t.Logf("ips new state: %v {%T}\n", newRes.State(), newRes.State())

						t.Logf("ips recover res: %v {%T}\n", res, res)
						t.Logf("ips recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ips recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ips recover state: %v {%T}\n", res.State(), res.State())
					}

					return newRes
				})

				rNew := newIps.WaitRes()

				if testEnableLogs {
					t.Logf("newIps res: %v {%T}\n", rNew, rNew)
					t.Logf("newIps val: %v {%T}\n", rNew.Val(), rNew.Val())
					t.Logf("newIps err: %v {%T}\n", rNew.Err(), rNew.Err())
					t.Logf("newIps state: %v {%T}\n", rNew.State(), rNew.State())
				}

				// make sure the Panic is handled in the new promise.
				if expected := Success; rNew.State() != expected {
					t.Errorf("newIps state: %v, expected: %v\n", rNew.State(), expected)
				}
			}

			if test.noPanic {
				return
			}

			if !errors.Is(err, ErrPromisePanicked) {
				t.Errorf("err: %v doesn't implement ErrPromisePanicked\n", err)
			}

			if !test.externalPanic {
				if !errors.Is(err, newTestStrError()) {
					t.Errorf("err: %v doesn't implement newTestStrError\n", err)
				}
			}
		})
	}
}

var (
	testsCases = []caseConfig{
		{
			name: "case1",
			async: generateAsyncPromisesConfig{
				successNum:    1,
				errorDelayNum: 31,
				panicDelayNum: 0,
				useUniqueDur:  true,
			},
			extRes: extensionsResult{
				selectCall: extensionsCall{
					resState:    Success,
					targetState: Success,
				},
				allCall: extensionsCall{
					resState:    Error,
					targetState: Error,
				},
				allWaitCall: extensionsCall{
					resState: Error,
				},
				anyCall: extensionsCall{
					resState:    Success,
					targetState: Success,
				},
				anyWaitCall: extensionsCall{
					resState: Success,
				},
				joinCall: extensionsCall{
					resState: Success,
				},
			},
		},
		{
			name: "case2",
			async: generateAsyncPromisesConfig{
				successDelayNum: 31,
				errorNum:        1,
				panicDelayNum:   0,
				useUniqueDur:    true,
			},
			extRes: extensionsResult{
				selectCall: extensionsCall{
					resState:    Error,
					targetState: Error,
				},
				allCall: extensionsCall{
					resState:    Error,
					targetState: Error,
				},
				allWaitCall: extensionsCall{
					resState: Error,
				},
				anyCall: extensionsCall{
					resState:    Success,
					targetState: Success,
				},
				anyWaitCall: extensionsCall{
					resState: Success,
				},
				joinCall: extensionsCall{
					resState: Success,
				},
			},
		},
		{
			name: "case3",
			async: generateAsyncPromisesConfig{
				successDelayNum: 30,
				errorDelayNum:   1,
				panicNum:        1,
				useUniqueDur:    true,
			},
			extRes: extensionsResult{
				selectCall: extensionsCall{
					resState:    Panic,
					targetState: Panic,
				},
				allCall: extensionsCall{
					resState:    Panic,
					targetState: Panic,
				},
				allWaitCall: extensionsCall{
					resState: Panic,
				},
				anyCall: extensionsCall{
					resState:    Success,
					targetState: Success,
				},
				anyWaitCall: extensionsCall{
					resState: Success,
				},
				joinCall: extensionsCall{
					resState: Success,
				},
			},
		},
		{
			name: "case4",
			async: generateAsyncPromisesConfig{
				successNum:   0,
				errorNum:     32,
				panicNum:     0,
				useUniqueDur: true,
			},
			extRes: extensionsResult{
				selectCall: extensionsCall{
					resState:    Error,
					targetState: Error,
				},
				allCall: extensionsCall{
					resState:    Error,
					targetState: Error,
				},
				allWaitCall: extensionsCall{
					resState: Error,
				},
				anyCall: extensionsCall{
					resState:    Error,
					targetState: Success,
				},
				anyWaitCall: extensionsCall{
					resState: Error,
				},
				joinCall: extensionsCall{
					resState: Success,
				},
			},
		},
	}
)

// helper functions for testing [Wait], [Select], [All], [Any], and [Join]...

func helperTest[TFun extFuncs[TRes], TRes extRes](
	t *testing.T,
	tc *caseConfig,
	fun TFun,
	funName string,
	op joinOperationLogic,
	wantCall *extensionsCall,
) {
	t.Helper()
	t.Run(funName, func(t *testing.T) {
		tt := compileTestCase(t, tc, nil, false)

		// sort the got result by the expected resolution order.
		// note: we rely on each subsequent delays are far from each other,
		// so that we (somewhat) can rely on an expected resolution order.
		slices.SortStableFunc(
			tt.generatedResults,
			func(a, b generatedResult) int { return int(a.delay - b.delay) },
		)

		switch f := any(fun).(type) {
		case waitFunc:
			st := time.Now()
			f(tt.ps...)
			counted := tt.counter.Load()
			el := time.Since(st)

			if testEnableLogs {
				t.Logf("%s (delay): want_min: %v got: %v\n", funName, tt.minWait, el)
				t.Logf("%s (delay): want_max: %v got: %v\n", funName, tt.maxWait, el)
				t.Logf("%s (counted): want: %v got %v\n", funName, len(tt.ps), counted)
			}

			lo := tt.maxWait - (2 * time.Millisecond)
			hi := tt.maxWait + (2 * time.Millisecond)
			if el > hi || el < lo {
				t.Errorf("%s (delay): want (%v : %v) got %v", funName, lo, hi, el)
			}

			if want := int64(len(tt.ps)); want != counted {
				t.Errorf("%s (counted): want %v got %v", funName, want, counted)
			}
		case singleIdxFunc:
			p := f(tt.ps...)
			gotRes := p.WaitRes()

			// get the expected generated result.
			// it's the first element as it's gonna be the Promise
			// that was resolved the earliest.
			wantGenRes := tt.generatedResults[0]

			// make sure the got result matches the expected state.
			if wantCall.resState != wantGenRes.state {
				t.Errorf(
					"%s: want State %v, got %v",
					funName,
					wantCall.resState,
					wantGenRes.state,
				)
			}

			// construct the expected result string.
			wantResStr := wantGenRes.String()

			// construct the expected result value.
			var res singleIdxRes
			if wantGenRes.state == Success {
				res = singleIdxRes{
					Idx: wantGenRes.idx,
					Result: result[string]{
						val:   wantResStr,
						state: wantGenRes.state,
					},
				}
			} else {
				res = singleIdxRes{
					Idx: wantGenRes.idx,
					Result: result[string]{
						err:   errors.New(wantResStr),
						state: wantGenRes.state,
					},
				}
			}
			wantRes := newSingleRes(wantGenRes.state, res)

			if !isEqualResult(gotRes, wantRes) {
				t.Errorf(
					"%s: want %v(%T), got %v(%T)",
					funName,
					wantRes,
					wantRes,
					gotRes,
					gotRes,
				)
			}
		case multiIdxFunc:
			p := f(tt.ps...)
			gotRes := p.WaitRes()

			// addToMultiRes adds the genRes to the multiRes value at the
			// expected index, which is the index of the original promise.
			addToMultiRes := func(multiRes multiIdxRes, genRes generatedResult) multiIdxRes {
				if genRes.state == Success {
					multiRes[genRes.idx] = singleIdxRes{
						Idx: genRes.idx,
						Result: result[string]{
							val:   genRes.String(),
							state: genRes.state,
						},
					}
				} else {
					multiRes[genRes.idx] = singleIdxRes{
						Idx: genRes.idx,
						Result: result[string]{
							err:   errors.New(genRes.String()),
							state: genRes.state,
						},
					}
				}
				return multiRes
			}

			// construct the expected result value...
			// if there's no targetState, then we are expected to wait for
			// all promises, so construct the result from all the generated
			// results.
			// otherwise, include only upto the targetState.
			res := make(multiIdxRes, len(tt.generatedResults))
			if wantCall.targetState == unknown {
				for _, genRes := range tt.generatedResults {
					res = addToMultiRes(res, genRes)
				}
			} else {
				for _, genRes := range tt.generatedResults {
					res = addToMultiRes(res, genRes)
					if genRes.state == wantCall.targetState {
						break
					}
				}
			}
			res = slices.DeleteFunc(
				res,
				func(r IdxRes[string]) bool {
					return r.Result == nil
				},
			)
			res = slices.Clip(res)
			wantRes := newMultiRes(op, wantCall.resState, res)

			if !isEqualResult(gotRes, wantRes) {
				t.Errorf(
					"%s: want %v(%T), got %v(%T)",
					funName,
					wantRes,
					wantRes,
					gotRes,
					gotRes,
				)
			}
		}
	})
}

func TestExtFuncs(t *testing.T) {
	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			helperTest[waitFunc, singleIdxRes](
				t,
				&tc,
				Wait,
				"Wait",
				nil,
				nil,
			)

			// skip this test, as it's not meant for the rest of the functions...
			if tc.extRes.isStateUnknown() {
				return
			}

			helperTest[singleIdxFunc, singleIdxRes](
				t,
				&tc,
				Select,
				"Select",
				nil,
				&tc.extRes.selectCall,
			)
			helperTest[multiIdxFunc, multiIdxRes](
				t,
				&tc,
				All,
				"All",
				allOp,
				&tc.extRes.allCall,
			)
			helperTest[multiIdxFunc, multiIdxRes](
				t,
				&tc,
				AllWait,
				"AllWait",
				allWaitOp,
				&tc.extRes.allWaitCall,
			)
			helperTest[multiIdxFunc, multiIdxRes](
				t,
				&tc,
				Any,
				"Any",
				anyOp,
				&tc.extRes.anyCall,
			)
			helperTest[multiIdxFunc, multiIdxRes](
				t,
				&tc,
				AnyWait,
				"AnyWait",
				anyWaitOp,
				&tc.extRes.anyWaitCall,
			)
			helperTest[multiIdxFunc, multiIdxRes](
				t,
				&tc,
				Join,
				"Join",
				joinOp,
				&tc.extRes.joinCall,
			)
		})
	}
}

func TestJoin(t *testing.T) {
	t.Run("no-sleep", func(t *testing.T) {
		p1 := GoFunc[any, any](func() error {
			return errors.New("p1 error")
		})
		p2 := GoFunc[any, any](func() error {
			return errors.New("p2 error")
		})
		p3 := GoFunc[any, any](func(ctx context.Context) Result[any] {
			return ValErrRes[any]("never", errors.New("p3 error"))
		})
		p4 := GoFunc[any, any](func(ctx context.Context) Result[any] {
			return p1
		})

		joinP1 := Join(p1, p2, p3, p4)
		joinP2 := joinP1.Follow(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
			if res.State() != Success {
				return res
			}
			for _, v := range res.Val() {
				if testEnableLogs {
					t.Logf("[%T] %v", v.Result, v)
				}
			}
			return ErrRes[[]IdxRes[any]](errors.New("join error"))
		})
		joinP1.Wait()
		joinP2.Wait()

		if joinP1.State() != Success {
			t.Errorf("Join: %v, expected: %v", joinP1.WaitRes(), Success)
		}
		if joinP2.State() != Error {
			t.Errorf("Join: %v, expected: %v", joinP1.WaitRes(), Error)
		}
	})

	t.Run("with-sleep", func(t *testing.T) {
		p1 := GoFunc[any, any](func() error {
			time.Sleep(time.Millisecond * 100)
			return errors.New("p1 error")
		})
		p2 := GoFunc[any, any](func() error {
			time.Sleep(time.Millisecond * 100)
			return errors.New("p2 error")
		})
		p3 := GoFunc[any, any](func(ctx context.Context) Result[any] {
			time.Sleep(time.Millisecond * 100)
			return ValErrRes[any]("never", errors.New("p3 error"))
		})
		p4 := GoFunc[any, any](func(ctx context.Context) Result[any] {
			time.Sleep(time.Millisecond * 100)
			return p1
		})

		joinP1 := Join(p1, p2, p3, p4)
		joinP2 := joinP1.Follow(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
			if res.State() != Success {
				return res
			}
			for _, v := range res.Val() {
				if testEnableLogs {
					t.Logf("[%T] %v", v.Result, v)
				}
			}
			return ErrRes[[]IdxRes[any]](errors.New("join error"))
		})
		joinP1.Wait()
		joinP2.Wait()

		if joinP1.State() != Success {
			t.Errorf("Join: %v, expected: %v", joinP1.WaitRes(), Success)
		}
		if joinP2.State() != Error {
			t.Errorf("Join: %v, expected: %v", joinP1.WaitRes(), Error)
		}
	})

	t.Run("other", func(t *testing.T) {
		p1 := GoFunc[any, any](func() error {
			panic("p1 panic")
		})

		called := atomic.Bool{}

		join := Join(p1).Follow(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
			if res.State() != Success {
				return res
			}
			called.Store(true)

			for _, v := range res.Val() {
				if testEnableLogs {
					t.Logf("[%T] %v", v.Result, v)
				}
			}
			return nil
		})

		join.Wait()

		if !called.Load() {
			t.Errorf("Then not called on Join Promise")
		}
	})
}

// to truly benchmark the target functions, all benchmark cases generate only
// [Success] promises in the async cases, to avoid allocating any [Result]
// values that might mislead the result, hence we only use the totalNum.
var (
	benchmarkCases = []caseConfig{
		{
			name: "16-fast-async",
			async: generateAsyncPromisesConfig{
				totalNum: 16,
				minWait:  10 * time.Nanosecond,
				maxWait:  10 * time.Nanosecond,
			},
		},
		{
			name: "16:8-async-8-sync",
			async: generateAsyncPromisesConfig{
				totalNum: 8,
				minWait:  10 * time.Microsecond,
				maxWait:  10 * time.Microsecond,
			},
			sync: generateSyncPromisesConfig{
				totalNum: 8,
			},
		},
		{
			name: "100:50-async-50-sync",
			async: generateAsyncPromisesConfig{
				totalNum: 50,
				minWait:  10 * time.Microsecond,
				maxWait:  10 * time.Microsecond,
			},
			sync: generateSyncPromisesConfig{
				totalNum: 50,
			},
		},
		{
			name: "100-async",
			async: generateAsyncPromisesConfig{
				totalNum: 100,
				minWait:  10 * time.Microsecond,
				maxWait:  10 * time.Microsecond,
			},
		},
		{
			name: "100-sync",
			sync: generateSyncPromisesConfig{
				totalNum: 100,
			},
		},
		{
			name: "1000-async",
			async: generateAsyncPromisesConfig{
				totalNum: 1000,
				minWait:  10 * time.Microsecond,
				maxWait:  10 * time.Microsecond,
			},
		},
		{
			name: "1000-sync",
			sync: generateSyncPromisesConfig{
				totalNum: 1000,
			},
		},
	}
)

// helper functions and types for benchmarking [Wait], [Select], [All], [AllWait],
// [Any], [AnyWait], and [Join]...

func helperBenchmark[TFun extFuncs[TRes], TRes extRes](
	b *testing.B,
	bc caseConfig,
	fun TFun,
	funName string,
	addWait bool,
) {
	b.Helper()
	switch f := any(fun).(type) {
	case waitFunc:
		b.Run(funName, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				tt := compileTestCase(b, &bc, nil, true)
				b.StartTimer()

				f(tt.ps...)
			}
		})
	case idxFunc[TRes]:
		b.Run(funName, func(b *testing.B) {
			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, &bc, nil, true)
					b.StartTimer()

					f(tt.ps...)
				}
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, &bc, nil, true)
					b.StartTimer()

					p := f(tt.ps...)
					p.Wait()
				}
			}
		})
	}
}

func helperBenchmarkParallel[TFun extFuncs[TRes], TRes IdxRes[string] | []IdxRes[string]](
	b *testing.B,
	bc caseConfig,
	fun TFun,
	funName string,
	addWait bool,
) {
	b.Helper()
	switch f := any(fun).(type) {
	case waitFunc:
		b.Run(funName+"-Parallel", func(b *testing.B) {
			tt := compileTestCase(b, &bc, nil, true)

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					f(tt.ps...)
				}
			})
		})
	case idxFunc[TRes]:
		b.Run(funName+"-Parallel", func(b *testing.B) {
			tt := compileTestCase(b, &bc, nil, true)

			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						f(tt.ps...)
					}
				})
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						p := f(tt.ps...)
						p.Wait()
					}
				})
			}
		})
	}
}

func BenchmarkExtFuncs(b *testing.B) {
	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.Run("Wait", func(b *testing.B) {
				helperBenchmark[waitFunc, IdxRes[string]](
					b,
					bc,
					Wait,
					"Wait",
					false,
				)
				helperBenchmarkParallel[waitFunc, IdxRes[string]](
					b,
					bc,
					Wait,
					"Wait",
					false,
				)
			})

			b.Run("Select", func(b *testing.B) {
				helperBenchmark[singleIdxFunc, IdxRes[string]](
					b,
					bc,
					Select,
					"Select",
					false,
				)
				helperBenchmarkParallel[singleIdxFunc, IdxRes[string]](
					b,
					bc,
					Select,
					"Select",
					false,
				)
				helperBenchmark[singleIdxFunc, IdxRes[string]](
					b,
					bc,
					Select,
					"Select-Wait",
					true,
				)
				helperBenchmarkParallel[singleIdxFunc, IdxRes[string]](
					b,
					bc,
					Select,
					"Select-Wait",
					true,
				)
			})

			b.Run("AllWait", func(b *testing.B) {
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AllWait,
					"AllWait",
					false,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AllWait,
					"AllWait",
					false,
				)
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AllWait,
					"AllWait-Wait",
					true,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AllWait,
					"AllWait-Wait",
					true,
				)
			})

			b.Run("AnyWait", func(b *testing.B) {
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AnyWait,
					"AnyWait",
					false,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AnyWait,
					"AnyWait",
					false,
				)
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AnyWait,
					"AnyWait-Wait",
					true,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					AnyWait,
					"AnyWait-Wait",
					true,
				)
			})

			b.Run("Join", func(b *testing.B) {
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					Join,
					"Join",
					false,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					Join,
					"Join",
					false,
				)
				helperBenchmark[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					Join,
					"Join-Wait",
					true,
				)
				helperBenchmarkParallel[multiIdxFunc, []IdxRes[string]](
					b,
					bc,
					Join,
					"Join-Wait",
					true,
				)
			})
		})
	}
}

func helperBenchmarkLeakDetector[TFun extFuncs[TRes], TRes extRes](
	b *testing.B,
	bc caseConfig,
	fun TFun,
	funName string,
) {
	b.Run(funName, func(b *testing.B) {
		debugMetrics, debugCB := createDebugMetricsCB()
		g := NewGroup[string](UnhandledErrorCB(func(err error) {}))
		g.core.debugCB = debugCB

		tt := compileTestCase(b, &bc, g, true)

		b.ReportAllocs()

		switch f := any(fun).(type) {
		case waitFunc:
			b.ResetTimer()
			for range b.N {
				f(tt.ps...)
			}
			b.StopTimer()
		case idxFunc[TRes]:
			b.ResetTimer()
			for range b.N {
				p := f(tt.ps...)
				p.Wait()
			}
			b.StopTimer()
		}

		if testEnableLogs {
			b.Log(debugMetrics.String())
		}

		// at this point, all counters up to the ext-calls related should
		// report correct values, so validate that...
		expected := int64(bc.async.totalResultSize())
		startedCount := debugMetrics.get(startHandler)
		resolvedCount := debugMetrics.get(resolve)
		foundExtChanCount := debugMetrics.get(foundExtChan)
		foundExtQueueCount := debugMetrics.get(foundExtQueue)
		if expected != startedCount ||
			expected != resolvedCount {
			b.Errorf(
				"Possible goroutine leaks from %s: expected: %d, started: %d, resolved: %d\n",
				funName,
				expected,
				startedCount,
				resolvedCount,
			)
		}
		if foundExtChanCount != foundExtQueueCount {
			b.Errorf(
				"Possible goroutine leaks from %s handleExtCalls: foundExtChanCount: %d, foundExtQueueCount: %d\n",
				funName,
				foundExtChanCount,
				foundExtQueueCount,
			)
		}

		// at this point, only the group-calls related might still not counted,
		// so we need to wait for them to get a correct result...
		g.Wait()

		if testEnableLogs {
			b.Log(debugMetrics.String())
		}

		endedCount := debugMetrics.get(endHandler)
		if expected != endedCount {
			b.Errorf(
				"Possible goroutine leaks from %s: expected: %d, started: %d, ended: %d\n",
				funName,
				expected,
				startedCount,
				endedCount,
			)
		}
	})
}

// BenchmarkExtFuncs_LeakDetector is only concerned with [Wait], [AllWait],
// [AnyWait] and [Join], as it requires an extension function that blocks
// until all ongoing promises have finished.
func BenchmarkExtFuncs_LeakDetector(b *testing.B) {
	// skip based on the present build tags.
	if !debugEnabled {
		b.Skip("the 'enable_promise_debug' build tag isn't enabled")
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			helperBenchmarkLeakDetector[waitFunc, singleIdxRes](
				b,
				bc,
				Wait,
				"Wait",
			)
			helperBenchmarkLeakDetector[multiIdxFunc, multiIdxRes](
				b,
				bc,
				AllWait,
				"AllWait",
			)
			helperBenchmarkLeakDetector[multiIdxFunc, multiIdxRes](
				b,
				bc,
				AnyWait,
				"AnyWait",
			)
			helperBenchmarkLeakDetector[multiIdxFunc, multiIdxRes](
				b,
				bc,
				Join,
				"Join",
			)
		})
	}
}
