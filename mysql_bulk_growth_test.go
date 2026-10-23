package sqlbuilder

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestBulkInsertGrowthCompatibility(t *testing.T) {
	for _, count := range []int{1024, 1025, 1135, 1136, 5000, 16384, 16385} {
		actual := MySQL.NewInsertBuilder().InsertInto("t").Cols("id", "name")
		reference := &Args{}
		rows := make([]string, 0, count)
		for i := range count {
			input := []any{i, "name"}
			switch i % 17 {
			case 0:
				input[1] = Raw("DEFAULT")
			case 1:
				input[1] = Build("$? + $?", i, i)
			case 2:
				input[1] = sql.Named("shared", true)
			case 3:
				input[1] = Named("duplicate", false)
			}

			actual.Values(input...)
			rows = append(rows, "("+reference.Add(input[0])+", "+reference.Add(input[1])+")")
		}

		template := "INSERT INTO t (id, name) VALUES " + strings.Join(rows, ", ")
		for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle} {
			wantSQL, wantArgs := reference.CompileWithFlavor(template, flavor, "initial")
			query, values := actual.BuildWithFlavor(flavor, "initial")
			if query != wantSQL || !reflect.DeepEqual(values, wantArgs) {
				t.Fatalf("%v/%d: SQL or argument order changed", flavor, count)
			}

			saved := strings.Clone(query)
			savedArgs := append([]any(nil), values...)
			actual.BuildWithFlavor(flavor, "other")
			if query != saved || !reflect.DeepEqual(values, savedArgs) {
				t.Fatal("retained output changed")
			}
		}

		if count == 5000 {
			clone := actual.Clone()
			clone.Values(-1, "clone")
			if actual.NumValue() != count || clone.NumValue() != count+1 {
				t.Fatal("clone shares row storage")
			}

			actual.Values(-2, "original")
			query, values := actual.Build()
			cloneQuery, cloneValues := clone.Build()
			if query != cloneQuery || values[len(values)-2] != "original" || cloneValues[len(cloneValues)-2] != "clone" {
				t.Fatal("clone or original argument storage changed")
			}
		}
	}
}

func TestLargeMySQLTemplateReservationLimit(t *testing.T) {
	for _, size := range []int{(1 << 20) - 1, 1 << 20, (1 << 20) + 1} {
		ib := MySQL.NewInsertBuilder().InsertInto("t").Cols("id")
		for range 1025 {
			ib.Values(Raw("DEFAULT"))
		}

		table := strings.Repeat("x", size-ib.estimateSQLSize()+1)
		ib.InsertInto(table)
		query, values := ib.Build()
		want := "INSERT INTO " + table + " (id) VALUES " + strings.Repeat("(DEFAULT), ", 1024) + "(DEFAULT)"
		if query != want || values != nil {
			t.Fatal("large template or nil argument result changed")
		}
	}
}
