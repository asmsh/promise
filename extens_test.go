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
)

var testEnableLogs = false

// used as a randomizer in compileTestCase.
var closedChan chan struct{}

func init() {
	closedChan = make(chan struct{})
	close(closedChan)
}

func getTestAsyncSuccessPromise(
	g *Group[string],
	id int,
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
		return "", errors.New(fmt.Sprintf("%d hello world async %s", id, d))
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
		panic(errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
	})
}

var (
	syncSuccessPromise1 = Wrap[string](ValRes("hello world sync 1"))
	syncErrorPromise1   = Wrap[string](ValErrRes("err world sync 1", errors.New("wrap error")))
	syncPanicPromise1   = Wrap[string](PanicRes[string](errors.New("panic error")))
)

type testCase struct {
	name string
	p    []*Promise[string]

	minWait time.Duration
	maxWait time.Duration

	counter *atomic.Int64

	extRes extensionsResult
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
}

func (g generateAsyncPromisesConfig) resultSize() int {
	if g.successNum != 0 || g.successDelayNum != 0 ||
		g.errorNum != 0 || g.errorDelayNum != 0 ||
		g.panicNum != 0 || g.panicDelayNum != 0 {
		return g.successNum + g.successDelayNum +
			g.errorNum + g.errorDelayNum +
			g.panicNum + g.panicDelayNum
	}
	return g.totalNum
}

