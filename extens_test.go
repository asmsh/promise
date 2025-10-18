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
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"
)

var testEnableLogs = false

func getTestAsyncPromFulfilled(g *Group[string], id int) *Promise[string] {
	d := 5 * time.Millisecond
	if g != nil {
		g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			return ValRes(fmt.Sprintf("%d hello world async %s", id, d))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		return ValRes(fmt.Sprintf("%d hello world async %s", id, d))
	})
}
func getTestAsyncPromFulfilledDelay(g *Group[string], id int) *Promise[string] {
	d := 1000 * time.Millisecond
	if g != nil {
		return g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			return ValRes(fmt.Sprintf("%d hello world async %s", id, d))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		return ValRes(fmt.Sprintf("%d hello world async %s", id, d))
	})
}

func getTestAsyncPromRejected(g *Group[string], id int) *Promise[string] {
	d := 5 * time.Millisecond
	if g != nil {
		return g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			return ErrRes[string](errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		return ErrRes[string](errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
	})
}
func getTestAsyncPromRejectedDelay(g *Group[string], id int) *Promise[string] {
	d := 1000 * time.Millisecond
	if g != nil {
		return g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			return ErrRes[string](errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		return ErrRes[string](errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
	})
}

func getTestAsyncPromPanicked(g *Group[string], id int) *Promise[string] {
	d := 5 * time.Millisecond
	if g != nil {
		return g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			panic(errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		panic(errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
	})
}
func getTestAsyncPromPanickedDelay(g *Group[string], id int) *Promise[string] {
	d := 1000 * time.Millisecond
	if g != nil {
		return g.GoCtxRes(func(ctx context.Context) Result[string] {
			time.Sleep(d)
			panic(errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
		})
	}
	return GoCtxRes(func(ctx context.Context) Result[string] {
		time.Sleep(d)
		panic(errors.New(fmt.Sprintf("%d hello world async %s", id, d)))
	})
}

var (
	syncPromFulfilled1 = Wrap[string](ValRes("hello world sync 1"))
	syncPromRejected1  = Wrap[string](ValErrRes("err world sync 1", errors.New("wrap error")))
	syncPromPanicked1  = Wrap[string](PanicRes[string](errors.New("panic error")))
)

type testCase struct {
	name string
	p    []*Promise[string]

	extResState extensionsResultState
}

type generateAsyncPromisesConfig struct {
	// totalNum is the total number of randomly generated async promises
	totalNum int

	// if any of the below is provided, the totalNum is ignored.
	fulfilledNum      int
	fulfilledDelayNum int
	rejectedNum       int
	rejectedDelayNum  int
	panickedNum       int
	panickedDelayNum  int
}

func (g generateAsyncPromisesConfig) resultSize() int {
	if g.fulfilledNum != 0 || g.fulfilledDelayNum != 0 ||
		g.rejectedNum != 0 || g.rejectedDelayNum != 0 ||
		g.panickedNum != 0 || g.panickedDelayNum != 0 {
		return g.fulfilledNum + g.fulfilledDelayNum +
			g.rejectedNum + g.rejectedDelayNum +
			g.panickedNum + g.panickedDelayNum
	}
	return g.totalNum
}

func (g generateAsyncPromisesConfig) validate() {
	// if any of the specific values is provided, the totalNum must be 0
	if g.fulfilledNum != 0 || g.fulfilledDelayNum != 0 ||
		g.rejectedNum != 0 || g.rejectedDelayNum != 0 ||
		g.panickedNum != 0 || g.panickedDelayNum != 0 {
		if g.totalNum != 0 {
			panic(fmt.Sprintf("bad generateAsyncPromisesConfig: unexpectedly totalNum=%d", g.totalNum))
		}
	}
}

type generateSyncPromisesConfig struct {
	// totalNum is the total number of randomly generated sync promises
	totalNum int

	// if any of the below is provided, the totalNum is ignored.
	fulfilledNum int
	rejectedNum  int
	panickedNum  int
}

func (g generateSyncPromisesConfig) resultSize() int {
	if g.fulfilledNum != 0 || g.rejectedNum != 0 || g.panickedNum != 0 {
		return g.fulfilledNum + g.rejectedNum + g.panickedNum
	}
	return g.totalNum
}

func (g generateSyncPromisesConfig) validate() {
	// if any of the specific values is provided, the totalNum must be 0
	if g.fulfilledNum != 0 || g.rejectedNum != 0 || g.panickedNum != 0 {
		if g.totalNum != 0 {
			panic(fmt.Sprintf("bad generateSyncPromisesConfig: unexpectedly totalNum=%d", g.totalNum))
		}
	}
}

type extensionsResultState struct {
	selectState  State
	allState     State
	allWaitState State
	anyState     State
	anyWaitState State
	joinState    State
}

type caseConfig struct {
	name string

	async generateAsyncPromisesConfig
	sync  generateSyncPromisesConfig

	extResState extensionsResultState
}

func compileTestCases(configs []caseConfig, g *Group[string]) []testCase {
	// used as a randomizer below
	closedChan := make(chan struct{})
	close(closedChan)

	cases := make([]testCase, 0, len(configs))
	for _, c := range configs {
		// validate the configs
		c.async.validate()
		c.sync.validate()

		// create the result promise slice
		p := make([]*Promise[string], 0, c.async.resultSize()+c.sync.resultSize())

		// generate the required promises...
		// generate async promises
		genAsync := 0
		if c.async.totalNum != 0 {
			for i := 0; i < c.async.totalNum; i++ {
				select {
				case <-closedChan:
					p = append(p, getTestAsyncPromFulfilled(g, genAsync))
				case <-closedChan:
					p = append(p, getTestAsyncPromFulfilledDelay(g, genAsync))
				case <-closedChan:
					p = append(p, getTestAsyncPromRejected(g, genAsync))
				case <-closedChan:
					p = append(p, getTestAsyncPromRejectedDelay(g, genAsync))
				case <-closedChan:
					p = append(p, getTestAsyncPromPanicked(g, genAsync))
				case <-closedChan:
					p = append(p, getTestAsyncPromPanickedDelay(g, genAsync))
				}
				genAsync++
			}
		} else {
			for i := 0; i < c.async.fulfilledNum; i++ {
				p = append(p, getTestAsyncPromFulfilled(g, genAsync))
				genAsync++
			}
			for i := 0; i < c.async.fulfilledDelayNum; i++ {
				p = append(p, getTestAsyncPromFulfilledDelay(g, genAsync))
				genAsync++
			}
			for i := 0; i < c.async.rejectedNum; i++ {
				p = append(p, getTestAsyncPromRejected(g, genAsync))
				genAsync++
			}
			for i := 0; i < c.async.rejectedDelayNum; i++ {
				p = append(p, getTestAsyncPromRejectedDelay(g, genAsync))
				genAsync++
			}
			for i := 0; i < c.async.panickedNum; i++ {
				p = append(p, getTestAsyncPromPanicked(g, genAsync))
				genAsync++
			}
			for i := 0; i < c.async.panickedDelayNum; i++ {
				p = append(p, getTestAsyncPromPanickedDelay(g, genAsync))
				genAsync++
			}
		}
		if want := c.async.resultSize(); want != genAsync {
			panic(fmt.Sprintf("invalid generated number of async promises: want=%d, got=%d", want, genAsync))
		}

		// generate sync promises
		genSync := 0
		if c.sync.totalNum != 0 {
			for i := 0; i < c.sync.totalNum; i++ {
				select {
				case <-closedChan:
					p = append(p, syncPromFulfilled1)
				case <-closedChan:
					p = append(p, syncPromRejected1)
				case <-closedChan:
					p = append(p, syncPromPanicked1)
				}
				genSync++
			}
		} else {
			for i := 0; i < c.sync.fulfilledNum; i++ {
				p = append(p, syncPromFulfilled1)
				genSync++
			}
			for i := 0; i < c.sync.rejectedNum; i++ {
				p = append(p, syncPromRejected1)
				genSync++
			}
			for i := 0; i < c.sync.panickedNum; i++ {
				p = append(p, syncPromPanicked1)
				genSync++
			}
		}
		if want := c.sync.resultSize(); want != genSync {
			panic(fmt.Sprintf("invalid generated number of sync promises: want=%d, got=%d", want, genSync))
		}

		cases = append(cases, testCase{
			name:        c.name,
			p:           p,
			extResState: c.extResState,
		})
	}
	return cases
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		p    *Promise[any]
		ip   *Promise[IdxRes[any]]
		ips  *Promise[[]IdxRes[any]]
	}{
		{
			name: "GoFunc[any, any]",
			p: GoFunc[any, any](func() error {
				return newStrError()
			}),
		},
		{
			name: "Select",
			ip: Select(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
		{
			name: "All",
			ips: All(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
		{
			name: "AllWait",
			ips: AllWait(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
		{
			name: "Any",
			ips: Any(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
		{
			name: "AnyWait",
			ips: AnyWait(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
		{
			name: "Join",
			ips: Join(
				GoFunc[any, any](func() error {
					return newStrError()
				}),
				GoFunc[any, any](func() error {
					return newStrError()
				}),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.p != nil {
				r := test.p.Res()

				if testEnableLogs {
					t.Logf("p res: %v {%T}\n", r, r)
					t.Logf("p val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("p err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("p state: %v {%T}\n", r.State(), r.State())
				}
			} else if test.ip != nil {
				r := test.ip.Res()

				if testEnableLogs {
					t.Logf("ip res: %v {%T}\n", r, r)
					t.Logf("ip val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ip err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ip state: %v {%T}\n", r.State(), r.State())
				}
			} else {
				r := test.ips.Res()

				if testEnableLogs {
					t.Logf("ips res: %v {%T}\n", r, r)
					t.Logf("ips val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ips err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ips state: %v {%T}\n", r.State(), r.State())
				}
			}
		})
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
			name: "GoFunc[any, any]",
			p: func() *Promise[any] {
				return GoFunc[any, any](func() error {
					panic(newStrError())
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
						panic(newStrError())
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
						panic(newStrError())
					}),
					GoFunc[any, any](func() error {
						panic(newStrError())
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
						panic(newStrError())
					}),
					GoFunc[any, any](func() error {
						panic(newStrError())
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
						panic(newStrError())
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
						panic(newStrError())
					}),
					GoFunc[any, any](func() error {
						time.Sleep(time.Second)
						return nil
					}),
					GoFunc[any, any](func() error {
						return newStrError()
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
						panic(newStrError())
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

				err = r.Err()

				p.Recover(func(ctx context.Context, res Result[any]) Result[any] {
					if testEnableLogs {
						t.Logf("p recover res: %v {%T}\n", res, res)
						t.Logf("p recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("p recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("p recover state: %v {%T}\n", res.State(), res.State())
					}

					return nil
				}).Wait()
			} else if test.ip != nil {
				ip := test.ip()
				r := ip.Res()
				if testEnableLogs {
					t.Logf("ip res: %v {%T}\n", r, r)
					t.Logf("ip val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ip err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ip state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err()

				ip.Recover(func(ctx context.Context, res Result[IdxRes[any]]) Result[IdxRes[any]] {
					if testEnableLogs {
						t.Logf("ip recover res: %v {%T}\n", res, res)
						t.Logf("ip recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ip recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ip recover state: %v {%T}\n", res.State(), res.State())
					}

					return nil
				}).Wait()
			} else {
				ips := test.ips()
				r := ips.Res()

				if testEnableLogs {
					t.Logf("ips res: %v {%T}\n", r, r)
					t.Logf("ips val: %v {%T}\n", r.Val(), r.Val())
					t.Logf("ips err: %v {%T}\n", r.Err(), r.Err())
					t.Logf("ips state: %v {%T}\n", r.State(), r.State())
				}

				err = r.Err()

				newIps := ips.Recover(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
					// FIXME: how to allow filtering the 'val' value, to eliminate the panic result?
					//  in case the same value 'val' is returned, we should resolve to panic.
					if testEnableLogs {
						t.Logf("ips recover res: %v {%T}\n", res, res)
						t.Logf("ips recover val: %v {%T}\n", res.Val(), res.Val())
						t.Logf("ips recover err: %v {%T}\n", res.Err(), res.Err())
						t.Logf("ips recover state: %v {%T}\n", res.State(), res.State())
					}

					return res
				})

				rNew := newIps.Res()

				if testEnableLogs {
					t.Logf("newIps res: %v {%T}\n", rNew, rNew)
					t.Logf("newIps val: %v {%T}\n", rNew.Val(), rNew.Val())
					t.Logf("newIps err: %v {%T}\n", rNew.Err(), rNew.Err())
					t.Logf("newIps state: %v {%T}\n", rNew.State(), rNew.State())
				}
			}

			if test.noPanic {
				return
			}

			if !errors.Is(err, ErrPromisePanicked) {
				t.Errorf("err: %v doesn't implement ErrPromisePanicked\n", err)
			}

			if !test.externalPanic {
				if !errors.Is(err, newStrError()) {
					t.Errorf("err: %v doesn't implement newStrError\n", err)
				}
			}
		})
	}
}

var (
	testsCases = []caseConfig{
		{
			async: generateAsyncPromisesConfig{
				fulfilledNum:     1,
				rejectedDelayNum: 32,
				panickedDelayNum: 0,
			},
			extResState: extensionsResultState{
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
				fulfilledDelayNum: 32,
				rejectedNum:       1,
				panickedDelayNum:  0,
			},
			extResState: extensionsResultState{
				selectState:  Error,
				allState:     Error,
				allWaitState: Error,
				anyState:     Success,
				anyWaitState: Success,
				joinState:    Success,
			},
		},
		// TODO: add more test cases...
	}
)

func TestSelect(t *testing.T) {
	for _, tt := range compileTestCases(testsCases, nil) {
		t.Run(tt.name, func(t *testing.T) {
			p := Select[string](tt.p...)
			if p.State() != tt.extResState.selectState {
				t.Errorf("Select: want %v got %v", tt.extResState.selectState, p.State())
			}
			p.Wait()
		})
	}
}

func TestAll(t *testing.T) {
	t.Run("All", func(t *testing.T) {
		for _, tt := range compileTestCases(testsCases, nil) {
			t.Run(tt.name, func(t *testing.T) {
				p := All[string](tt.p...)
				if p.State() != tt.extResState.allState {
					t.Errorf("All: want %v got %v", tt.extResState.allState, p.State())
				}
			})
		}
	})
	t.Run("AllWait", func(t *testing.T) {
		for _, tt := range compileTestCases(testsCases, nil) {
			t.Run(tt.name, func(t *testing.T) {
				p := AllWait[string](tt.p...)
				if p.State() != tt.extResState.allWaitState {
					t.Errorf("AllWait: want %v got %v", tt.extResState.allWaitState, p.State())
				}
			})
		}
	})
}

func TestAny(t *testing.T) {
	t.Run("Any", func(t *testing.T) {
		for _, tt := range compileTestCases(testsCases, nil) {
			t.Run(tt.name, func(t *testing.T) {
				p := Any[string](tt.p...)
				if p.State() != tt.extResState.anyState {
					t.Errorf("Any: want %v got %v", tt.extResState.anyState, p.State())
				}
			})
		}
	})
	t.Run("AnyWait", func(t *testing.T) {
		for _, tt := range compileTestCases(testsCases, nil) {
			t.Run(tt.name, func(t *testing.T) {
				p := AnyWait[string](tt.p...)
				if p.State() != tt.extResState.anyWaitState {
					t.Errorf("AnyWait: want %v got %v", tt.extResState.anyWaitState, p.State())
				}
			})
		}
	})
}

func TestJoin(t *testing.T) {
	for _, tt := range compileTestCases(testsCases, nil) {
		t.Run(tt.name, func(t *testing.T) {
			p := Join[string](tt.p...)
			if p.State() != tt.extResState.joinState {
				t.Errorf("Join: want %v got %v", tt.extResState.joinState, p.State())
			}
		})
	}
}

func TestJoin2(t *testing.T) {
	log.SetFlags(log.Lmicroseconds)

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
		join := Join(p1).Then(func(ctx context.Context, res Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
			for _, v := range res.Val() {
				if testEnableLogs {
					t.Logf("[%T] %v", v.Result, v)
				}
			}
			return nil
		})
		join.Wait()
	})
}

var (
	benchmarkCases = []caseConfig{
		{
			name: "16:8-async-8-sync",
			async: generateAsyncPromisesConfig{
				totalNum: 8,
			},
			sync: generateSyncPromisesConfig{
				totalNum: 8,
			},
		},
		{
			name: "100:50-async-50-sync",
			async: generateAsyncPromisesConfig{
				totalNum: 50,
			},
			sync: generateSyncPromisesConfig{
				totalNum: 50,
			},
		},
		{
			name: "16-async",
			async: generateAsyncPromisesConfig{
				totalNum: 16,
			},
		},
		{
			name: "100-async",
			async: generateAsyncPromisesConfig{
				totalNum: 100,
			},
		},
		{
			name: "16-sync",
			sync: generateSyncPromisesConfig{
				totalNum: 16,
			},
		},
		{
			name: "100-sync",
			sync: generateSyncPromisesConfig{
				totalNum: 100,
			},
		},
	}
)

func BenchmarkSelect(b *testing.B) {
	benchmarks := compileTestCases(benchmarkCases, nil)

	for _, tt := range benchmarks {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Select[string](tt.p...)
			}
		})

		b.Run(tt.name+"-parallel", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					Select[string](tt.p...)
				}
			})
		})

		b.Run(tt.name+"-Wait", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				selectedProm := Select[string](tt.p...)
				selectedProm.Wait()
			}
		})

		b.Run(tt.name+"-parallel-Wait", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					selectedProm := Select[string](tt.p...)
					selectedProm.Wait()
				}
			})
		})
	}
}

func BenchmarkAllWait(b *testing.B) {
	benchmarks := compileTestCases(benchmarkCases, nil)

	for _, tt := range benchmarks {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				AllWait[string](tt.p...)
			}
		})

		b.Run(tt.name+"-parallel", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					AllWait[string](tt.p...)
				}
			})
		})

		b.Run(tt.name+"-Wait", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				allProm := AllWait[string](tt.p...)
				allProm.Wait()
			}
		})

		b.Run(tt.name+"-parallel-Wait", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					allProm := AllWait[string](tt.p...)
					allProm.Wait()
				}
			})
		})
	}
}

