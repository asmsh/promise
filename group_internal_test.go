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
	"testing"
)

func Test_allOperation_InitState(t *testing.T) {
	type args struct {
		stateHist State
	}
	tests := []struct {
		name string
		args args
		want State
	}{
		{
			args: args{
				stateHist: 0,
			},
			want: unknown,
		},
		{
			args: args{
				stateHist: 0b1000,
			},
			want: State(8),
		},
		{
			args: args{
				stateHist: Success | Error | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Success | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Success | Error,
			},
			want: Error,
		},
		{
			args: args{
				stateHist: Error | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Panic | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Error | Error,
			},
			want: Error,
		},
		{
			args: args{
				stateHist: Success | Success,
			},
			want: unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			al := allOperation{}
			if got := al.initState(tt.args.stateHist); got != tt.want {
				t.Errorf("InitState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_allOperation_NextState(t *testing.T) {
	type args struct {
		currState State
		prevState State
	}
	tests := []struct {
		name string
		args args
		want State
	}{
		{
			args: args{
				currState: unknown,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Success,
				prevState: unknown,
			},
			want: Success,
		},

		{
			args: args{
				currState: Success,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Success,
				prevState: Error,
			},
			want: Error,
		},
		{
			args: args{
				currState: Success,
				prevState: Panic,
			},
			want: Panic,
		},

		{
			args: args{
				currState: Error,
				prevState: Success,
			},
			want: Error,
		},
		{
			args: args{
				currState: Error,
				prevState: Error,
			},
			want: Error,
		},
		{
			args: args{
				currState: Error,
				prevState: Panic,
			},
			want: Panic,
		},

		{
			args: args{
				currState: Panic,
				prevState: Success,
			},
			want: Panic,
		},
		{
			args: args{
				currState: Panic,
				prevState: Error,
			},
			want: Panic,
		},
		{
			args: args{
				currState: Panic,
				prevState: Panic,
			},
			want: Panic,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			al := allOperation{}
			if got := al.nextState(tt.args.currState, tt.args.prevState); got != tt.want {
				t.Errorf("NextState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_allOperation_IsTargetState(t *testing.T) {
	type args struct {
		currState State
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{
				currState: unknown,
			},
			want: false,
		},
		{
			args: args{
				currState: Panic,
			},
			want: true,
		},
		{
			args: args{
				currState: Error,
			},
			want: true,
		},
		{
			args: args{
				currState: Success,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			al := allOperation{}
			if got := al.isTargetState(tt.args.currState); got != tt.want {
				t.Errorf("IsTargetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anyOperation_InitState(t *testing.T) {
	type args struct {
		stateHist State
	}
	tests := []struct {
		name string
		args args
		want State
	}{
		{
			args: args{
				stateHist: 0,
			},
			want: unknown,
		},
		{
			args: args{
				stateHist: 0b1000,
			},
			want: State(8),
		},
		{
			args: args{
				stateHist: Success | Error | Panic,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: Success | Panic,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: Success | Error,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: Error | Panic,
			},
			want: unknown,
		},
		{
			args: args{
				stateHist: Panic | Panic,
			},
			want: unknown,
		},
		{
			args: args{
				stateHist: Success | Success,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: Error | Error,
			},
			want: unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			an := anyOperation{}
			if got := an.initState(tt.args.stateHist); got != tt.want {
				t.Errorf("InitState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anyOperation_NextState(t *testing.T) {
	type args struct {
		currState State
		prevState State
	}
	tests := []struct {
		name string
		args args
		want State
	}{
		{
			args: args{
				currState: unknown,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Success,
				prevState: unknown,
			},
			want: Success,
		},

		{
			args: args{
				currState: Success,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Success,
				prevState: Error,
			},
			want: Success,
		},
		{
			args: args{
				currState: Success,
				prevState: Panic,
			},
			want: Success,
		},

		{
			args: args{
				currState: Error,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Error,
				prevState: Error,
			},
			want: Error,
		},
		{
			args: args{
				currState: Error,
				prevState: Panic,
			},
			want: Panic,
		},

		{
			args: args{
				currState: Panic,
				prevState: Success,
			},
			want: Success,
		},
		{
			args: args{
				currState: Panic,
				prevState: Error,
			},
			want: Panic,
		},
		{
			args: args{
				currState: Panic,
				prevState: Panic,
			},
			want: Panic,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			an := anyOperation{}
			if got := an.nextState(tt.args.currState, tt.args.prevState); got != tt.want {
				t.Errorf("NextState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_anyOperation_IsTargetState(t *testing.T) {
	type args struct {
		currState State
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{
				currState: unknown,
			},
			want: false,
		},
		{
			args: args{
				currState: Panic,
			},
			want: false,
		},
		{
			args: args{
				currState: Error,
			},
			want: false,
		},
		{
			args: args{
				currState: Success,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			an := anyOperation{}
			if got := an.isTargetState(tt.args.currState); got != tt.want {
				t.Errorf("IsTargetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calcGroupResState(t *testing.T) {
	type args struct {
		stateHist State
	}
	tests := []struct {
		name string
		args args
		want State
	}{
		{
			args: args{
				stateHist: 0,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: 0b1000,
			},
			want: State(8),
		},
		{
			args: args{
				stateHist: Success | Error | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Success | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Success | Error,
			},
			want: Error,
		},
		{
			args: args{
				stateHist: Error | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Panic | Panic,
			},
			want: Panic,
		},
		{
			args: args{
				stateHist: Success | Success,
			},
			want: Success,
		},
		{
			args: args{
				stateHist: Error | Error,
			},
			want: Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcGroupResState(tt.args.stateHist); got != tt.want {
				t.Errorf("calcGroupResState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinOperationLogic_Distinct(t *testing.T) {
	if allOp == anyOp {
		t.Errorf("allOp should not be equal to anyOp")
	}
	if allOp == joinOp {
		t.Errorf("allOp should not be equal to joinOp")
	}
	if anyOp == joinOp {
		t.Errorf("anyOp should not be equal to joinOp")
	}
}
