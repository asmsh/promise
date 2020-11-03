package promise_test

import (
	"reflect"
	"testing"

	"github.com/asmsh/promise"
)

func TestImmutRes(t *testing.T) {
	res := promise.Res{"go", "go"}
	newRes := promise.ImmutRes(res...)
	res[0], res[1] = "golang", "golang"

	if equalRess(res, newRes) {
		t.Errorf("ImmutRes returned a mutable Res value")
	}
}

func TestReuseRes(t *testing.T) {
	tests := []struct {
		name        string
		base        promise.Res
		updatedBase promise.Res
		vals        []interface{}
		wantRes     promise.Res
		reused      bool
	}{
		{
			name:        "nil res, nil vals",
			base:        nil,
			updatedBase: nil,
			vals:        nil,
			wantRes:     nil,
			reused:      false,
		}, {
			name:        "empty res, nil vals",
			base:        promise.Res{},
			updatedBase: promise.Res{},
			vals:        nil,
			wantRes:     nil,
			reused:      false,
		}, {
			name:        "2 element res, nil vals",
			base:        promise.Res{"go", "golang"},
			updatedBase: promise.Res{"go", "golang"},
			vals:        nil,
			wantRes:     nil,
			reused:      false,
		}, {
			name:        "nil res, with 2 vals",
			base:        nil,
			updatedBase: nil,
			vals:        []interface{}{"golang", "golang"},
			wantRes:     promise.Res{"golang", "golang"},
			reused:      false,
		}, {
			name:        "2 element res, with 2 vals",
			base:        promise.Res{"go", "go"},
			updatedBase: promise.Res{"golang", "golang"},
			vals:        []interface{}{"golang", "golang"},
			wantRes:     promise.Res{"golang", "golang"},
			reused:      true,
		}, {
			name:        "2 element res, with 1 val",
			base:        promise.Res{"go", "go"},
			updatedBase: promise.Res{"golang", nil},
			vals:        []interface{}{"golang"},
			wantRes:     promise.Res{"golang"},
			reused:      true,
		}, {
			name:        "2 element res, with 3 vals",
			base:        promise.Res{"go", "go"},
			updatedBase: promise.Res{"go", "go"},
			vals:        []interface{}{"golang", "golang", "golang"},
			wantRes:     promise.Res{"golang", "golang", "golang"},
			reused:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes := promise.ReuseRes(tt.base, tt.vals...)
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("ReuseRes() = %v, want %v", gotRes, tt.wantRes)
			}

			if tt.wantRes != nil {
				gotPtr := reflect.ValueOf(gotRes).Pointer()
				basePtr := reflect.ValueOf(tt.base).Pointer()
				if tt.reused {
					if gotPtr != basePtr {
						t.Errorf("ReuseRes didn't reuse the base Res value")
					}
				} else {
					if gotPtr == basePtr {
						t.Errorf("ReuseRes reused the base Res value")
					}
				}
			}

			if !equalRess(tt.base, tt.updatedBase) {
				t.Errorf("ReuseRes updated the base unexpectedly")
			}
		})
	}
}