func BenchmarkAnyWait(b *testing.B) {
	benchmarks := compileTestCases(benchmarkCases, nil)

	for _, tt := range benchmarks {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				AnyWait[string](tt.p...)
			}
		})

		b.Run(tt.name+"-parallel", func(b *testing.B) {
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					AnyWait[string](tt.p...)
				}
			})
		})

		b.Run(tt.name+"-Wait", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				anyProm := AnyWait[string](tt.p...)
				anyProm.Wait()
			}
		})

		b.Run(tt.name+"-parallel-Wait", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					anyProm := AnyWait[string](tt.p...)
					anyProm.Wait()
				}
			})
		})
	}
}

func BenchmarkAllWait_LeakDetector(b *testing.B) {
	// skip based on the present build tags.
	if !debugEnabled {
		b.Skip("the 'enable_promise_debug' build tag isn't enabled")
	}

	log.SetFlags(log.Lmicroseconds)

	debugMetrics, debugCB := createDebugMetricsCB()
	g := NewGroup[string](UnhandledErrorCB(func(err error) {}))
	g.core.debugCB = debugCB
	benchmarks := compileTestCases(
		[]caseConfig{
			{
				async: generateAsyncPromisesConfig{
					rejectedNum: 2000,
					//rejectedDelayNum: 2000,
				},
			},
		},
		g,
	)

	for _, tt := range benchmarks {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				allProm := AllWait[string](tt.p...)
				allProm.Wait()
			}
		})

		// validate the got debug metrics vs the expected for this test.
		expected := int64(len(tt.p))
		startedCount := debugMetrics.get(startHandler)
		endedCount := debugMetrics.get(endHandler)
		resolvedCount := debugMetrics.get(resolve)
		detectedCount := debugMetrics.get(callUnhandledErrorCallback)
		foundExtChanCount := debugMetrics.get(foundExtChan)
		foundExtQueueCount := debugMetrics.get(foundExtQueue)
		debugMetricsStr := debugMetrics.String()
		if expected != startedCount || expected != endedCount || expected != resolvedCount || expected != detectedCount {
			b.Errorf(
				"Possible goroutine leaks from AllWait: expected: %d, started: %d, ended: %d, resolved: %d, detected: %d",
				expected,
				startedCount,
				endedCount,
				resolvedCount,
				detectedCount,
			)
		}
		if foundExtChanCount != foundExtQueueCount {
			b.Errorf(
				"Possible goroutine leaks from AllWait handleExtCalls: foundExtChanCount: %d, foundExtQueueCount: %d",
				foundExtChanCount,
				foundExtQueueCount,
			)
		}

		b.Log(debugMetricsStr)
	}
}
