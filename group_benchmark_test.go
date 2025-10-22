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
	"reflect"
	"sync"
	"testing"
	"time"
)

type workInput struct {
	Name string
}

type workOutput struct {
	Greeting string
}

func (w workOutput) Val() workOutput {
	return w
}

func (w workOutput) Err() error {
	return nil
}

func (w workOutput) State() State {
	return Success
}

func actualWorkWG(
	ctx context.Context,
	inputs []workInput,
) ([]workOutput, error) {
	var wg sync.WaitGroup

	workInputsChan := make(chan workInput, len(inputs))
	workOutputsChan := make(chan workOutput, len(inputs))

	for range inputs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for w := range workInputsChan {
				time.Sleep(100 * time.Microsecond) // simulates some work

				workOutputsChan <- workOutput{Greeting: "Hello " + w.Name}
			}
		}()
	}

	for _, input := range inputs {
		workInputsChan <- input
	}

	close(workInputsChan)

	go func() {
		wg.Wait()
		close(workOutputsChan)
	}()

	outputs := make([]workOutput, 0, len(inputs))

	for output := range workOutputsChan {
		outputs = append(outputs, output)
	}

	return outputs, nil
}

func actualWorkPG_customWorkerResult_customGroupResult(
	ctx context.Context,
	inputs []workInput,
) ([]workOutput, error) {
	var pg Group[workOutput]

	for i := range inputs {
		pg.GoCtxRes(func(ctx context.Context) Result[workOutput] {
			time.Sleep(100 * time.Microsecond) // simulates some work

			return workOutput{Greeting: "Hello " + inputs[i].Name}
		})
	}

	return UnwrapMultiResVal(pg.AllWaitRes()), nil
}

func actualWorkPG_customWorkerResult_standardGroupResult(
	ctx context.Context,
	inputs []workInput,
) ([]GroupRes[workOutput], error) {
	var pg Group[workOutput]

	for i := range inputs {
		pg.GoCtxRes(func(ctx context.Context) Result[workOutput] {
			time.Sleep(100 * time.Microsecond) // simulates some work

			return workOutput{Greeting: "Hello " + inputs[i].Name}
		})
	}

	return pg.AllWaitRes().Val(), nil
}

func actualWorkPG_standardWorkerResult_standardGroupResult(
	ctx context.Context,
	inputs []workInput,
) ([]GroupRes[workOutput], error) {
	var pg Group[workOutput]

	for i := range inputs {
		pg.GoCtxRes(func(ctx context.Context) Result[workOutput] {
			time.Sleep(100 * time.Microsecond) // simulates some work

			return ValRes(workOutput{Greeting: "Hello " + inputs[i].Name})
		})
	}

	return pg.AllWaitRes().Val(), nil
}

func actualWorkPG_standardWorkerResult_callbackGroupResult(
	ctx context.Context,
	inputs []workInput,
) ([]GroupRes[workOutput], error) {
	var pg Group[workOutput]

	for i := range inputs {
		cb := CallbackFrom[workOutput, workOutput](func(ctx context.Context) workOutput {
			time.Sleep(100 * time.Microsecond) // simulates some work

			return workOutput{Greeting: "Hello " + inputs[i].Name}
		})
		pg.GoCallback(cb)
	}

	return pg.AllWaitRes().Val(), nil
}

func Benchmark_actualWorkWG(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	for b.Loop() {
		_, err := actualWorkWG(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}

	}
}

func Benchmark_actualWorkPG_customWorkerResult_customGroupResult(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	for b.Loop() {
		_, err := actualWorkPG_customWorkerResult_customGroupResult(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}

	}
}

func Benchmark_actualWorkPG_customWorkerResult_standardGroupResult(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	for b.Loop() {
		_, err := actualWorkPG_customWorkerResult_standardGroupResult(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}

	}
}

func Benchmark_actualWorkPG_standardWorkerResult_standardGroupResult(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	for b.Loop() {
		_, err := actualWorkPG_standardWorkerResult_standardGroupResult(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}

	}
}

func Benchmark_actualWorkPG_standardWorkerResult_callbackGroupResult(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	for b.Loop() {
		_, err := actualWorkPG_standardWorkerResult_callbackGroupResult(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}

	}
}