func (g generateAsyncPromisesConfig) validate() {
	// if any of the specific values is provided, the totalNum must be 0
	if g.successNum != 0 || g.successDelayNum != 0 ||
		g.errorNum != 0 || g.errorDelayNum != 0 ||
		g.panicNum != 0 || g.panicDelayNum != 0 {
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
}

type generateSyncPromisesConfig struct {
	// totalNum is the total number of randomly generated sync promises
	totalNum int

	// if any of the below is provided, the totalNum is ignored.
	successNum int
	errorNum   int
	panicNum   int
}

func (g generateSyncPromisesConfig) resultSize() int {
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

type extensionsResult struct {
	selectState  State
	allState     State
	allWaitState State
	anyState     State
	anyWaitState State
	joinState    State
}

func (r extensionsResult) isStateUnknown() bool {
	set := cmp.Or(
		r.selectState,
		r.allState,
		r.allWaitState,
		r.anyState,
		r.anyWaitState,
		r.joinState,
	)

	// will be false if at least one of the above values is set.
	return set == unknown
}

type caseConfig struct {
	name string

	async generateAsyncPromisesConfig
	sync  generateSyncPromisesConfig

	extRes extensionsResult
}

func (c caseConfig) totalResultSize() int {
	return c.async.resultSize() + c.sync.resultSize()
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
	config caseConfig,
	g *Group[string],
) testCase {
	t.Helper()

	// validate the configs.
	config.async.validate()
	config.sync.validate()

	// record the min and max wait used in the generated promises.
	mind := time.Duration(0)
	maxd := time.Duration(0)

	// tracks how many promises executed fully.
	cnt := &atomic.Int64{}

	// create helper functions to generate the delay values.
	d := func() time.Duration {
		gd := getDelayVal(
			config.async.minWait,
			config.async.maxWait,
			5*time.Millisecond,
			10*time.Millisecond, // default range of 5ms.
		)

		if mind == 0 {
			mind = gd
		}

		mind = min(gd, mind)
		maxd = max(gd, maxd)

		return gd
	}
	dd := func() time.Duration {
		gd := getDelayVal(
			config.async.minDelayedWait,
			config.async.maxDelayedWait,
			1000*time.Millisecond,
			1100*time.Millisecond, // default range of 100ms.
		)

		if mind == 0 {
			mind = gd
		}

		mind = min(gd, mind)
		maxd = max(gd, maxd)

		return gd
	}

	// create the result promise slice.
	p := make([]*Promise[string], 0, config.totalResultSize())

	// generate the required promises...
	// generate sync promises.
	genSync := 0
	if config.sync.totalNum != 0 {
		for i := 0; i < config.sync.totalNum; i++ {
			select {
			case <-closedChan:
				p = append(p, syncSuccessPromise1)
			case <-closedChan:
				p = append(p, syncErrorPromise1)
			case <-closedChan:
				p = append(p, syncPanicPromise1)
			}
			genSync++
		}
	} else {
		for i := 0; i < config.sync.successNum; i++ {
			p = append(p, syncSuccessPromise1)
			genSync++
		}
		for i := 0; i < config.sync.errorNum; i++ {
			p = append(p, syncErrorPromise1)
			genSync++
		}
		for i := 0; i < config.sync.panicNum; i++ {
			p = append(p, syncPanicPromise1)
			genSync++
		}
	}
	if want := config.sync.resultSize(); want != genSync {
		panic(fmt.Sprintf("invalid generated number of sync promises: want=%d, got=%d", want, genSync))
	}

	// account for the generated sync promises.
	cnt.Add(int64(genSync))

	// generate async promises.
	asyncWg := &sync.WaitGroup{}
	asyncWg.Add(config.async.resultSize())
	genAsync := 0
	if config.async.totalNum != 0 {
		for i := 0; i < config.async.totalNum; i++ {
			select {
			case <-closedChan:
				p = append(p, getTestAsyncSuccessPromise(g, genAsync, d(), cnt, asyncWg))
			case <-closedChan:
				p = append(p, getTestAsyncSuccessPromise(g, genAsync, dd(), cnt, asyncWg))
			case <-closedChan:
				p = append(p, getTestAsyncErrorPromise(g, genAsync, d(), cnt, asyncWg))
			case <-closedChan:
				p = append(p, getTestAsyncErrorPromise(g, genAsync, dd(), cnt, asyncWg))
			case <-closedChan:
				p = append(p, getTestAsyncPanicPromise(g, genAsync, d(), cnt, asyncWg))
			case <-closedChan:
				p = append(p, getTestAsyncPanicPromise(g, genAsync, dd(), cnt, asyncWg))
			}
			genAsync++
		}
	} else {
		for i := 0; i < config.async.successNum; i++ {
			p = append(p, getTestAsyncSuccessPromise(g, genAsync, d(), cnt, asyncWg))
			genAsync++
		}
		for i := 0; i < config.async.successDelayNum; i++ {
			p = append(p, getTestAsyncSuccessPromise(g, genAsync, dd(), cnt, asyncWg))
			genAsync++
		}
		for i := 0; i < config.async.errorNum; i++ {
			p = append(p, getTestAsyncErrorPromise(g, genAsync, d(), cnt, asyncWg))
			genAsync++
		}
		for i := 0; i < config.async.errorDelayNum; i++ {
			p = append(p, getTestAsyncErrorPromise(g, genAsync, dd(), cnt, asyncWg))
			genAsync++
		}
		for i := 0; i < config.async.panicNum; i++ {
			p = append(p, getTestAsyncPanicPromise(g, genAsync, d(), cnt, asyncWg))
			genAsync++
		}
		for i := 0; i < config.async.panicDelayNum; i++ {
			p = append(p, getTestAsyncPanicPromise(g, genAsync, dd(), cnt, asyncWg))
			genAsync++
		}
	}
	if want := config.async.resultSize(); want != genAsync {
		panic(fmt.Sprintf("invalid generated number of async promises: want=%d, got=%d", want, genAsync))
	}

	// Wait for all async promises to start.
	asyncWg.Wait()

	return testCase{
		name:    config.name,
		p:       p,
		minWait: mind,
		maxWait: maxd,
		counter: cnt,
		extRes:  config.extRes,
	}
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
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
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
						time.Sleep(time.Second)
						return nil
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
				r := p.Res()

				if testEnableLogs {
					t.Logf("p res: %v {%T}\n", r, r)
					t.Logf("p val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("p err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("p state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newP := p.Recover(func(ctx context.Context, res Result[any]) Result[any] {
					if testEnableLogs {
						t.Logf("p recover res: %v {%T}\n", res, res)
						t.Logf("p recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("p recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("p recover state: %v {%T}\n", res.State(), res.State())
					}

					return nil
				})

				rNew := newP.Res()

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
				r := ip.Res()

				if testEnableLogs {
					t.Logf("ip res: %v {%T}\n", r, r)
					t.Logf("ip val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ip err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ip state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newIp := ip.Recover(func(ctx context.Context, res Result[IdxRes[any]]) Result[IdxRes[any]] {
					if testEnableLogs {
						t.Logf("ip recover res: %v {%T}\n", res, res)
						t.Logf("ip recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ip recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ip recover state: %v {%T}\n", res.State(), res.State())
					}

					return nil
				})

				rNew := newIp.Res()

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
				r := ips.Res()

				if testEnableLogs {
					t.Logf("ips res: %v {%T}\n", r, r)
					t.Logf("ips val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ips err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ips state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err() // needed for the whole test.

				newIps := ips.Recover(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
					if testEnableLogs {
						t.Logf("ips recover res: %v {%T}\n", res, res)
						t.Logf("ips recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ips recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ips recover state: %v {%T}\n", res.State(), res.State())
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

				rNew := newIps.Res()

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
			async: generateAsyncPromisesConfig{
				successNum:    1,
				errorDelayNum: 32,
				panicDelayNum: 0,
			},
			extRes: extensionsResult{
				selectState:  Success,
				allState:     Error,
				allWaitState: Error,
				anyState:     Success,
				anyWaitState: Success,
				joinState:    Success,
			},
		},
		{
			async: generateAsyncPromisesConfig{
				successDelayNum: 32,
				errorNum:        1,
				panicDelayNum:   0,
			},
			extRes: extensionsResult{
				selectState:  Error,
				allState:     Error,
				allWaitState: Error,
				anyState:     Success,
				anyWaitState: Success,
				joinState:    Success,
			},
		},
		{
			async: generateAsyncPromisesConfig{
				successDelayNum: 32,
				errorDelayNum:   1,
				panicNum:        1,
			},
			extRes: extensionsResult{
				selectState:  Panic,
				allState:     Panic,
				allWaitState: Panic,
				anyState:     Panic,
				anyWaitState: Panic,
				joinState:    Success,
			},
		},
		{
			async: generateAsyncPromisesConfig{
				successNum: 0,
				errorNum:   32,
				panicNum:   0,
			},
			extRes: extensionsResult{
				selectState:  Error,
				allState:     Error,
				allWaitState: Error,
				anyState:     Error,
				anyWaitState: Error,
				joinState:    Success,
			},
		},
	}
)

// helper types for tests and benchmarks...

type waitFunc func(...*Promise[string])
type singleIdxFunc func(...*Promise[string]) *Promise[IdxRes[string]]
type multiIdxFunc func(...*Promise[string]) *Promise[[]IdxRes[string]]

type extFunc interface {
	waitFunc | singleIdxFunc | multiIdxFunc
}

// helper functions for testing [Wait], [Select], [All], [AllWait],
// [Any], [AnyWait], and [Join]...

func helperTest[TFun extFunc](
	t *testing.T,
	tc caseConfig,
	fun TFun,
	funName string,
	wantState State,
) {
	t.Helper()
	t.Run(funName, func(t *testing.T) {
		tt := compileTestCase(t, tc, nil)

		switch f := any(fun).(type) {
		case waitFunc:
			st := time.Now()
			f(tt.p...)
			counted := tt.counter.Load()
			el := time.Since(st)

			if testEnableLogs {
				t.Logf("%s (delay): want_min: %v got: %v\n", funName, tt.minWait, el)
				t.Logf("%s (delay): want_max: %v got: %v\n", funName, tt.maxWait, el)
				t.Logf("%s (counted): want: %v got %v\n", funName, len(tt.p), counted)
			}

			lo := tt.maxWait - (2 * time.Millisecond)
			hi := tt.maxWait + (2 * time.Millisecond)
			if el > hi || el < lo {
				t.Errorf("%s (delay): want (%v : %v) got %v", funName, lo, hi, el)
			}

			if want := int64(len(tt.p)); want != counted {
				t.Errorf("%s (counted): want %v got %v", funName, want, counted)
			}
		case singleIdxFunc:
			p := f(tt.p...)
			if got := p.State(); got != wantState {
				t.Errorf("%s: want %v got %v", funName, wantState, got)
			}
			p.Wait()
		case multiIdxFunc:
			p := f(tt.p...)
			if got := p.State(); got != wantState {
				t.Errorf("%s: want %v got %v", funName, wantState, got)
			}
			p.Wait()
		}
	})
}

func TestExtFuncs(t *testing.T) {
	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			helperTest[waitFunc](t, tc, Wait, "Wait", unknown)

			// skip this test, as it's not meant for the rest of the functions...
			if tc.extRes.isStateUnknown() {
				return
			}

			helperTest[singleIdxFunc](t, tc, Select, "Select", tc.extRes.selectState)
			helperTest[multiIdxFunc](t, tc, All, "All", tc.extRes.allState)
			helperTest[multiIdxFunc](t, tc, AllWait, "AllWait", tc.extRes.allWaitState)
			helperTest[multiIdxFunc](t, tc, Any, "Any", tc.extRes.anyState)
			helperTest[multiIdxFunc](t, tc, AnyWait, "AnyWait", tc.extRes.anyWaitState)
			helperTest[multiIdxFunc](t, tc, Join, "Join", tc.extRes.joinState)
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
		p3 := GoCtxRes(func(ctx context.Context) Result[any] {
			return ValErrRes[any]("never", errors.New("p3 error"))
		})
		p4 := GoCtxRes(func(ctx context.Context) Result[any] {
			return p1
		})

		joinP1 := Join(p1, p2, p3, p4)
		joinP2 := joinP1.Then(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
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
			t.Errorf("Join: %v, expected: %v", joinP1.Res(), Success)
		}
		if joinP2.State() != Error {
			t.Errorf("Join: %v, expected: %v", joinP1.Res(), Error)
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
		p3 := GoCtxRes(func(ctx context.Context) Result[any] {
			time.Sleep(time.Millisecond * 100)
			return ValErrRes[any]("never", errors.New("p3 error"))
		})
		p4 := GoCtxRes(func(ctx context.Context) Result[any] {
			time.Sleep(time.Millisecond * 100)
			return p1
		})

		joinP1 := Join(p1, p2, p3, p4)
		joinP2 := joinP1.Then(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
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
			t.Errorf("Join: %v, expected: %v", joinP1.Res(), Success)
		}
		if joinP2.State() != Error {
			t.Errorf("Join: %v, expected: %v", joinP1.Res(), Error)
		}
	})

	t.Run("other", func(t *testing.T) {
		p1 := GoFunc[any, any](func() error {
			panic("p1 panic")
		})

		called := atomic.Bool{}

		join := Join(p1).Then(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
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
// values that might mislead the result.
var (
	benchmarkCases = []caseConfig{
		{
			name: "16-fast-async",
			async: generateAsyncPromisesConfig{
				successNum: 16,
				minWait:    10 * time.Nanosecond,
				maxWait:    10 * time.Nanosecond,
			},
		},
		{
			name: "16:8-async-8-sync",
			async: generateAsyncPromisesConfig{
				successNum: 8,
				minWait:    10 * time.Microsecond,
				maxWait:    10 * time.Microsecond,
			},
			sync: generateSyncPromisesConfig{
				successNum: 8,
			},
		},
		{
			name: "100:50-async-50-sync",
			async: generateAsyncPromisesConfig{
				successNum: 50,
				minWait:    10 * time.Microsecond,
				maxWait:    10 * time.Microsecond,
			},
			sync: generateSyncPromisesConfig{
				successNum: 50,
			},
		},
		{
			name: "100-async",
			async: generateAsyncPromisesConfig{
				successNum: 100,
				minWait:    10 * time.Microsecond,
				maxWait:    10 * time.Microsecond,
			},
		},
		{
			name: "100-sync",
			sync: generateSyncPromisesConfig{
				successNum: 100,
			},
		},
		{
			name: "1000-async",
			async: generateAsyncPromisesConfig{
				successNum: 1000,
				minWait:    10 * time.Microsecond,
				maxWait:    10 * time.Microsecond,
			},
		},
		{
			name: "1000-sync",
			sync: generateSyncPromisesConfig{
				successNum: 1000,
			},
		},
	}
)

// helper functions and types for benchmarking [Wait], [Select], [All], [AllWait],
// [Any], [AnyWait], and [Join]...

func helperBenchmark[TFun extFunc](
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
				tt := compileTestCase(b, bc, nil)
				b.StartTimer()

				f(tt.p...)
			}
		})
	case singleIdxFunc:
		b.Run(funName, func(b *testing.B) {
			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, bc, nil)
					b.StartTimer()

					f(tt.p...)
				}
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, bc, nil)
					b.StartTimer()

					p := f(tt.p...)
					p.Wait()
				}
			}
		})
	case multiIdxFunc:
		b.Run(funName, func(b *testing.B) {
			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, bc, nil)
					b.StartTimer()

					f(tt.p...)
				}
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					tt := compileTestCase(b, bc, nil)
					b.StartTimer()

					p := f(tt.p...)
					p.Wait()
				}
			}
		})
	}
}

