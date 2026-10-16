// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import "github.com/huandu/go-clone"

const (
	insertMarkerInit injectionMarker = iota
	insertMarkerAfterInsertInto
	insertMarkerAfterCols
	insertMarkerAfterValues
	insertMarkerAfterSelect
	insertMarkerAfterReturning
)

const maxInsertPreallocationRows = 1024

func NewInsertBuilder() *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder()
}

func newInsertBuilder() *InsertBuilder {
	args := &Args{}
	return &InsertBuilder{
		verb:      "INSERT",
		args:      args,
		injection: newInjection(),
	}
}

func (ib *InsertBuilder) Clone() *InsertBuilder {
	return clone.Clone(ib).(*InsertBuilder)
}

type InsertBuilder struct {
	verb      string
	table     string
	cols      []string
	values    [][]string
	returning []string

	args *Args

	injection *injection
	marker    injectionMarker

	sbHolder string
}

var _ Builder = new(InsertBuilder)

func InsertInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertInto(table)
}

func (ib *InsertBuilder) InsertInto(table string) *InsertBuilder {
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

func InsertIgnoreInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertIgnoreInto(table)
}

func (ib *InsertBuilder) InsertIgnoreInto(table string) *InsertBuilder {
	ib.args.Flavor.PrepareInsertIgnore(table, ib)
	return ib
}

func ReplaceInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().ReplaceInto(table)
}

func (ib *InsertBuilder) ReplaceInto(table string) *InsertBuilder {
	ib.verb = "REPLACE"
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

func (ib *InsertBuilder) Cols(col ...string) *InsertBuilder {
	ib.cols = EscapeAll(col...)
	ib.marker = insertMarkerAfterCols
	return ib
}

func (isb *InsertBuilder) Select(col ...string) *SelectBuilder {
	sb := Select(col...)
	isb.sbHolder = isb.args.Add(sb)
	return sb
}

func (ib *InsertBuilder) Values(value ...any) *InsertBuilder {
	placeholders := make([]string, 0, len(value))

	for _, v := range value {
		placeholders = append(placeholders, ib.args.Add(v))
	}

	ib.values = append(ib.values, placeholders)
	ib.marker = insertMarkerAfterValues
	return ib
}

func (ib *InsertBuilder) Returning(col ...string) *InsertBuilder {
	ib.returning = col
	ib.marker = insertMarkerAfterReturning
	return ib
}

func (ib *InsertBuilder) NumValue() int {
	return len(ib.values)
}

func (ib *InsertBuilder) String() string {
	s, _ := ib.Build()
	return s
}

func (ib *InsertBuilder) Build() (sql string, args []any) {
	return ib.BuildWithFlavor(ib.args.Flavor)
}

func (ib *InsertBuilder) BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any) {
	buf := newStringBuilder()
	if len(ib.values) > 1 && len(ib.values) <= maxInsertPreallocationRows && (ib.sbHolder == "" || ib.args.Flavor == Oracle) {
		if size := ib.estimateSQLSize(); size <= maxCompileBufferCapacity {
			buf.Grow(size)
		}
	}

	ib.injection.WriteTo(buf, insertMarkerInit)

	if len(ib.values) > 1 && ib.args.Flavor == Oracle {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" ALL")

		for _, v := range ib.values {
			if len(ib.table) > 0 {
				buf.WriteString(" INTO ")
				buf.WriteString(ib.table)
			}

			ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)
			if len(ib.cols) > 0 {
				buf.WriteLeadingString("(")
				buf.WriteStrings(ib.cols, ", ")
				buf.WriteString(")")

				ib.injection.WriteTo(buf, insertMarkerAfterCols)
			}

			buf.WriteLeadingString("VALUES ")
			writeInsertRow(buf, v)
		}

		buf.WriteString(" SELECT 1 from DUAL")

		ib.injection.WriteTo(buf, insertMarkerAfterValues)

		return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
	}

	if len(ib.table) > 0 {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" INTO ")
		buf.WriteString(ib.table)
	}

	ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)

	if len(ib.cols) > 0 {
		buf.WriteLeadingString("(")
		buf.WriteStrings(ib.cols, ", ")
		buf.WriteString(")")

		ib.injection.WriteTo(buf, insertMarkerAfterCols)
	}

	if flavor == SQLServer {
		if len(ib.returning) > 0 {
			buf.WriteLeadingString("OUTPUT ")
			buf.WriteStringsPrefixed("INSERTED.", ib.returning, ", ")
		}

		ib.injection.WriteTo(buf, insertMarkerAfterReturning)
	}

	if ib.sbHolder != "" {
		buf.WriteString(" ")
		buf.WriteString(ib.sbHolder)

		ib.injection.WriteTo(buf, insertMarkerAfterSelect)
	}

	if ib.sbHolder == "" && len(ib.values) > 0 {
		buf.WriteLeadingString("VALUES ")
		for i, v := range ib.values {
			if i > 0 {
				buf.WriteString(", ")
			}

			writeInsertRow(buf, v)
		}
	}

	ib.injection.WriteTo(buf, insertMarkerAfterValues)

	if flavor == PostgreSQL || flavor == SQLite {
		if len(ib.returning) > 0 {
			buf.WriteLeadingString("RETURNING ")
			buf.WriteStrings(ib.returning, ", ")
		}

		ib.injection.WriteTo(buf, insertMarkerAfterReturning)
	}

	return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

func (ib *InsertBuilder) estimateSQLSize() int {
	header := len(" VALUES ")
	if len(ib.table) > 0 {
		header += len(" INTO ") + len(ib.table)
	}

	if len(ib.cols) > 0 {
		header += 3
		count := 0
		for _, col := range ib.cols {
			if col == "" {
				continue
			}

			if count > 0 {
				header += 2
			}

			header += len(col)
			count++
		}
	}

	size := len(ib.verb) + header + 2*(len(ib.values)-1)
	if ib.args.Flavor == Oracle {
		size = len(ib.verb) + len(" ALL") + len(" SELECT 1 from DUAL") + len(ib.values)*header
	}

	for _, row := range ib.values {
		size += 2 + 2*max(0, len(row)-1)
		for _, value := range row {
			size += len(value)
		}
	}

	return size
}

func writeInsertRow(buf *stringBuilder, values []string) {
	buf.WriteString("(")

	for i, value := range values {
		if i > 0 {
			buf.WriteString(", ")
		}

		buf.WriteString(value)
	}

	buf.WriteString(")")
}

func (ib *InsertBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ib.args.Flavor
	ib.args.Flavor = flavor
	return
}

func (ib *InsertBuilder) Flavor() Flavor {
	return ib.args.Flavor
}

func (ib *InsertBuilder) Var(arg any) string {
	return ib.args.Add(arg)
}

func (ib *InsertBuilder) SQL(sql string) *InsertBuilder {
	ib.injection.SQL(ib.marker, sql)
	return ib
}
