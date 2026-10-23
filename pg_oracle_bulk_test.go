package sqlbuilder

import (
	"database/sql"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLargeDialectInsertCompatibility(t *testing.T) {
	for _, stored := range []Flavor{PostgreSQL, Oracle} {
		for _, output := range []Flavor{MySQL, PostgreSQL, Oracle} {
			ib := stored.NewInsertBuilder().SQL("/* start */").InsertInto("t").SQL("/* table */").Cols("id", "name", "active").SQL("/* cols */")
			reference := &Args{}
			var format strings.Builder
			if stored == Oracle {
				format.WriteString("/* start */ INSERT ALL")
			}
			if stored != Oracle {
				format.WriteString("/* start */ INSERT INTO t /* table */ (id, name, active) /* cols */ VALUES ")
			}

			for row := range 1025 {
				values := []any{row, Build("COALESCE($?, 0)", nil), sql.Named("active", true)}
				if row%7 == 0 {
					values[1] = Raw("DEFAULT")
				}

				ib.Values(values...)
				if stored == Oracle {
					format.WriteString(" INTO t /* table */ (id, name, active) /* cols */ VALUES ")
				}
				if stored != Oracle && row > 0 {
					format.WriteString(", ")
				}

				format.WriteByte('(')
				for i, value := range values {
					if i > 0 {
						format.WriteString(", ")
					}

					format.WriteString(reference.Add(value))
				}

				format.WriteByte(')')
			}

			ib.SQL("/* values */").Returning("id").SQL("/* returning */")
			if stored == Oracle {
				format.WriteString(" SELECT 1 from DUAL")
			}

			format.WriteString(" /* values */")
			if stored != Oracle && output == PostgreSQL {
				format.WriteString(" RETURNING id /* returning */")
			}

			wantSQL, wantArgs := reference.CompileWithFlavor(format.String(), output, "initial")
			for _, builder := range []*InsertBuilder{ib, ib.Clone()} {
				query, values := builder.BuildWithFlavor(output, "initial")
				if query != wantSQL || !reflect.DeepEqual(values, wantArgs) {
					t.Fatalf("stored %v, output %v: result changed", stored, output)
				}

				saved := strings.Clone(query)
				savedArgs := append([]any(nil), values...)
				builder.Values(-1, "later", false).BuildWithFlavor(output)
				if query != saved || !reflect.DeepEqual(values, savedArgs) {
					t.Fatal("retained result changed")
				}
			}
		}
	}
}

func TestLargeDialectReservationFallback(t *testing.T) {
	for _, flavor := range []Flavor{PostgreSQL, Oracle} {
		sizes := []int{(1 << 20) - 16384, (1 << 20) + 1}
		if flavor == Oracle {
			sizes = []int{800, 1100}
		}

		for _, size := range sizes {
			table := strings.Repeat("t", size)
			ib := flavor.NewInsertBuilder().InsertInto(table).Cols("id")
			for range 1025 {
				ib.Values(Raw("DEFAULT"))
			}

			want := "INSERT INTO " + table + " (id) VALUES " + strings.Repeat("(DEFAULT), ", 1024) + "(DEFAULT)"
			if flavor == Oracle {
				want = "INSERT ALL" + strings.Repeat(" INTO "+table+" (id) VALUES (DEFAULT)", 1025) + " SELECT 1 from DUAL"
			}

			query, values := ib.Build()
			if query != want || values != nil {
				t.Fatalf("%v/%d: fallback output changed", flavor, size)
			}
		}
	}
}

func TestLargeCompileBufferEncoding(t *testing.T) {
	for _, size := range []int{maxCompileBufferCapacity - 1, maxCompileBufferCapacity, maxCompileBufferCapacity + 1, 128 << 10} {
		var buffer compileBuffer
		prefix := strings.Repeat("x", size)
		buffer.WriteString(prefix)
		var want strings.Builder
		want.WriteString(prefix)
		for _, r := range []rune{-1, 0, 'a', '界', 0xd800, math.MaxInt32} {
			buffer.WriteRune(r)
			want.WriteString(string(r))
		}

		for _, v := range []int{math.MinInt, -1, 0, 1, math.MaxInt} {
			buffer.WriteInt(v)
			want.WriteString(strconv.Itoa(v))
		}

		result := buffer.String()
		if result != want.String() {
			t.Fatalf("size %d: encoding changed", size)
		}

		buffer.Reset()
		buffer.WriteString(strings.Repeat("z", 128<<10))
		if result != want.String() {
			t.Fatal("retained large string changed")
		}
	}
}

func TestLargeRawCompilationStaysAllocationFree(t *testing.T) {
	args := &Args{}
	args.Add(Raw(""))
	format := strings.Repeat("$0", 40000)
	for _, flavor := range []Flavor{PostgreSQL, Oracle} {
		allocations := testing.AllocsPerRun(10, func() {
			scanSQL, scanArgs = args.CompileWithFlavor(format, flavor)
		})
		if allocations != 0 || scanSQL != "" || scanArgs != nil {
			t.Fatalf("%v: empty output changed or allocated: %v", flavor, allocations)
		}
	}
}

func BenchmarkLargeInsertBuild(b *testing.B) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle} {
		builder := flavor.NewInsertBuilder().InsertInto("users").Cols("id", "name", "active")
		for i := range 5000 {
			builder.Values(i, "alice", true)
		}

		b.Run(flavor.String(), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				scanSQL, scanArgs = builder.Build()
			}
		})
	}
}
