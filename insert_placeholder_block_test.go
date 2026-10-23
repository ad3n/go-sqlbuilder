package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestInsertPlaceholderBlockContinuations(t *testing.T) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle, SQLServer} {
		for _, base := range []int{0, 63, 64, 255, 9999} {
			got := flavor.NewInsertBuilder().InsertInto("t")
			want := flavor.NewInsertBuilder().InsertInto("t")
			got.args.indexBase, want.args.indexBase = base, base
			for row := range 150 {
				if row%5 == 0 {
					for range row%19 + 1 {
						if got.Var("skip") != want.Var("skip") {
							t.Fatal("Var changed")
						}
					}
				}

				count := []int{0, 1, 3, 8, 9, 31, 64, 239, 240, 241, 255, 256, 257, 1000}[row%14]
				input := make([]any, count)
				for i := range input {
					switch i % 5 {
					case 0:
						input[i] = Named("outer", Named("inner", row))
					case 1:
						input[i] = sql.Named("shared", row)
					case 2:
						input[i] = Raw("DEFAULT")
					case 3:
						input[i] = Build("COALESCE($?, 0)", row)
					default:
						input[i] = i
					}
				}

				got.Values(input...)
				placeholders := make([]string, count)
				for i, v := range input {
					placeholders[i] = want.args.Add(v)
				}

				want.values = append(want.values, placeholders)
				want.marker = insertMarkerAfterValues
				if !reflect.DeepEqual(got.values, want.values) {
					t.Fatalf("%v/%d/%d: placeholders changed", flavor, base, row)
				}

				if row%7 == 0 {
					got = got.Clone()
				}
			}

			query, args := got.BuildWithFlavor(flavor, "initial")
			expected, expectedArgs := want.BuildWithFlavor(flavor, "initial")
			if query != expected || !reflect.DeepEqual(args, expectedArgs) {
				t.Fatalf("%v/%d: output changed", flavor, base)
			}

			copied := got.Clone()
			copied.Values("clone", "only")
			got.Values("original")
			if reflect.DeepEqual(got.values, copied.values) {
				t.Fatal("clone shares rows")
			}

			again, againArgs := want.BuildWithFlavor(flavor, "initial")
			if query != again || !reflect.DeepEqual(args, againArgs) {
				t.Fatal("retained output changed")
			}
		}
	}
}

func BenchmarkBulkValues5000(b *testing.B) {
	for _, columns := range []int{1, 3, 20} {
		b.Run(fmt.Sprintf("Columns%d", columns), func(b *testing.B) {
			values := make([]any, columns)
			for i := range values {
				values[i] = i
			}

			b.ReportAllocs()
			for b.Loop() {
				ib := NewInsertBuilder()
				for range 5000 {
					ib.Values(values...)
				}

				allocationBuilder = ib
			}
		})
	}
}

func BenchmarkBulkInsertClone(b *testing.B) {
	ib := PostgreSQL.NewInsertBuilder().InsertInto("users").Cols("id", "name", "active")
	for i := range 5000 {
		ib.Values(i, "alice", true)
	}
	b.ReportAllocs()
	for b.Loop() {
		allocationBuilder = ib.Clone()
	}
}

func TestInsertNilClone(t *testing.T) {
	var ib *InsertBuilder
	if ib.Clone() != nil {
		t.Fatal("nil clone changed")
	}
}

func TestInsertCloneRowsIndependent(t *testing.T) {
	ib := NewInsertBuilder().Values("a", "b").Values()
	copied := ib.Clone()
	if !reflect.DeepEqual(ib.values, copied.values) {
		t.Fatal("cloned rows changed")
	}

	copied.values[0][0] = "$99"
	if ib.values[0][0] != "$0" {
		t.Fatal("cloned row shares mutable storage")
	}

	if copied.values[1] == nil {
		t.Fatal("empty row became nil")
	}
}

func TestInsertCloneRowStorage(t *testing.T) {
	for _, rows := range [][][]string{
		nil,
		{},
		{nil, {}, nil},
		{{"$0", "$1"}, nil, {}, {"$2"}, {"$3", "$4", "$5"}},
	} {
		ib := NewInsertBuilder()
		ib.values = rows
		copied := ib.Clone()
		if !reflect.DeepEqual(rows, copied.values) {
			t.Fatalf("rows changed: %#v -> %#v", rows, copied.values)
		}

		for i, row := range copied.values {
			if len(row) == 0 {
				continue
			}

			original := rows[i][0]
			row[0] = "$changed"
			if rows[i][0] != original {
				t.Fatal("clone aliases source")
			}

			row[0] = original
			extended := append(row, "$appended")
			if extended[len(row)] != "$appended" || !reflect.DeepEqual(rows, copied.values) {
				t.Fatal("append overwrote another row")
			}
		}
	}
}