func helperBenchmarkParallel[TFun extFunc](
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
			tt := compileTestCase(b, bc, nil)

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					f(tt.p...)
				}
			})
		})
	case singleIdxFunc:
		b.Run(funName+"-Parallel", func(b *testing.B) {
			tt := compileTestCase(b, bc, nil)

			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						f(tt.p...)
					}
				})
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						p := f(tt.p...)
						p.Wait()
					}
				})
			}
		})
	case multiIdxFunc:
		b.Run(funName+"-Parallel", func(b *testing.B) {
			tt := compileTestCase(b, bc, nil)

			if !addWait {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						f(tt.p...)
					}
				})
			} else {
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						p := f(tt.p...)
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
				helperBenchmark[waitFunc](b, bc, Wait, "Wait", false)
				helperBenchmarkParallel[waitFunc](b, bc, Wait, "Wait", false)
			})

			b.Run("Select", func(b *testing.B) {
				helperBenchmark[singleIdxFunc](b, bc, Select, "Select", false)
				helperBenchmarkParallel[singleIdxFunc](b, bc, Select, "Select", false)
				helperBenchmark[singleIdxFunc](b, bc, Select, "Select-Wait", true)
				helperBenchmarkParallel[singleIdxFunc](b, bc, Select, "Select-Wait", true)
			})

			b.Run("AllWait", func(b *testing.B) {
				helperBenchmark[multiIdxFunc](b, bc, AllWait, "AllWait", false)
				helperBenchmarkParallel[multiIdxFunc](b, bc, AllWait, "AllWait", false)
				helperBenchmark[multiIdxFunc](b, bc, AllWait, "AllWait-Wait", true)
				helperBenchmarkParallel[multiIdxFunc](b, bc, AllWait, "AllWait-Wait", true)
			})

			b.Run("AnyWait", func(b *testing.B) {
				helperBenchmark[multiIdxFunc](b, bc, AnyWait, "AnyWait", false)
				helperBenchmarkParallel[multiIdxFunc](b, bc, AnyWait, "AnyWait", false)
				helperBenchmark[multiIdxFunc](b, bc, AnyWait, "AnyWait-Wait", true)
				helperBenchmarkParallel[multiIdxFunc](b, bc, AnyWait, "AnyWait-Wait", true)
			})
		})
	}
}

