package sqlbuilder

import (
	"database/sql"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestCompileBufferBoundaryOwnership(t *testing.T) {
	for _, size := range []int{0, 1, maxCompileBufferCapacity - 1, maxCompileBufferCapacity, maxCompileBufferCapacity + 1, 128 << 10} {
		var buf compileBuffer
		want := strings.Repeat("x", size)
		buf.WriteString(want)
		buf.WriteRune('界')
		buf.WriteInt(math.MinInt)
		want += "界" + strconv.Itoa(math.MinInt)

		query := buf.String()
		if query != want {
			t.Fatalf("size %d: output differs", size)
		}

		buf.Reset()
		if buf.String() != "" || buf.Cap() > maxCompileBufferCapacity || buf.large.Cap() != 0 {
			t.Fatalf("size %d: reset retained oversized storage", size)
		}

		buf.WriteString(strings.Repeat("z", 128<<10))
		if query != want {
			t.Fatalf("size %d: retained query changed", size)
		}
	}
}

func TestCompileContextReleaseClearsReferences(t *testing.T) {
	for _, count := range []int{1, maxCompileNamedArgsCapacity, maxCompileNamedArgsCapacity + 1} {
		ctx := new(argsCompileContext)
		ctx.Values = []any{new(int)}
		ctx.NamedArgs = make([]sql.NamedArg, count)
		for i := range ctx.NamedArgs {
			ctx.NamedArgs[i] = sql.Named("private", new(int))
		}

		retained := ctx.NamedArgs
		releaseArgsCompileContext(ctx)
		for _, arg := range retained {
			if arg.Name != "" || arg.Value != nil {
				t.Fatal("released context retained named argument references")
			}
		}
	}
}

func TestCompileBufferRuneCompatibility(t *testing.T) {
	for _, r := range []rune{-1, 0, 'a', '界', math.MaxInt32} {
		for _, size := range []int{0, maxCompileBufferCapacity} {
			var buf compileBuffer
			buf.WriteString(strings.Repeat("x", size))
			buf.WriteRune(r)
			if buf.String() != strings.Repeat("x", size)+string(r) {
				t.Fatalf("size %d, rune %d: encoding changed", size, r)
			}
		}
	}
}

func TestCompileBufferScratchAllocations(t *testing.T) {
	var buf compileBuffer
	buf.WriteString(strings.Repeat("x", 128))
	buf.Reset()
	allocations := testing.AllocsPerRun(100, func() {
		buf.Reset()
		buf.WriteString("SELECT ")
		buf.WriteRune('$')
		buf.WriteInt(123)
	})
	if allocations != 0 {
		t.Fatalf("warm scratch buffer allocated %v times", allocations)
	}

	if buf.String() != "SELECT $123" {
		t.Fatal("scratch output changed")
	}
}

func TestCompileBufferGrowthStaysBounded(t *testing.T) {
	var buf compileBuffer
	for range maxCompileBufferCapacity / 37 {
		buf.WriteString(strings.Repeat("x", 37))
		if buf.Cap() > maxCompileBufferCapacity {
			t.Fatal("incremental growth exceeded pool capacity limit")
		}
	}

	capacity := buf.Cap()
	buf.Reset()
	if buf.Cap() != capacity {
		t.Fatal("bounded scratch storage was discarded")
	}
}
