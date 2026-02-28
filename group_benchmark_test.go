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

// workOutput implements [Result].
// it used in the tests below as an example for a custom [Result] type.
type workOutput struct {
	Greeting string
}

func (w workOutput) Val() workOutput { return w }
func (w workOutput) Err() error      { return nil }
func (w workOutput) State() State    { return Success }

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

	return UnwrapMultiResVal(pg.JoinRes()), nil
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

	return pg.JoinRes().Val(), nil
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

	return pg.JoinRes().Val(), nil
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

	return pg.JoinRes().Val(), nil
}

func Test_actualWork(t *testing.T) {
	t.Run("WG", func(t *testing.T) {
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
	})

	t.Run("PG_customWorkerResult_customGroupResult", func(t *testing.T) {
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
	})

	t.Run("PG_customWorkerResult_standardGroupResult", func(t *testing.T) {
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
	})

	t.Run("PG_standardWorkerResult_standardGroupResult", func(t *testing.T) {
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
	})

	t.Run("PG_standardWorkerResult_directGroupResult", func(t *testing.T) {
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
	})
}

func Benchmark_actualWork(b *testing.B) {
	b.Run("WG", func(b *testing.B) {
		ctx := context.Background()
		inputs := []workInput{{"ahmad"}, {"sameh"}}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, err := actualWorkWG(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PG_customWorkerResult_customGroupResult", func(b *testing.B) {
		ctx := context.Background()
		inputs := []workInput{{"ahmad"}, {"sameh"}}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, err := actualWorkPG_customWorkerResult_customGroupResult(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PG_customWorkerResult_standardGroupResult", func(b *testing.B) {
		ctx := context.Background()
		inputs := []workInput{{"ahmad"}, {"sameh"}}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, err := actualWorkPG_customWorkerResult_standardGroupResult(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PG_standardWorkerResult_standardGroupResult", func(b *testing.B) {
		ctx := context.Background()
		inputs := []workInput{{"ahmad"}, {"sameh"}}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, err := actualWorkPG_standardWorkerResult_standardGroupResult(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("PG_standardWorkerResult_callbackGroupResult", func(b *testing.B) {
		ctx := context.Background()
		inputs := []workInput{{"ahmad"}, {"sameh"}}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, err := actualWorkPG_standardWorkerResult_callbackGroupResult(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
