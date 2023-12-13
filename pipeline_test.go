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
	"sync"
)

// setNoPanicsPipelineCore should be called at the beginning of every benchmark/test
// function where it's expected to return errors.
// it's required so that the Promise doesn't panic according to the default handlers' logic.
var setNoPanicsPipelineCore = sync.OnceFunc(func() {
	// override the default handlers, to avoid panics during benchmarks
	defPipelineCore = &pipelineCore{
		uncaughtPanicHandler: func(v UncaughtPanic) {},
		uncaughtErrorHandler: func(v UncaughtError) {},
	}
})
