// Copyright 2026 Ahmad Sameh(asmsh)
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

import "testing"

func Test_filterResultFunc(t *testing.T) {
	type args struct {
		got    State
		target State
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// match Success
		{
			name: "Success matches Success",
			args: args{got: Success, target: Success},
			want: true,
		},
		{
			name: "Error does not match Success",
			args: args{got: Error, target: Success},
			want: false,
		},
		{
			name: "Panic does not match Success",
			args: args{got: Panic, target: Success},
			want: false,
		},
		// match Error
		{
			name: "Success does not match Error",
			args: args{got: Success, target: Error},
			want: false,
		},
		{
			name: "Error matches Error",
			args: args{got: Error, target: Error},
			want: true,
		},
		{
			name: "Panic does not match Error",
			args: args{got: Panic, target: Error},
			want: false,
		},
		// match Panic
		{
			name: "Success does not match Panic",
			args: args{got: Success, target: Panic},
			want: false,
		},
		{
			name: "Error matches Panic",
			args: args{got: Error, target: Panic},
			want: true,
		},
		{
			name: "Panic matches Panic",
			args: args{got: Panic, target: Panic},
			want: true,
		},
		// match unknown
		{
			name: "Success does not match unknown",
			args: args{got: Success, target: unknown},
			want: false,
		},
		// match unknown
		{
			name: "Success does not match unknown",
			args: args{got: Success, target: State(255)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterResultFunc(tt.args.got, tt.args.target); got != tt.want {
				t.Errorf("filterResultFunc() = %v, want %v", got, tt.want)
			}
		})
	}
}
