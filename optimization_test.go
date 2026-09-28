package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestCompileLiteralOwnership(t *testing.T) {
	a := &Args{}
	for _, initial := range [][]any{nil, {}, {123, sql.Named("name", "alice")}} {
		for _, format := range []string{"", "SELECT id FROM users"} {
			query, values := a.CompileWithFlavor(format, PostgreSQL, initial...)
			if query != format || !reflect.DeepEqual(values, initial) {
				t.Fatalf("literal changed: %q %#v", query, values)
			}

			if len(initial) > 0 && &values[0] != &initial[0] {
				t.Fatal("initial argument ownership changed")
			}
		}
	}

	a.Add(sql.Named("unused", 456))
	query, values := a.Compile("SELECT 1", 123)
	if query != "SELECT 1" || !reflect.DeepEqual(values, []any{123, sql.Named("unused", 456)}) {
		t.Fatalf("named arguments lost: %q %#v", query, values)
	}
}

func TestCompileLiteralAllocations(t *testing.T) {
	a := &Args{}
	allocations := testing.AllocsPerRun(100, func() {
		scanSQL, scanArgs = a.Compile("SELECT id FROM users")
	})
	if allocations != 0 {
		t.Fatalf("literal compile allocated %v times", allocations)
	}
}

func TestNumberedPlaceholderBoundaries(t *testing.T) {
	for _, flavor := range []Flavor{PostgreSQL, SQLServer, Oracle} {
		prefix := map[Flavor]string{PostgreSQL: "$", SQLServer: "@p", Oracle: ":"}[flavor]
		for _, count := range []int{0, 8, 9, 98, 99, 254, 255, 998, 999} {
			initial := make([]any, count, count+8)
			a := &Args{}
			a.Add(7)
			a.Add(Build("($0)", 8))
			query, values := a.CompileWithFlavor("$0, $1, $0", flavor, initial...)
			expected := fmt.Sprintf("%s%d, (%s%d), %s%d", prefix, count+1, prefix, count+2, prefix, count+3)
			if query != expected || len(values) != count+3 || !reflect.DeepEqual(values[count:], []any{7, 8, 7}) {
				t.Fatalf("%v count %d: %q %#v", flavor, count, query, values)
			}
		}
	}
}

func TestBuildResultLifetime(t *testing.T) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLServer, Oracle} {
		b := flavor.NewInsertBuilder().InsertInto("users").Cols("id", "name")
		b.Values(1, "alice").Values(2, "bob")
		query, values := b.Build()
		savedQuery := strings.Clone(query)
		savedValues := append([]any(nil), values...)
		b.args.Replace("$0", 3)
		b.Values(4, "carol")
		for range 10 {
			b.Build()
		}

		if query != savedQuery || !reflect.DeepEqual(values, savedValues) {
			t.Fatalf("%v: previous result changed", flavor)
		}
	}
}

func TestConcurrentIndependentBuilds(t *testing.T) {
	a := &Args{}
	a.Add(123)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			b := PostgreSQL.NewInsertBuilder().InsertInto("users").Cols("id")
			b.Values(123)
			for range 100 {
				query, values := a.CompileWithFlavor("SELECT $0", PostgreSQL)
				if query != "SELECT $1" || !reflect.DeepEqual(values, []any{123}) {
					t.Errorf("concurrent compile: %q %#v", query, values)
					return
				}

				query, values = b.Build()
				if query != "INSERT INTO users (id) VALUES ($1)" || !reflect.DeepEqual(values, []any{123}) {
					t.Errorf("independent build: %q %#v", query, values)
					return
				}
			}
		})
	}

	workers.Wait()
}

func TestWriteIntBoundaries(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, value := range []int{0, 1, -1, 9, 10, 99, 100, max, -max - 1} {
		b := newStringBuilder()
		b.WriteInt(value)
		if b.String() != strconv.Itoa(value) {
			t.Fatalf("unexpected decimal %q", b.String())
		}
	}
}

func FuzzWriteStringsPrefixed(f *testing.F) {
	f.Add("INSERTED.", "", "id", ", ")
	f.Add("", "", "id", ", ")
	f.Add("", "", "", "")
	f.Fuzz(func(t *testing.T, prefix, first, second, sep string) {
		values := []string{first, "", second}
		var expected []string
		for _, value := range values {
			if combined := prefix + value; combined != "" {
				expected = append(expected, combined)
			}
		}

		b := newStringBuilder()
		b.WriteStringsPrefixed(prefix, values, sep)
		if b.String() != strings.Join(expected, sep) {
			t.Fatalf("prefix output mismatch: %q", b.String())
		}
	})
}

func FuzzWriteInsertRow(f *testing.F) {
	f.Add("$0", "$1")
	f.Add("", "$0")
	f.Add("", "")
	f.Fuzz(func(t *testing.T, first, second string) {
		for _, values := range [][]string{nil, {}, {first}, {first, second}} {
			b := newStringBuilder()
			writeInsertRow(b, values)
			if b.String() != "("+strings.Join(values, ", ")+")" {
				t.Fatalf("row output mismatch: %q", b.String())
			}
		}
	})
}

func TestInsertRowsCompatibility(t *testing.T) {
	for flavor := MySQL; flavor <= Doris; flavor++ {
		for _, rows := range []int{1, 2, 1000} {
			b := flavor.NewInsertBuilder().InsertInto("users").Cols("id", "name")
			wantValues := []any{"initial"}
			var wantRows []string
			for row := range rows {
				b.Values(row, "alice")
				wantValues = append(wantValues, row, "alice")
				placeholders := []string{"?", "?"}
				for col := range placeholders {
					index := 2 + row*2 + col
					switch flavor {
					case PostgreSQL:
						placeholders[col] = fmt.Sprintf("$%d", index)
					case SQLServer:
						placeholders[col] = fmt.Sprintf("@p%d", index)
					case Oracle:
						placeholders[col] = fmt.Sprintf(":%d", index)
					}
				}

				wantRows = append(wantRows, "("+strings.Join(placeholders, ", ")+")")
			}

			wantQuery := "INSERT INTO users (id, name) VALUES " + strings.Join(wantRows, ", ")
			if flavor == Oracle && rows > 1 {
				wantQuery = "INSERT ALL"
				for _, row := range wantRows {
					wantQuery += " INTO users (id, name) VALUES " + row
				}

				wantQuery += " SELECT 1 from DUAL"
			}

			query, values := b.BuildWithFlavor(flavor, "initial")
			if query != wantQuery || !reflect.DeepEqual(values, wantValues) {
				t.Fatalf("%v, %d rows: SQL or arguments changed", flavor, rows)
			}
		}
	}
}