func helperBenchmarkLeakDetector[TFun extFunc](
	b *testing.B,
	bc caseConfig,
	fun TFun,
	funName string,
) {
	b.Run(funName, func(b *testing.B) {
		debugMetrics, debugCB := createDebugMetricsCB()
		g := NewGroup[string](UnhandledErrorCB(func(err error) {}))
		g.core.debugCB = debugCB

		tt := compileTestCase(b, bc, g)

		b.ReportAllocs()

		switch f := any(fun).(type) {
		case waitFunc:
			b.ResetTimer()
			for range b.N {
				f(tt.p...)
			}
			b.StopTimer()
		case singleIdxFunc:
			b.ResetTimer()
			for range b.N {
				p := f(tt.p...)
				p.Wait()
			}
			b.StopTimer()
		case multiIdxFunc:
			b.ResetTimer()
			for range b.N {
				p := f(tt.p...)
				p.Wait()
			}
			b.StopTimer()
		}

		if testEnableLogs {
			b.Log(debugMetrics.String())
		}

		// at this point, all counters up to the ext-calls related should
		// report correct values, so validate that...
		expected := int64(bc.async.resultSize())
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

// BenchmarkExtFuncs_LeakDetector is only concerned with [Wait], [AllWait]
// and [AnyWait], as it requires an extension function that blocks until
// all ongoing promises have finished.
func BenchmarkExtFuncs_LeakDetector(b *testing.B) {
	// skip based on the present build tags.
	if !debugEnabled {
		b.Skip("the 'enable_promise_debug' build tag isn't enabled")
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			helperBenchmarkLeakDetector[waitFunc](b, bc, Wait, "Wait")
			helperBenchmarkLeakDetector[multiIdxFunc](b, bc, AllWait, "AllWait")
			helperBenchmarkLeakDetector[multiIdxFunc](b, bc, AnyWait, "AnyWait")
		})
	}
}
