package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestCompilePoolRetainsResults(t *testing.T) {
	a := &Args{}
	a.Add(Build("($0, $1)", 42, sql.Named("n", 7)))
	a.Add(9)
	initial := make([]any, 1, 8)
	initial[0] = 11
	query, values := a.CompileWithFlavor("SELECT $0, $1", PostgreSQL, initial...)
	wantQuery := "SELECT ($2, @n), $3"
	wantValues := []any{11, 42, 9, sql.Named("n", 7)}
	if query != wantQuery || !reflect.DeepEqual(values, wantValues) {
		t.Fatalf("nested compile: %q %#v", query, values)
	}

	if &values[0] != &initial[0] {
		t.Fatal("initial argument backing array ownership changed")
	}

	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			other := &Args{}
			other.Add(worker)
			for range 200 {
				other.CompileWithFlavor(strings.Repeat("$0,", 100), SQLServer)
				if query != wantQuery || !reflect.DeepEqual(values, wantValues) {
					t.Error("retained output changed during concurrent pool reuse")
					return
				}
			}
		})
	}

	workers.Wait()
	runtime.GC()
	again, _ := a.CompileWithFlavor("SELECT $1", PostgreSQL)
	if again != "SELECT $1" || query != wantQuery || !reflect.DeepEqual(values, wantValues) {
		t.Fatal("pool cleanup or garbage collection changed retained output")
	}
}

func TestCompilePoolPanicCleanup(t *testing.T) {
	marker := new(int)
	a := &Args{}
	a.Add(condBuilder{Builder: func(ctx *argsCompileContext) {
		ctx.WriteString(strings.Repeat("x", 1<<16))
		ctx.Values = append(ctx.Values, marker)
		ctx.NamedArgs = append(ctx.NamedArgs, sql.Named("private", marker))
		panic(marker)
	}})

	func() {
		defer func() {
			if recovered := recover(); recovered != marker {
				t.Errorf("panic changed: %v", recovered)
			}
		}()

		a.CompileWithFlavor("$0", PostgreSQL)
		t.Error("expected panic")
	}()

	ctx := argsCompileContextPool.Get().(*argsCompileContext)
	defer releaseArgsCompileContext(ctx)

	if ctx.String() != "" || ctx.Cap() > maxCompileBufferCapacity || ctx.Values != nil || len(ctx.NamedArgs) != 0 || ctx.Flavor != invalidFlavor {
		t.Fatal("acquired context retained data from a previous compile")
	}

	b := &Args{}
	b.Add(123)
	query, values := b.CompileWithFlavor("SELECT $0", MySQL)
	if query != "SELECT ?" || !reflect.DeepEqual(values, []any{123}) {
		t.Fatalf("compile after panic: %q %#v", query, values)
	}
}

func TestCompilePoolConcurrentOwnership(t *testing.T) {
	var workers sync.WaitGroup
	for worker := range 16 {
		workers.Go(func() {
			a := &Args{}
			a.Add(worker)
			a.Add(sql.Named("worker", worker))
			wantValues := []any{worker, sql.Named("worker", worker)}
			for range 100 {
				query, values := a.CompileWithFlavor("SELECT $0, $1", PostgreSQL)
				runtime.Gosched()
				if query != "SELECT $1, @worker" || !reflect.DeepEqual(values, wantValues) {
					t.Errorf("worker %d: %q %#v", worker, query, values)
					return
				}
			}
		})
	}

	workers.Wait()
}

func FuzzCompilePoolLifetime(f *testing.F) {
	for _, format := range []string{"$0", "$0 $1 $2", "${name}", "$$", "$", "$?", "$999", "SELECT 1"} {
		f.Add(format, byte(0))
	}

	f.Fuzz(func(t *testing.T, format string, flavorIndex byte) {
		flavor := Flavor(int(flavorIndex)%int(Doris) + 1)
		a := &Args{}
		a.Add(17)
		a.Add(sql.Named("named", 23))
		a.Add(Named("name", 42))
		query, values := a.CompileWithFlavor(format, flavor)
		savedQuery := strings.Clone(query)
		savedValues := append([]any(nil), values...)
		other := &Args{}
		other.Add(99)
		for range 3 {
			other.CompileWithFlavor("different: $0, $0, $0", MySQL)
		}

		if query != savedQuery || !reflect.DeepEqual(values, savedValues) {
			t.Fatalf("retained result changed for %q (%v)", format, flavor)
		}
	})
}

func BenchmarkCompilePoolParallel(b *testing.B) {
	a := &Args{}
	a.Add(42)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			query, values := a.CompileWithFlavor("SELECT $0", PostgreSQL)
			if query != "SELECT $1" || len(values) != 1 || values[0] != 42 {
				panic(fmt.Sprintf("unexpected result: %q %#v", query, values))
			}
		}
	})
}
