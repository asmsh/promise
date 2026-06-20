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

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

func TestFollow(t *testing.T) {
	t.Run("standard Result", func(t *testing.T) {
		pAny := GoFunc[*int, any](func(ctx context.Context) Result[*int] {
			v := 10
			return ValRes(&v)
		})
		pStr := Follow[string](
			pAny,
			func(ctx context.Context, res Result[*int]) Result[string] {
				return ValRes(fmt.Sprintf("%d", *res.Val()))
			},
		)
		pInt := Follow[int](
			pStr,
			func(ctx context.Context, res Result[string]) Result[int] {
				return ValErrRes(strconv.Atoi(res.Val()))
			},
		)

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		if testEnableLogs {
			t.Logf("pAny: %+v\n", pAny.WaitRes())
			t.Logf("pStr: %+v\n", pStr.WaitRes())
			t.Logf("pInt: %+v\n", pInt.WaitRes())
		}
	})

	t.Run("custom Result", func(t *testing.T) {
		pAny := GoFunc[*int, any](func(ctx context.Context) Result[*int] {
			v := 10
			return ValRes(&v)
		})
		pStr := Follow[string](
			pAny,
			func(ctx context.Context, res Result[*int]) Result[string] {
				return newResult(fmt.Sprintf("%d", *res.Val()), nil, Success)
			},
		)
		pInt := Follow[int](
			pStr,
			func(ctx context.Context, res Result[string]) Result[int] {
				return ValErrRes(strconv.Atoi(res.Val()))
			},
		)

		if want, got := 10, pAny.Val(); !reflect.DeepEqual(got, &want) {
			t.Errorf("pAny.Val(): %v, want: %v\n", *got, want)
		}
		if want, got := "10", pStr.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pStr.Val(): %v, want: %v\n", want, got)
		}
		if want, got := 10, pInt.Val(); !reflect.DeepEqual(got, want) {
			t.Errorf("pInt.Val(): %v, want: %v\n", want, got)
		}

		if testEnableLogs {
			t.Logf("pAny: %+v\n", pAny.WaitRes())
			t.Logf("pStr: %+v\n", pStr.WaitRes())
			t.Logf("pInt: %+v\n", pInt.WaitRes())
		}
	})
}

func BenchmarkFollow(b *testing.B) {
	var pStr *Promise[int]
	pBase := GoFunc[string, any](func(ctx context.Context) Result[string] {
		return ValRes("10")
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pStr = Follow[int](
			pBase,
			func(ctx context.Context, res Result[string]) int {
				return 10
			},
		)
	}
	_ = pStr
}
