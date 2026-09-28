package sqlbuilder

import (
	"fmt"
	"testing"
)

var scanSQL string
var scanArgs []any

func BenchmarkScan(b *testing.B) {
	b.Run("CompileLiteral", func(b *testing.B) {
		a := &Args{}
		b.ReportAllocs()
		for b.Loop() {
			scanSQL, scanArgs = a.Compile("SELECT id, name FROM users")
		}
	})
	for _, flavor := range []Flavor{MySQL, PostgreSQL, Oracle} {
		b.Run(fmt.Sprint(flavor), func(b *testing.B) {
			b.Run("SelectBuild", func(b *testing.B) {
				s := flavor.NewSelectBuilder()
				s.Select("id", "name").From("users")
				s.Where(s.Equal("active", true))
				s.Limit(10)
				b.ReportAllocs()
				for b.Loop() {
					scanSQL, scanArgs = s.Build()
				}
			})
			for _, rows := range []int{1, 100, 1000} {
				b.Run(fmt.Sprintf("Insert%d", rows), func(b *testing.B) {
					s := flavor.NewInsertBuilder()
					s.InsertInto("users").Cols("id", "name", "active")
					for i := range rows {
						s.Values(i, "alice", true)
					}

					b.ReportAllocs()
					for b.Loop() {
						scanSQL, scanArgs = s.Build()
					}
				})
			}
		})
	}

	b.Run("Prefix", func(b *testing.B) {
		cols := []string{"id", "name", "active", "created_at"}
		b.ReportAllocs()
		for b.Loop() {
			s := newStringBuilder()
			s.WriteStringsPrefixed("INSERTED.", cols, ", ")
			scanSQL = s.String()
		}
	})
}
