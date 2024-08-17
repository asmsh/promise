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
	"time"
)

type syncCtx struct {
	syncChan chan struct{}
}

func (s syncCtx) Deadline() (deadline time.Time, ok bool) { return }
func (s syncCtx) Done() <-chan struct{}                   { return s.syncChan }
func (s syncCtx) Err() error                              { return nil }
func (s syncCtx) Value(any) any                           { return nil }

func closeSyncCtx(ctx context.Context) {
	s := ctx.(syncCtx)
	close(s.syncChan)
}
