package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var allocationBuilder Builder

func BenchmarkBuilderAllocation(b *testing.B) {
	for _, count := range []int{0, 3, 64, 1000} {
		values := make([]any, count)
		named := make(map[string]any, count)
		names := make([]string, count)
		for i := range values {
			values[i] = i
			name := fmt.Sprintf("v%d", i)
			named[name] = i
			names[i] = "${" + name + "}"
		}

		format := strings.Repeat("$? ", count)
		namedFormat := strings.Join(names, " ")
		b.Run(fmt.Sprintf("Build/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				allocationBuilder = Build(format, values...)
			}
		})
		b.Run(fmt.Sprintf("BuildNamed/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				allocationBuilder = BuildNamed(namedFormat, named)
			}
		})
		b.Run(fmt.Sprintf("Buildf/%d", count), func(b *testing.B) {
			format := strings.Repeat("%v ", count)
			b.ReportAllocs()
			for b.Loop() {
				allocationBuilder = Buildf(format, values...)
			}
		})
		b.Run(fmt.Sprintf("BuildAndCompile/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				scanSQL, scanArgs = Build(format, values...).BuildWithFlavor(PostgreSQL)
			}
		})
	}
}

func BenchmarkInterpolateTimes(b *testing.B) {
	for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLite, SQLServer, CQL, ClickHouse, Presto, Oracle, Informix, Doris} {
		values := make([]any, 100)
		for i := range values {
			values[i] = time.Date(2026, 10, 23, 12, 34, 56, 123456789, time.UTC)
		}

		query, args := Build(strings.Repeat("$? ", len(values)), values...).BuildWithFlavor(flavor)
		b.Run(flavor.String(), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				scanSQL, err = flavor.Interpolate(query, args)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestBuilderAllocationCompatibility(t *testing.T) {
	for flavor := MySQL; flavor <= Doris; flavor++ {
		input := []any{1, Named("x", 2), Named("x", 99), Raw("NULL"), Build("$?", 3), sql.Named("n", 4), sql.Named("n", 99)}
		builder := Build("$0 $1 $2 $3 $4 $5 $6 ${x}", input...)
		input[0] = -1
		initial := make([]any, 1, 32)
		initial[0] = "initial"
		gotSQL, got := builder.BuildWithFlavor(flavor, initial...)
		placeholders, _ := Build("$? $? $? $? $?", 1, 2, 2, 3, 2).BuildWithFlavor(flavor, "initial")
		parts := strings.Fields(placeholders)
		wantSQL := strings.Join([]string{parts[0], parts[1], parts[2], "NULL", parts[3], "@n", "@n", parts[4]}, " ")
		want := []any{"initial", 1, 2, 2, 3, 2, sql.Named("n", 4)}
		if gotSQL != wantSQL || !reflect.DeepEqual(got, want) {
			t.Fatalf("%v: %q %#v; want %q %#v", flavor, gotSQL, got, wantSQL, want)
		}

		snapshot := append([]any(nil), got...)
		builder.BuildWithFlavor(flavor, "other")
		if !reflect.DeepEqual(got, snapshot) {
			t.Fatal("retained result changed")
		}
	}

	for _, constructor := range []func() Builder{
		func() Builder { return Build("literal") },
		func() Builder { return Buildf("literal") },
		func() Builder { return BuildNamed("literal", nil) },
	} {
		query, args := constructor().Build()
		if query != "literal" || args != nil {
			t.Fatalf("empty arguments changed: %q %#v", query, args)
		}
	}
}

func TestTimeAppendCompatibility(t *testing.T) {
	layouts := map[Flavor]string{
		MySQL: "'2006-01-02 15:04:05.999999'", PostgreSQL: "'2006-01-02 15:04:05.999999 MST'",
		SQLite: "'2006-01-02 15:04:05.000'", SQLServer: "'2006-01-02 15:04:05.999999 Z07:00'",
		CQL: "'2006-01-02 15:04:05.999999Z0700'", ClickHouse: "'2006-01-02 15:04:05.999999'",
		Presto: "'2006-01-02 15:04:05.000'", Oracle: "2006-01-02 15:04:05.999999",
		Informix: "'2006-01-02 15:04:05.999999'", Doris: "'2006-01-02 15:04:05.999999'",
	}
	for flavor, layout := range layouts {
		for _, zone := range []*time.Location{time.UTC, time.FixedZone("custom", 7*3600+30*60), time.FixedZone("negative", -5*3600)} {
			for _, nanos := range []int{0, 123456499, 123456500, 999499999, 999500000, 999999999} {
				value := time.Date(2024, 12, 31, 23, 59, 59, nanos, zone)
				round := 500 * time.Nanosecond
				if flavor == SQLite || flavor == Presto {
					round = 500 * time.Microsecond
				}

				want := value.Add(round).Format(layout)
				if flavor == Oracle {
					want = "to_timestamp('" + want + "', 'YYYY-MM-DD HH24:MI:SS.FF')"
				}

				got, err := encodeValue([]byte("prefix:"), value, flavor)
				if err != nil || string(got) != "prefix:"+want {
					t.Fatalf("%v: %q, %v; want %q", flavor, got, err, want)
				}
			}
		}
	}
}

func TestBuilderLargeArgumentCompatibility(t *testing.T) {
	for _, count := range []int{63, 64, 65, 1000} {
		values := make([]any, count)
		named := make(map[string]any, count)
		names := make([]string, count)
		placeholders := make([]string, count)
		for i := range values {
			values[i] = i
			name := fmt.Sprintf("v%d", i)
			named[name] = i
			names[i] = "${" + name + "}"
			placeholders[i] = fmt.Sprintf("$%d", i+2)
		}

		builders := []Builder{
			Build(strings.Repeat("$? ", count), values...),
			Buildf(strings.Repeat("%v ", count), values...),
			BuildNamed(strings.Join(names, " ")+" ", named),
		}
		wantValues := append([]any{"initial"}, values...)
		wantSQL := strings.Join(placeholders, " ") + " "
		values[0] = -1
		named["v0"] = -1
		for _, builder := range builders {
			query, args := builder.BuildWithFlavor(PostgreSQL, "initial")
			if query != wantSQL || !reflect.DeepEqual(args, wantValues) {
				t.Fatalf("%d arguments: output or input ownership changed", count)
			}
		}
	}
}

func TestTimeEncodingAllocations(t *testing.T) {
	var value any = time.Date(2026, 10, 23, 12, 34, 56, 123456789, time.UTC)
	for flavor := MySQL; flavor <= Doris; flavor++ {
		buffer := make([]byte, 0, 256)
		allocations := testing.AllocsPerRun(100, func() {
			var err error
			buffer, err = encodeValue(buffer[:0], value, flavor)
			if err != nil {
				t.Fatal(err)
			}
		})
		if allocations != 0 {
			t.Fatalf("%v: time encoding allocated %v times", flavor, allocations)
		}
	}
}
