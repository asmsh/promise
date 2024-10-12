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

type debugEvent int

const (
	_ debugEvent = iota

	resolve
	resolveFulfilled
	resolveRejected
	resolvePanicked

	startExtCall
	endExtCall
	foundExtChan
	missingExtChan
	foundExtQueue

	startUncaughtPanicHandler
	callUncaughtPanicHandler
	startUncaughtErrorHandler
	callUncaughtErrorHandler

	// handler = goroutine
	// note: in a program that doesn't call the Group.WaitX methods,
	// all endXHandler aren't guaranteed to match their startXXHandler,
	// because the endXHandler event is fired last thing in the flow,
	// and if the program is exiting without any arrangement to wait
	// for running promises, this event might never get to be fired.
	startHandler
	endHandler

	// constructor (functions) values
	// start values...
	startConstrHandler
	startConstrChanHandler
	startConstrGoHandler
	startConstrGoErrHandler
	startConstrGoResHandler
	startConstrDelayHandler
	// end values...
	endConstrHandler
	endConstrChanHandler
	endConstrGoHandler
	endConstrGoErrHandler
	endConstrGoResHandler
	endConstrDelayHandler

	// follow (methods) values
	// start values...
	startFollowHandler
	startCallbackFollowHandler
	startDelayFollowHandler
	startThenFollowHandler
	startCatchFollowHandler
	startRecoverFollowHandler
	startFinallyFollowHandler
	// end values...
	endFollowHandler
	endCallbackFollowHandler
	endDelayFollowHandler
	endThenFollowHandler
	endCatchFollowHandler
	endRecoverFollowHandler
	endFinallyFollowHandler
)
