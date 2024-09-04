// Copyright 2020 Ahmad Sameh(asmsh)
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
	"testing"
)

// testStrError is an error implementation that's used only for testing.
// it's a string to allow comparing its values.
type testStrError string

func (t testStrError) Error() string {
	return string(t)
}

func newStrError() error {
	return testStrError("str_test_error")
}

// testPtrError is an error implementation that's used only for testing.
// it's a pointer-based error, to mimick most error structures in real-scenarios.
type testPtrError struct {
	txt string
}

func (t *testPtrError) Error() string {
	return t.txt
}

func newPtrError() error {
	return &testPtrError{txt: "ptr_test_error"}
}

func TestPanicking(t *testing.T) {
	panicValue := "test_panic"

	t.Run("callback handling", func(t *testing.T) {
		defer func() {
			v := recover()
			if v != nil {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := Go(func() {
			panic(panicValue)
		}).Recover(func(ctx context.Context, _ any, v any) (res Result[any]) {
			return nil
		})
		p.Wait()
	})

	t.Run("Res handling", func(t *testing.T) {
		defer func() {
			v := recover()
			if v != nil {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := Go(func() {
			panic(panicValue)
		})
		res := p.Res()
		if res == nil {
			t.Fatalf("Res() = %v, want: non-nil", res)
		}
		if vv, ok := res.Err().(PanicError); !ok || vv.V != panicValue {
			t.Fatalf("Res() got unexpected error: %v", res.Err())
		}
	})

	t.Run("non-recovered panic 1", func(t *testing.T) {
		defer func() {
			v := recover()
			if v == nil {
				t.Fatal("expected a panic, but none happened")
			}
			if vv, ok := v.(PanicError); !ok || vv.V != panicValue {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := Go(func() {
			panic(panicValue)
		})
		p.Wait()
	})

	t.Run("non-recovered panic 2", func(t *testing.T) {
		defer func() {
			v := recover()
			if v == nil {
				t.Fatal("expected a panic, but none happened")
			}
			if vv, ok := v.(PanicError); !ok || vv.V != panicValue {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := Go(func() {
			panic(panicValue)
		}).Then(func(ctx context.Context, val any) Result[any] {
			return nil
		})
		p.Wait()
	})
}

func TestRejection(t *testing.T) {
	t.Run("callback handling", func(t *testing.T) {
		defer func() {
			v := recover()
			if v != nil {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := GoRes(func(ctx context.Context) Result[any] {
			return Err[any](newStrError())
		}).Catch(func(ctx context.Context, val any, err error) Result[any] {
			// handle the error...
			return nil
		})
		p.Wait()
	})

	t.Run("Res handling", func(t *testing.T) {
		wantErr := newStrError()
		defer func() {
			v := recover()
			if v != nil {
				t.Fatalf("got unexpected panic: %v", v)
			}
		}()

		p := GoRes(func(ctx context.Context) Result[any] {
			return Err[any](wantErr)
		})
		res := p.Res()
		if res == nil {
			t.Errorf("Res() = nil, want: non-nil")
		}
		if res.Err() == nil {
			t.Errorf("Res().Err() = nil, want: non-nil")
		}
		if err := res.Err(); !errors.Is(err, wantErr) {
			t.Errorf("Res().Err() = %v, want: %v", err, wantErr)
		}
	})
}
