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

// DelayCond describes when the wait duration provided to [Delay] or [Group.Delay]
// takes effect, in respect to the provided [Result] value to the same call.
type DelayCond int

func (m DelayCond) String() string {
	switch m {
	case OnAll:
		return "OnAll"
	case OnSuccess:
		return "OnSuccess"
	case OnError:
		return "OnError"
	case OnPanic:
		return "OnPanic"
	default:
		return "<unknown condition>"
	}
}

// any values other than the ones listed below will be ignored.
const (
	// OnAll means that [Delay] will wait on all [Result] values,
	// regardless of what [Result.State] returns.
	// This is the default behavior if no conditions are passed.
	OnAll DelayCond = iota

	// OnSuccess means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Success].
	OnSuccess

	// OnError means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Error].
	OnError

	// OnPanic means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Panic].
	OnPanic
)

type delayFlags struct {
	onSuccess bool
	onError   bool
	onPanic   bool
}

var delayAllFlags = delayFlags{
	onSuccess: true,
	onError:   true,
	onPanic:   true,
}

func getDelayFlags(conds []DelayCond) delayFlags {
	if len(conds) == 0 {
		return delayAllFlags
	}

	f := delayFlags{}
	for _, m := range conds {
		switch m {
		case OnAll:
			f.onSuccess = true
			f.onError = true
			f.onPanic = true
		case OnSuccess:
			f.onSuccess = true
		case OnError:
			f.onError = true
		case OnPanic:
			f.onPanic = true
		}
	}
	return f
}

// shouldDelayRes reports whether a delay should be applied for the given
// Result and delayFlags combination, based on the Result's state.
func shouldDelayRes[T any](res Result[T], flags delayFlags) bool {
	if res == nil {
		return flags.onSuccess
	}
	switch res.State() {
	case Panic:
		return flags.onPanic
	case Error:
		return flags.onError
	case Success:
		return flags.onSuccess
	default:
		return false
	}
}
