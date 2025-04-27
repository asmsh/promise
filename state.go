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

import "fmt"

type State uint32

const (
	// the order and value here matter (increasing bitwise).

	Success State = 1 << iota
	Error
	Panic

	unknown State = 0
)

func (s State) String() string {
	switch s {
	case Success:
		return "Success"
	case Error:
		return "Error"
	case Panic:
		return "Panic"
	default:
		return fmt.Sprintf("<unknown>(%d)", s)
	}
}
