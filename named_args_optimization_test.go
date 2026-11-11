package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestParseNamedArgsReuse(t *testing.T) {
	marker := new(int)
	for _, input := range [][]any{nil, {}, {1}, {sql.Named("middle", 2), 3}, {1, sql.Named("last", marker)}, {sql.Named("only", marker)}} {
		ctx := &argsCompileContext{NamedArgs: make([]sql.NamedArg, 1, 8)}
		ctx.NamedArgs[0] = sql.Named("previous", 4)
		saved := append([]any(nil), input...)
		values, named := parseNamedArgs(input, ctx.NamedArgs)
		split := len(input)
		for split > 0 {
			if _, ok := input[split-1].(sql.NamedArg); !ok {
				break
			}
			split--
		}
		if !reflect.DeepEqual(values, input[:split]) || len(named) != 1+len(input)-split || named[0] != sql.Named("previous", 4) {
			t.Fatalf("unexpected split: %#v %#v", values, named)
		}
		for i := split; i < len(input); i++ {
			if named[1+i-split] != input[i].(sql.NamedArg) {
				t.Fatal("named argument order or identity changed")
			}
		}
		ctx.NamedArgs = named
		releaseArgsCompileContext(ctx)
		for i := range saved {
			if !reflect.DeepEqual(input[i], saved[i]) {
				t.Fatal("pool cleanup changed builder output")
			}
		}
		for _, arg := range named {
			if arg != (sql.NamedArg{}) {
				t.Fatal("pool retains named argument references")
			}
		}
	}
}

func TestParseNamedArgsZeroAllocations(t *testing.T) {
	input := []any{1, sql.Named("a", 2), sql.Named("b", 3)}
	destination := make([]sql.NamedArg, 0, 8)
	allocations := testing.AllocsPerRun(100, func() {
		values, named := parseNamedArgs(input, destination)
		if len(values) != 1 || len(named) != 2 {
			panic("invalid split")
		}
	})
	if allocations != 0 {
		t.Fatalf("named argument split allocated %v times", allocations)
	}
}

func BenchmarkParseNamedArgsReuse(b *testing.B) {
	input := []any{1, sql.Named("a", 2), sql.Named("b", 3)}
	destination := make([]sql.NamedArg, 0, 8)
	b.ReportAllocs()
	for b.Loop() {
		_, named := parseNamedArgs(input, destination)
		if len(named) != 2 {
			b.Fatal("invalid split")
		}
	}
}

func TestNamedArgsMergeBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 63, 64, 65, 256} {
		args := &Args{}
		want := make([]any, 1, count+1)
		want[0] = "initial"
		for i := range count {
			value := sql.Named(fmt.Sprintf("n%d", i), i)
			args.Add(value)
			want = append(want, value)
		}
		query, values := args.CompileWithFlavor("SELECT 1", PostgreSQL, "initial")
		if query != "SELECT 1" || !reflect.DeepEqual(values, want) {
			t.Fatalf("count %d: %q %#v", count, query, values)
		}
	}
}

func TestParseNamedArgsPoolBoundaries(t *testing.T) {
	for _, count := range []int{63, 64, 65, 256} {
		input := make([]any, count)
		for i := range input {
			input[i] = sql.Named(fmt.Sprintf("n%d", i), new(int))
		}

		ctx := &argsCompileContext{NamedArgs: make([]sql.NamedArg, 0, maxCompileNamedArgsCapacity)}
		ctx.Values, ctx.NamedArgs = parseNamedArgs(input, ctx.NamedArgs)
		if len(ctx.Values) != 0 || len(ctx.NamedArgs) != count {
			t.Fatal("incorrect argument split")
		}

		retained := ctx.NamedArgs
		releaseArgsCompileContext(ctx)
		if cap(ctx.NamedArgs) > maxCompileNamedArgsCapacity {
			t.Fatal("pool retained oversized storage")
		}

		for i, arg := range retained {
			if arg != (sql.NamedArg{}) || input[i].(sql.NamedArg).Value == nil {
				t.Fatal("pool cleanup retained references or modified source")
			}
		}
	}
}
