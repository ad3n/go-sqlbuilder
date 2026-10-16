package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBulkInsertSpecialArguments(t *testing.T) {
	for storedFlavor := MySQL; storedFlavor <= Doris; storedFlavor++ {
		for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLServer, Oracle} {
			ib := storedFlavor.NewInsertBuilder().SQL("/* start */").InsertInto("users").SQL("/* table */").Cols("id", "name", "active").SQL("/* cols */")
			ib.Values(Raw("DEFAULT"), Build("COALESCE($0, $1)", nil, "alice"), sql.Named("active", true))
			ib.Values(2, "bob", sql.Named("active", false)).SQL("/* values */").Returning("id").SQL("/* returning */")
			format := "/* start */ INSERT INTO users /* table */ (id, name, active) /* cols */"
			if storedFlavor == Oracle {
				format = "/* start */ INSERT ALL INTO users /* table */ (id, name, active) /* cols */ VALUES ($0, $1, $2) INTO users /* table */ (id, name, active) /* cols */ VALUES ($3, $4, $5) SELECT 1 from DUAL /* values */"
			}

			if storedFlavor != Oracle {
				if flavor == SQLServer {
					format += " OUTPUT INSERTED.id /* returning */"
				}

				format += " VALUES ($0, $1, $2), ($3, $4, $5) /* values */"
				if flavor == PostgreSQL || flavor == SQLite {
					format += " RETURNING id /* returning */"
				}
			}

			wantSQL, wantArgs := Build(format, Raw("DEFAULT"), Build("COALESCE($0, $1)", nil, "alice"), sql.Named("active", true), 2, "bob", sql.Named("active", false)).BuildWithFlavor(flavor, "initial")
			for _, builder := range []*InsertBuilder{ib, ib.Clone()} {
				gotSQL, gotArgs := builder.BuildWithFlavor(flavor, "initial")
				if gotSQL != wantSQL || !reflect.DeepEqual(gotArgs, wantArgs) {
					t.Fatalf("stored %v, output %v: got %q %#v; want %q %#v", storedFlavor, flavor, gotSQL, gotArgs, wantSQL, wantArgs)
				}
			}
		}
	}
}

func TestBulkInsertLargeResultLifetime(t *testing.T) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle} {
		ib := flavor.NewInsertBuilder().InsertInto("users").Cols("id", "name")
		for i := range 5000 {
			ib.Values(i, Raw("'"+strings.Repeat("x", 32)+"'"))
		}

		query, values := ib.Build()
		if len(query) <= maxCompileBufferCapacity || len(values) != 5000 {
			t.Fatalf("%v: unexpected large result size", flavor)
		}

		savedQuery := strings.Clone(query)
		savedValues := append([]any(nil), values...)
		ib.Values(-1, "changed")
		ib.Build()
		if query != savedQuery || !reflect.DeepEqual(values, savedValues) {
			t.Fatalf("%v: retained result changed", flavor)
		}
	}
}

func BenchmarkInsertLifecycle(b *testing.B) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle} {
		for _, rows := range []int{2, 10, 1000, 5000} {
			b.Run(fmt.Sprintf("%v/Rows%d", flavor, rows), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					ib := flavor.NewInsertBuilder().InsertInto("users").Cols("id", "name", "active")
					for i := range rows {
						ib.Values(i, "alice", true)
					}

					scanSQL, scanArgs = ib.Build()
				}
			})
		}
	}
}

func TestBulkInsertSelectPrecedence(t *testing.T) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLServer} {
		ib := flavor.NewInsertBuilder().InsertInto("users").Cols("id")
		ib.Values(1).Values(2)
		sb := ib.Select("id").From("source")
		sb.Where(sb.Equal("active", true))
		query, values := ib.Build()
		placeholder := "?"
		if flavor == PostgreSQL {
			placeholder = "$1"
		}

		if flavor == SQLServer {
			placeholder = "@p1"
		}

		if query != "INSERT INTO users (id) SELECT id FROM source WHERE active = "+placeholder || !reflect.DeepEqual(values, []any{true}) {
			t.Fatalf("%v: SELECT precedence changed: %q %#v", flavor, query, values)
		}
	}
}
