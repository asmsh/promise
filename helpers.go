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

// WaitAll waits all the provided promises to resolve then return true, or
// returns false if no promises are provided.
func WaitAll[T any](proms ...Promise[T]) (waited bool) {
	if len(proms) == 0 {
		return false
	}

	for _, p := range proms {
		p.Wait()
	}
	return true
}
