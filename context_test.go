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
	"testing"
)

type testContext struct {
	context.Context
}

func newTestContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return &testContext{ctx}, cancel
}
func (t *testContext) String() string { return "testContext" }

var testCtx, testCtxCancel = newTestContext()

var testsCases_Group_callbackCtx = []struct {
	name            string
	g               *Group[any]
	syncCtx         context.Context
	expectedCtxName string
}{
	// Callback cases (nil syncCtx is passes)...
	{
		// when no Group is available.
		name:            "Callback:nil_group,nil_syncCtx",
		g:               nil,
		syncCtx:         nil,
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=false.
		name:            "Callback:non-nil_group,nil_group_ctx,cancel-callback-ctx,nil_syncCtx",
		g:               &Group[any]{core: &groupCore{}},
		syncCtx:         nil,
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=true.
		name: "Callback:non-nil_group,nil_group_ctx,never-cancel-callback-ctx,nil_syncCtx",
		g: &Group[any]{core: &groupCore{
			options: func() groupOptionsBitFlags {
				options := groupOptionsBitFlags(0)
				options.SetNeverCancelCBCtx()
				return options
			}(),
		}},
		syncCtx:         nil,
		expectedCtxName: "context.Background",
	},
	{
		// when a Group is available with the group Context set.
		name: "Callback:non-nil_group,non-nil_group_ctx,cancel-callback-ctx,nil_syncCtx",
		g: &Group[any]{core: &groupCore{
			ctx:    testCtx,
			cancel: testCtxCancel,
		}},
		syncCtx:         nil,
		expectedCtxName: "testContext.WithCancel",
	},

	// Follow cases (non-nil syncCtx is passes)...
	{
		// when no Group is available.
		name:            "Follow:nil_group,non-nil_syncCtx",
		g:               nil,
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=false.
		name:            "Follow:non-nil_group,nil_group_ctx,cancel-callback-ctx,non-nil_syncCtx",
		g:               &Group[any]{core: &groupCore{}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "syncCtx",
	},
	{
		// when a Group is available with neverCancelCBCtx=true.
		name: "Follow:non-nil_group,nil_group_ctx,never-cancel-callback-ctx,non-nil_syncCtx",
		g: &Group[any]{core: &groupCore{
			options: func() groupOptionsBitFlags {
				options := groupOptionsBitFlags(0)
				options.SetNeverCancelCBCtx()
				return options
			}(),
		}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "context.Background",
	},
	{
		// when a Group is available with the group Context set.
		name: "Follow:non-nil_group,non-nil_group_ctx,cancel-callback-ctx,non-nil_syncCtx",
		g: &Group[any]{core: &groupCore{
			ctx:    testCtx,
			cancel: testCtxCancel,
		}},
		syncCtx:         syncCtx{syncChan: make(chan struct{})},
		expectedCtxName: "testContext.WithCancel",
	},
}

func TestGroup_callbackCtx(t *testing.T) {
	for _, test := range testsCases_Group_callbackCtx {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := callbackCtx(test.g, test.syncCtx)
			if ctx == nil {
				t.Errorf("nil ctx")
			}
			if cancel == nil {
				t.Errorf("nil cancel")
			}
			ctxName := fmt.Sprintf("%s", ctx)
			if ctxName != test.expectedCtxName {
				t.Errorf("ctxName is %s, want %s", ctxName, test.expectedCtxName)
			}
		})
	}
}

func BenchmarkGroup_callbackCtx(b *testing.B) {
	for _, bm := range testsCases_Group_callbackCtx {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctx, cancel := callbackCtx(bm.g, bm.syncCtx)
				if ctx == nil {
					b.Errorf("nil ctx")
				}
				if cancel == nil {
					b.Errorf("nil cancel")
				}
			}
		})
	}
}
