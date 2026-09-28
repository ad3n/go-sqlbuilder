// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

const (
	lparen = "("
	rparen = ")"
	opOR   = " OR "
	opAND  = " AND "
	opNOT  = "NOT "
)

const minIndexBase = 256

// Cond provides several helper methods to build conditions.
type Cond struct {
	Args *Args
}

// NewCond returns a new Cond.
func NewCond() *Cond {
	return &Cond{
		Args: &Args{
			// Based on the discussion in #174, users may call this method to create
			// `Cond` for building various conditions, which is a misuse, but we
			// cannot completely prevent this error. To facilitate users in
			// identifying the issue when they make mistakes and to avoid
			// unexpected stackoverflows, the base index for `Args` is
			// deliberately set to a larger non-zero value here. This can
			// significantly reduce the likelihood of issues and allows for
			// timely error notification to users.
			indexBase: minIndexBase,
		},
	}
}

func (c *Cond) Equal(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" = ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) E(field string, value any) string {
	return c.Equal(field, value)
}

func (c *Cond) EQ(field string, value any) string {
	return c.Equal(field, value)
}

func (c *Cond) NotEqual(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" <> ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) NE(field string, value any) string {
	return c.NotEqual(field, value)
}

func (c *Cond) NEQ(field string, value any) string {
	return c.NotEqual(field, value)
}

func (c *Cond) GreaterThan(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" > ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) G(field string, value any) string {
	return c.GreaterThan(field, value)
}

func (c *Cond) GT(field string, value any) string {
	return c.GreaterThan(field, value)
}

func (c *Cond) GreaterEqualThan(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" >= ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) GE(field string, value any) string {
	return c.GreaterEqualThan(field, value)
}

func (c *Cond) GTE(field string, value any) string {
	return c.GreaterEqualThan(field, value)
}

func (c *Cond) LessThan(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" < ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) L(field string, value any) string {
	return c.LessThan(field, value)
}

func (c *Cond) LT(field string, value any) string {
	return c.LessThan(field, value)
}

func (c *Cond) LessEqualThan(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" <= ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) LE(field string, value any) string {
	return c.LessEqualThan(field, value)
}

func (c *Cond) LTE(field string, value any) string {
	return c.LessEqualThan(field, value)
}

func (c *Cond) In(field string, values ...any) string {
	if len(field) == 0 {
		return ""
	}

	if len(values) == 0 {
		return "0 = 1"
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" IN (")
			ctx.WriteValues(values, ", ")
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) NotIn(field string, values ...any) string {
	if len(field) == 0 {
		return ""
	}

	if len(values) == 0 {
		return "0 = 0"
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" NOT IN (")
			ctx.WriteValues(values, ", ")
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) Like(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" LIKE ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) ILike(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			switch ctx.Flavor {
			case PostgreSQL, ClickHouse:
				ctx.WriteString(field)
				ctx.WriteString(" ILIKE ")
				ctx.WriteValue(value)

			default:

				ctx.WriteString("LOWER(")
				ctx.WriteString(field)
				ctx.WriteString(") LIKE LOWER(")
				ctx.WriteValue(value)
				ctx.WriteString(")")
			}
		},
	})
}

func (c *Cond) NotLike(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" NOT LIKE ")
			ctx.WriteValue(value)
		},
	})
}

func (c *Cond) NotILike(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			switch ctx.Flavor {
			case PostgreSQL, ClickHouse:
				ctx.WriteString(field)
				ctx.WriteString(" NOT ILIKE ")
				ctx.WriteValue(value)

			default:

				ctx.WriteString("LOWER(")
				ctx.WriteString(field)
				ctx.WriteString(") NOT LIKE LOWER(")
				ctx.WriteValue(value)
				ctx.WriteString(")")
			}
		},
	})
}

// IsNull is used to construct the expression "field IS NULL".
func (c *Cond) IsNull(field string) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" IS NULL")
		},
	})
}

// IsNotNull is used to construct the expression "field IS NOT NULL".
func (c *Cond) IsNotNull(field string) string {
	if len(field) == 0 {
		return ""
	}
	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" IS NOT NULL")
		},
	})
}

func (c *Cond) Between(field string, lower, upper any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" BETWEEN ")
			ctx.WriteValue(lower)
			ctx.WriteString(" AND ")
			ctx.WriteValue(upper)
		},
	})
}

func (c *Cond) NotBetween(field string, lower, upper any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" NOT BETWEEN ")
			ctx.WriteValue(lower)
			ctx.WriteString(" AND ")
			ctx.WriteValue(upper)
		},
	})
}

// Or is used to construct the expression OR logic like "expr1 OR expr2 OR expr3".
func (c *Cond) Or(orExpr ...string) string {
	orExpr = filterEmptyStrings(orExpr)

	if len(orExpr) == 0 {
		return ""
	}

	exprByteLen := estimateStringsBytes(orExpr)
	if exprByteLen == 0 {
		return ""
	}

	buf := newStringBuilder()

	// Ensure that there is only 1 memory allocation.
	size := len(lparen) + len(rparen) + (len(orExpr)-1)*len(opOR) + exprByteLen
	buf.Grow(size)

	buf.WriteString(lparen)
	buf.WriteStrings(orExpr, opOR)
	buf.WriteString(rparen)
	return buf.String()
}

