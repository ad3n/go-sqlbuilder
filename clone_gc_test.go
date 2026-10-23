package sqlbuilder

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/huandu/go-clone"
)

type (
	cloneGCBox struct {
		Pointer *int
		Label   string
	}

	cloneGCLateScalar struct{ pointer *int }
	cloneGCValue      struct {
		pointer *int
		values  []string
		dynamic any
		mapping map[string]*int
		array   [2]*int
		channel chan int
		opaque  *cloneGCOpaque
		scalar  cloneGCScalar
		custom  cloneGCCustom
	}

	cloneGCOpaque struct{ value int }
	cloneGCScalar struct{ pointer *int }
	cloneGCCustom struct{ Value int }

	cloneGCParallel struct {
		Values []any
		Panic  bool
	}
)

func init() {
	clone.MarkAsOpaquePointer(reflect.TypeFor[*cloneGCOpaque]())
	clone.MarkAsScalar(reflect.TypeFor[cloneGCScalar]())
	clone.SetCustomFunc(reflect.TypeFor[cloneGCCustom](), func(_ *clone.Allocator, old, result reflect.Value) {
		result.FieldByName("Value").SetInt(old.FieldByName("Value").Int() + 1)
	})
	clone.SetCustomFunc(reflect.TypeFor[cloneGCParallel](), func(allocator *clone.Allocator, old, result reflect.Value) {
		if old.FieldByName("Panic").Bool() {
			panic("clone failure")
		}

		input := old.FieldByName("Values")
		output := allocator.MakeSlice(input.Type(), input.Len(), input.Cap())
		var workers sync.WaitGroup
		for i := range input.Len() {
			workers.Go(func() { output.Index(i).Set(allocator.Clone(input.Index(i))) })
		}

		workers.Wait()
		result.FieldByName("Values").Set(output)
	})
}

func TestCloneDuringGC(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	TestInsertPlaceholderBlockContinuations(t)
}

func TestCloneConcurrentGCValues(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	var workers sync.WaitGroup
	for worker := range 4 {
		workers.Go(func() {
			for i := range 100 {
				number := worker*100 + i
				input := cloneGCValue{
					pointer: &number, values: []string{"owned", "strings"},
					dynamic: cloneGCBox{Pointer: &number, Label: "boxed"}, mapping: map[string]*int{"value": &number}, array: [2]*int{&number, &number},
					channel: make(chan int, 2), opaque: &cloneGCOpaque{value: number},
					scalar: cloneGCScalar{pointer: &number}, custom: cloneGCCustom{Value: number},
				}
				input.channel <- number
				ib := PostgreSQL.NewInsertBuilder().InsertInto("t").Cols("v").Values(input, time.Unix(1, 2))
				copied := ib.Clone()
				_, args := copied.Build()
				got := args[0].(cloneGCValue)
				box := got.dynamic.(cloneGCBox)
				if box.Pointer == input.dynamic.(cloneGCBox).Pointer || *box.Pointer != number || box.Label != "boxed" {
					t.Errorf("private interface copy changed")
					return
				}

				if got.pointer == input.pointer || *got.pointer != number || got.mapping["value"] == input.mapping["value"] || *got.mapping["value"] != number || got.array[0] == input.array[0] || *got.array[0] != number {
					t.Errorf("deep copy changed")
					return
				}

				if got.opaque != input.opaque || got.scalar.pointer != input.scalar.pointer || got.custom.Value != number+1 || got.channel == input.channel || cap(got.channel) != 2 || len(got.channel) != 0 || !reflect.DeepEqual(args[1], time.Unix(1, 2)) {
					t.Errorf("clone policies changed")
					return
				}

				got.values[0] = "changed"
				if input.values[0] != "owned" {
					t.Errorf("slice storage shared")
					return
				}
			}
		})
	}

	workers.Wait()
}

func TestCloneScopeReset(t *testing.T) {
	for _, size := range []int{32, maxCloneRootCapacity + 1} {
		scope := &cloneScope{roots: make([]unsafe.Pointer, 0, size)}
		scope.keep(reflect.ValueOf(new(int)))
		roots := scope.roots
		scope.reset()
		if len(scope.roots) != 0 || roots[0] != nil {
			t.Fatal("clone roots retained")
		}

		if size > maxCloneRootCapacity && scope.roots != nil {
			t.Fatal("oversized roots retained")
		}
	}
}

func TestCloneCustomParallelAndPanic(t *testing.T) {
	for _, fail := range []bool{true, false} {
		func() {
			defer func() {
				if got := recover(); (got != nil) != fail {
					t.Errorf("panic behavior changed: %v", got)
				}
			}()

			input := cloneGCParallel{Panic: fail, Values: []any{new(int), []string{"a"}, map[string]int{"b": 2}}}
			ib := NewInsertBuilder().Values(input)
			_, values := ib.Clone().Build()
			if !reflect.DeepEqual(values[0], input) {
				t.Fatal("custom parallel clone changed")
			}
		}()
	}
}

func TestCloneLateScalarPolicy(t *testing.T) {
	value := 42
	input := cloneGCLateScalar{pointer: &value}
	ib := NewInsertBuilder().Values(input)
	ib.Clone()
	clone.MarkAsScalar(reflect.TypeFor[cloneGCLateScalar]())
	_, args := ib.Clone().Build()
	if args[0].(cloneGCLateScalar).pointer != input.pointer {
		t.Fatal("updated scalar policy ignored")
	}
}
