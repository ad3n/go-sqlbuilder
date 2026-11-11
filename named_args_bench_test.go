package sqlbuilder

import (
	"database/sql"
	"fmt"
	"testing"
)

func BenchmarkNamedCompile(b *testing.B) {
	for _, count := range []int{1, 8, 64, 65, 256} {
		for _, nested := range []bool{false, true} {
			b.Run(fmt.Sprintf("Count%d/Nested%t", count, nested), func(b *testing.B) {
				args := &Args{}
				format := ""
				for i := range count {
					value := any(sql.Named(fmt.Sprintf("arg%d", i), i))
					format += args.Add(value) + ","
				}

				if nested {
					inner := Build(format, args.argValues.Values...)
					args = &Args{}
					format = args.Add(inner)
				}

				b.ReportAllocs()
				for b.Loop() {
					scanSQL, scanArgs = args.CompileWithFlavor(format, PostgreSQL)
				}
			})
		}
	}
}