// And is used to construct the expression AND logic like "expr1 AND expr2 AND expr3".
func (c *Cond) And(andExpr ...string) string {
	andExpr = filterEmptyStrings(andExpr)

	if len(andExpr) == 0 {
		return ""
	}

	exprByteLen := estimateStringsBytes(andExpr)
	if exprByteLen == 0 {
		return ""
	}

	buf := newStringBuilder()

	// Ensure that there is only 1 memory allocation.
	size := len(lparen) + len(rparen) + (len(andExpr)-1)*len(opAND) + exprByteLen
	buf.Grow(size)

	buf.WriteString(lparen)
	buf.WriteStrings(andExpr, opAND)
	buf.WriteString(rparen)
	return buf.String()
}

// Not is used to construct the expression "NOT expr".
func (c *Cond) Not(notExpr string) string {
	if len(notExpr) == 0 {
		return ""
	}

	buf := newStringBuilder()

	// Ensure that there is only 1 memory allocation.
	size := len(opNOT) + len(notExpr)
	buf.Grow(size)

	buf.WriteString(opNOT)
	buf.WriteString(notExpr)
	return buf.String()
}

func (c *Cond) Exists(subquery any) string {
	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString("EXISTS (")
			ctx.WriteValue(subquery)
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) NotExists(subquery any) string {
	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString("NOT EXISTS (")
			ctx.WriteValue(subquery)
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) Any(field, op string, values ...any) string {
	if len(field) == 0 || len(op) == 0 {
		return ""
	}

	if len(values) == 0 {
		return "0 = 1"
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" ")
			ctx.WriteString(op)
			ctx.WriteString(" ANY (")
			ctx.WriteValues(values, ", ")
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) All(field, op string, values ...any) string {
	if len(field) == 0 || len(op) == 0 {
		return ""
	}

	if len(values) == 0 {
		return "0 = 1"
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" ")
			ctx.WriteString(op)
			ctx.WriteString(" ALL (")
			ctx.WriteValues(values, ", ")
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) Some(field, op string, values ...any) string {
	if len(field) == 0 || len(op) == 0 {
		return ""
	}

	if len(values) == 0 {
		return "0 = 1"
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			ctx.WriteString(field)
			ctx.WriteString(" ")
			ctx.WriteString(op)
			ctx.WriteString(" SOME (")
			ctx.WriteValues(values, ", ")
			ctx.WriteString(")")
		},
	})
}

func (c *Cond) IsDistinctFrom(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			switch ctx.Flavor {
			case PostgreSQL, SQLite, SQLServer:
				ctx.WriteString(field)
				ctx.WriteString(" IS DISTINCT FROM ")
				ctx.WriteValue(value)

			case MySQL:
				ctx.WriteString("NOT ")
				ctx.WriteString(field)
				ctx.WriteString(" <=> ")
				ctx.WriteValue(value)

			default:

				ctx.WriteString("CASE WHEN ")
				ctx.WriteString(field)
				ctx.WriteString(" IS NULL AND ")
				ctx.WriteValue(value)
				ctx.WriteString(" IS NULL THEN 0 WHEN ")
				ctx.WriteString(field)
				ctx.WriteString(" IS NOT NULL AND ")
				ctx.WriteValue(value)
				ctx.WriteString(" IS NOT NULL AND ")
				ctx.WriteString(field)
				ctx.WriteString(" = ")
				ctx.WriteValue(value)
				ctx.WriteString(" THEN 0 ELSE 1 END = 1")
			}
		},
	})
}

func (c *Cond) IsNotDistinctFrom(field string, value any) string {
	if len(field) == 0 {
		return ""
	}

	return c.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			switch ctx.Flavor {
			case PostgreSQL, SQLite, SQLServer:
				ctx.WriteString(field)
				ctx.WriteString(" IS NOT DISTINCT FROM ")
				ctx.WriteValue(value)

			case MySQL:
				ctx.WriteString(field)
				ctx.WriteString(" <=> ")
				ctx.WriteValue(value)

			default:

				ctx.WriteString("CASE WHEN ")
				ctx.WriteString(field)
				ctx.WriteString(" IS NULL AND ")
				ctx.WriteValue(value)
				ctx.WriteString(" IS NULL THEN 1 WHEN ")
				ctx.WriteString(field)
				ctx.WriteString(" IS NOT NULL AND ")
				ctx.WriteValue(value)
				ctx.WriteString(" IS NOT NULL AND ")
				ctx.WriteString(field)
				ctx.WriteString(" = ")
				ctx.WriteValue(value)
				ctx.WriteString(" THEN 1 ELSE 0 END = 1")
			}
		},
	})
}

func (c *Cond) Var(value any) string {
	return c.Args.Add(value)
}

type condBuilder struct {
	Builder func(ctx *argsCompileContext)
}

func estimateStringsBytes(strs []string) (n int) {
	for _, s := range strs {
		n += len(s)
	}

	return
}
