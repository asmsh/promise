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
	resolveSuccess
	resolveError
	resolvePanic

	startHandleExtCalls
	endHandleExtCalls
	foundExtChan
	missingExtChan
	foundExtQueue
	emptyExtQueue
	doneHandleExtCall

	startHandleGroupCalls
	endHandleGroupCalls
	doneHandleGroupCall
	doneSaveGroupResult

	startUnhandledPanicLogic
	callUnhandledPanicCallback
	startUnhandledErrorLogic
	callUnhandledErrorCallback

	// handler = goroutine
	// note: in a program that doesn't call the Group.WaitX methods,
	// all endXHandler aren't guaranteed to match their startXXHandler,
	// because the endXHandler event is fired last thing in the flow,
	// and if the program is exiting without any arrangement to wait
	// for running promises, this event might never get to be fired.
	startHandler
	endHandler

	// constructor (Group functions) values
	// start values...
	startGroupHandler
	startGroupChanHandler
	startGroupCtxHandler
	startGroupGoHandler
	startGroupGoErrHandler
	startGroupGoResHandler
	startGroupDelayHandler
	// end values...
	endGroupHandler
	endGroupChanHandler
	endGroupCtxHandler
	endGroupGoHandler
	endGroupGoErrHandler
	endGroupGoResHandler
	endGroupDelayHandler

	// follow (Promise methods) values
	// start values...
	startPromiseHandler
	startPromiseDelayHandler
	startPromiseCallbackHandler
	startPromiseFollowHandler
	startPromiseFollowCallbackHandler
	startPromiseThenHandler
	startPromiseCatchHandler
	startPromiseRecoverHandler
	startPromiseFinallyHandler
	// end values...
	endPromiseHandler
	endPromiseDelayHandler
	endPromiseCallbackHandler
	endPromiseFollowHandler
	endPromiseFollowCallbackHandler
	endPromiseThenHandler
	endPromiseCatchHandler
	endPromiseRecoverHandler
	endPromiseFinallyHandler
)
