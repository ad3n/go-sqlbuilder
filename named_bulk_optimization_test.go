package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBuildNamedNestedCollisions(t *testing.T) {
	for range 100 {
		query, values := BuildNamed("${outer}, ${inner}", map[string]any{
			"outer": Named("inner", 7), "inner": 8,
		}).BuildWithFlavor(PostgreSQL)
		if query != "$1, $2" || len(values) != 2 || values[0] != values[1] || (values[0] != 7 && values[0] != 8) {
			t.Fatalf("nested collision semantics changed: %q %#v", query, values)
		}
	}

	query, values := BuildNamed("${a}, ${b}, ${raw}, ${nested}", map[string]any{
		"a": sql.Named("shared", 7), "b": sql.Named("shared", 7),
		"raw": Raw("DEFAULT"), "nested": Build("$?", nil),
	}).BuildWithFlavor(PostgreSQL, "initial")
	if query != "@shared, @shared, DEFAULT, $2" || !reflect.DeepEqual(values, []any{"initial", nil, sql.Named("shared", 7)}) {
		t.Fatalf("special named values changed: %q %#v", query, values)
	}
}

func TestInsertPlaceholderBatchCompatibility(t *testing.T) {
	for _, count := range []int{0, 1, 21, 22, 63, 64, 65, 100, 1000} {
		for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLServer, Oracle} {
			got := flavor.NewInsertBuilder().InsertInto("t").Cols("a", "b", "c")
			want := flavor.NewInsertBuilder().InsertInto("t").Cols("a", "b", "c")
			for i := range count {
				input := []any{i, Named(fmt.Sprintf("v%d", i), i+1), sql.Named("shared", 7)}
				if i%7 == 0 {
					input[1] = Raw("DEFAULT")
				}

				if i%11 == 0 {
					input[2] = Build("COALESCE($?, 0)", nil)
				}

				got.Values(input...)
				placeholders := make([]string, 0, len(input))
				for _, v := range input {
					placeholders = append(placeholders, want.args.Add(v))
				}

				want.values = append(want.values, placeholders)
				want.marker = insertMarkerAfterValues
				input[0] = -1
			}

			got.Values()
			want.values = append(want.values, []string{})
			want.marker = insertMarkerAfterValues
			if !reflect.DeepEqual(got.values, want.values) {
				t.Fatalf("%v/%d: placeholders changed", flavor, count)
			}

			wantSQL, wantArgs := want.BuildWithFlavor(flavor, "initial")
			for _, candidate := range []*InsertBuilder{got, got.Clone()} {
				query, values := candidate.BuildWithFlavor(flavor, "initial")
				if query != wantSQL || !reflect.DeepEqual(values, wantArgs) {
					t.Fatalf("%v/%d: build changed", flavor, count)
				}
			}

			query, values := got.BuildWithFlavor(flavor, "initial")
			retained := strings.Clone(query)
			savedValues := append([]any(nil), values...)
			got.Values(100, 200, 300).Build()
			if query != retained || !reflect.DeepEqual(values, savedValues) {
				t.Fatal("retained result changed")
			}
		}
	}
}

func TestWideInsertPlaceholderBatches(t *testing.T) {
	for _, columns := range []int{1, 2, 31, 32, 33, 64, 65, 1000} {
		ib := NewInsertBuilder()
		input := make([]any, columns)
		for i := range input {
			input[i] = i
		}

		for row := range 4 {
			ib.Values(input...)
			for column, placeholder := range ib.values[row] {
				if want := fmt.Sprintf("$%d", row*columns+column); placeholder != want {
					t.Fatalf("%d columns, row %d: got %q, want %q", columns, row, placeholder, want)
				}
			}
		}
	}
}

func BenchmarkBulkValues(b *testing.B) {
	for _, columns := range []int{1, 3, 20} {
		b.Run(fmt.Sprintf("Columns%d", columns), func(b *testing.B) {
			values := make([]any, columns)
			for i := range values {
				values[i] = i
			}

			b.ReportAllocs()
			for b.Loop() {
				ib := NewInsertBuilder()
				for range 1000 {
					ib.Values(values...)
				}

				allocationBuilder = ib
			}
		})
	}
}