func Test_actualWorkWG(t *testing.T) {
	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	wantOutputs1 := []workOutput{{"Hello ahmad"}, {"Hello sameh"}}
	wantOutputs2 := []workOutput{{"Hello sameh"}, {"Hello ahmad"}}
	gotOutputs, err := actualWorkWG(ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOutputs, wantOutputs1) &&
		!reflect.DeepEqual(gotOutputs, wantOutputs2) {
		t.Errorf("actualWorkWG() = %v, want %v or %v", gotOutputs, wantOutputs1, wantOutputs2)
	}
}

func Test_actualWorkPG_customWorkerResult_customGroupResult(t *testing.T) {
	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	wantOutputs1 := []workOutput{
		{"Hello ahmad"},
		{"Hello sameh"},
	}
	wantOutputs2 := []workOutput{
		{"Hello sameh"},
		{"Hello ahmad"},
	}
	gotOutputs, err := actualWorkPG_customWorkerResult_customGroupResult(ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOutputs, wantOutputs1) &&
		!reflect.DeepEqual(gotOutputs, wantOutputs2) {
		t.Errorf("actualWorkPG_customWorkerResult_customGroupResult() = %v, want %v or %v", gotOutputs, wantOutputs1, wantOutputs2)
	}
}

func Test_actualWorkPG_customWorkerResult_standardGroupResult(t *testing.T) {
	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	wantOutputs1 := []GroupRes[workOutput]{
		{workOutput{"Hello ahmad"}},
		{workOutput{"Hello sameh"}},
	}
	wantOutputs2 := []GroupRes[workOutput]{
		{workOutput{"Hello sameh"}},
		{workOutput{"Hello ahmad"}},
	}
	gotOutputs, err := actualWorkPG_customWorkerResult_standardGroupResult(ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOutputs, wantOutputs1) &&
		!reflect.DeepEqual(gotOutputs, wantOutputs2) {
		t.Errorf("actualWorkPG_customWorkerResult_standardGroupResult() = %v, want %v or %v", gotOutputs, wantOutputs1, wantOutputs2)
	}
}

func Test_actualWorkPG_standardWorkerResult_standardGroupResult(t *testing.T) {
	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	wantOutputs1 := []GroupRes[workOutput]{
		{valResult[workOutput]{workOutput{Greeting: "Hello ahmad"}}},
		{valResult[workOutput]{workOutput{Greeting: "Hello sameh"}}},
	}
	wantOutputs2 := []GroupRes[workOutput]{
		{valResult[workOutput]{workOutput{Greeting: "Hello sameh"}}},
		{valResult[workOutput]{workOutput{Greeting: "Hello ahmad"}}},
	}
	gotOutputs, err := actualWorkPG_standardWorkerResult_standardGroupResult(ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOutputs, wantOutputs1) &&
		!reflect.DeepEqual(gotOutputs, wantOutputs2) {
		t.Errorf("actualWorkPG_standardWorkerResult_standardGroupResult() = %#v, want %#v or %#v", gotOutputs, wantOutputs1, wantOutputs2)
	}
}

func Test_actualWorkPG_standardWorkerResult_directGroupResult(t *testing.T) {
	ctx := context.Background()
	inputs := []workInput{{"ahmad"}, {"sameh"}}
	wantOutputs1 := []GroupRes[workOutput]{
		{valResult[workOutput]{workOutput{Greeting: "Hello ahmad"}}},
		{valResult[workOutput]{workOutput{Greeting: "Hello sameh"}}},
	}
	wantOutputs2 := []GroupRes[workOutput]{
		{valResult[workOutput]{workOutput{Greeting: "Hello sameh"}}},
		{valResult[workOutput]{workOutput{Greeting: "Hello ahmad"}}},
	}
	gotOutputs, err := actualWorkPG_standardWorkerResult_callbackGroupResult(ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOutputs, wantOutputs1) &&
		!reflect.DeepEqual(gotOutputs, wantOutputs2) {
		t.Errorf("actualWorkPG_standardWorkerResult_callbackGroupResult() = %#v, want %#v or %#v", gotOutputs, wantOutputs1, wantOutputs2)
	}
}
