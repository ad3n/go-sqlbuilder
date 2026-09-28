// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/huandu/go-clone"
)

// Args stores arguments associated with a SQL.
type Args struct {
	// The default flavor used by `Args#Compile`
	Flavor Flavor

	indexBase    int
	argValues    *valueStore
	namedArgs    map[string]int
	sqlNamedArgs map[string]int
	onlyNamed    bool
}

func init() {
	predefinedArgs = make([]string, 0, maxPredefinedArgs)

	for i := range maxPredefinedArgs {
		predefinedArgs = append(predefinedArgs, fmt.Sprintf("$%v", i))
	}
}

const maxPredefinedArgs = 64

var predefinedArgs []string

func (args *Args) Add(arg any) string {
	idx := args.add(arg)

	if idx < maxPredefinedArgs {
		return predefinedArgs[idx]
	}

	var scratch [21]byte
	scratch[0] = '$'
	return string(strconv.AppendInt(scratch[:1], int64(idx), 10))
}

func (args *Args) add(arg any) int {
	idx := args.argValues.Len() + args.indexBase

	switch a := arg.(type) {
	case sql.NamedArg:
		if args.sqlNamedArgs == nil {
			args.sqlNamedArgs = map[string]int{}
		}

		if p, ok := args.sqlNamedArgs[a.Name]; ok {
			arg = args.argValues.Load(p)
			break
		}

		args.sqlNamedArgs[a.Name] = idx
	case namedArgs:
		if args.namedArgs == nil {
			args.namedArgs = map[string]int{}
		}

		if p, ok := args.namedArgs[a.name]; ok {
			arg = args.argValues.Load(p)
			break
		}

		idx = args.add(a.arg)
		args.namedArgs[a.name] = idx
		return idx
	}

	if args.argValues == nil {
		args.argValues = &valueStore{}
	}

	args.argValues.Add(arg)
	return idx
}

func (args *Args) Replace(placeholder string, arg any) {
	dollar := strings.IndexRune(placeholder, '$')

	if dollar != 0 {
		return
	}

	if i, err := strconv.Atoi(placeholder[1:]); err == nil {
		i -= args.indexBase
		args.argValues.Set(i, arg)
	}
}

func (args *Args) Compile(format string, initialValue ...any) (query string, values []any) {
	return args.CompileWithFlavor(format, args.Flavor, initialValue...)
}

func (args *Args) CompileWithFlavor(format string, flavor Flavor, initialValue ...any) (query string, values []any) {
	idx := strings.IndexRune(format, '$')
	if idx < 0 && len(args.sqlNamedArgs) == 0 {
		return format, initialValue
	}

	offset := 0
	ctx := argsCompileContextPool.Get().(*argsCompileContext)
	defer releaseArgsCompileContext(ctx)

	ctx.Flavor = flavor
	ctx.Values = initialValue

	if ctx.Flavor == invalidFlavor {
		ctx.Flavor = DefaultFlavor
	}

	for idx >= 0 && len(format) > 0 {
		if idx > 0 {
			ctx.WriteString(format[:idx])
		}

		format = format[idx+1:]

		if len(format) == 0 {
			ctx.WriteRune('$')
			break
		}

		if r := format[0]; r == '$' {
			ctx.WriteRune('$')
			format = format[1:]
		} else if r == '{' {
			format = args.compileNamed(ctx, format)
		} else if !args.onlyNamed && '0' <= r && r <= '9' {
			format, offset = args.compileDigits(ctx, format, offset)
		} else if !args.onlyNamed && r == '?' {
			format, offset = args.compileSuccessive(ctx, format[1:], offset)
		} else {
			ctx.WriteRune('$')
		}

		idx = strings.IndexRune(format, '$')
	}

	if len(format) > 0 {
		ctx.WriteString(format)
	}

	query = ctx.String()
	values = args.mergeSQLNamedArgs(ctx)
	return
}

func (args *Args) Value(arg string) any {
	_, values := args.Compile(arg)

	if len(values) == 0 {
		return nil
	}

	return values[0]
}

func (args *Args) compileNamed(ctx *argsCompileContext, format string) string {
	i := 1

	for ; i < len(format) && format[i] != '}'; i++ {
		// Nothing.
	}

	// Invalid $ format. Ignore it.
	if i == len(format) {
		return format
	}

	name := format[1:i]
	format = format[i+1:]

	if p, ok := args.namedArgs[name]; ok {
		format, _ = args.compileSuccessive(ctx, format, p-args.indexBase)
	}

	return format
}

func (args *Args) compileDigits(ctx *argsCompileContext, format string, offset int) (string, int) {
	i := 1

	for ; i < len(format) && '0' <= format[i] && format[i] <= '9'; i++ {
		// Nothing.
	}

	digits := format[:i]
	format = format[i:]

	if pointer, err := strconv.Atoi(digits); err == nil {
		return args.compileSuccessive(ctx, format, pointer-args.indexBase)
	}

	return format, offset
}

func (args *Args) compileSuccessive(ctx *argsCompileContext, format string, offset int) (string, int) {
	if offset < 0 || offset >= args.argValues.Len() {
		ctx.WriteString("/* INVALID ARG $")
		ctx.WriteString(strconv.Itoa(offset))
		ctx.WriteString(" */")
		return format, offset
	}

	arg := args.argValues.Load(offset)
	ctx.WriteValue(arg)

	return format, offset + 1
}

func (args *Args) mergeSQLNamedArgs(ctx *argsCompileContext) []any {
	if len(args.sqlNamedArgs) == 0 && len(ctx.NamedArgs) == 0 {
		return ctx.Values
	}

	values := ctx.Values
	existingNames := make(map[string]struct{}, len(ctx.NamedArgs))

	for _, arg := range ctx.NamedArgs {
		if _, ok := existingNames[arg.Name]; !ok {
			existingNames[arg.Name] = struct{}{}
			values = append(values, arg)
		}
	}

	ints := make([]int, 0, len(args.sqlNamedArgs))

	for n, p := range args.sqlNamedArgs {
		if _, ok := existingNames[n]; ok {
			continue
		}

		ints = append(ints, p)
	}

	slices.Sort(ints)

	for _, i := range ints {
		values = append(values, args.argValues.Load(i))
	}

	return values
}

func parseNamedArgs(initialValue []any) (values []any, namedValues []sql.NamedArg) {
	if len(initialValue) == 0 {
		values = initialValue
		return
	}

	size := len(initialValue)
	i := size

	for ; i > 0; i-- {
		switch initialValue[i-1].(type) {
		case sql.NamedArg:
			continue
		}

		break
	}

	if i == size {
		values = initialValue
		return
	}

	values = initialValue[:i]
	namedValues = make([]sql.NamedArg, 0, size-i)

	for ; i < size; i++ {
		namedValues = append(namedValues, initialValue[i].(sql.NamedArg))
	}

	return
}

type argsCompileContext struct {
	*stringBuilder

	Flavor    Flavor
	Values    []any
	NamedArgs []sql.NamedArg
}

var argsCompileContextPool = sync.Pool{
	New: func() any {
		return &argsCompileContext{stringBuilder: newStringBuilder()}
	},
}

func releaseArgsCompileContext(ctx *argsCompileContext) {
	ctx.Reset()
	ctx.Values = nil
	ctx.NamedArgs = nil
	ctx.Flavor = invalidFlavor
	argsCompileContextPool.Put(ctx)
}

func (ctx *argsCompileContext) WriteValue(arg any) {
	switch a := arg.(type) {
	case Builder:
		s, values := a.BuildWithFlavor(ctx.Flavor, ctx.Values...)
		ctx.WriteString(s)

		values, namedArgs := parseNamedArgs(values)
		ctx.Values = values
		ctx.NamedArgs = append(ctx.NamedArgs, namedArgs...)

	case sql.NamedArg:
		ctx.WriteRune('@')
		ctx.WriteString(a.Name)
		ctx.NamedArgs = append(ctx.NamedArgs, a)

	case rawArgs:
		ctx.WriteString(a.expr)

	case listArgs:
		if a.isTuple {
			ctx.WriteRune('(')
		}

		if len(a.args) > 0 {
			ctx.WriteValue(a.args[0])
		}

		for i := 1; i < len(a.args); i++ {
			ctx.WriteString(", ")
			ctx.WriteValue(a.args[i])
		}

		if a.isTuple {
			ctx.WriteRune(')')
		}

	case condBuilder:
		a.Builder(ctx)

	default:
		switch ctx.Flavor {
		case MySQL, SQLite, CQL, ClickHouse, Presto, Informix, Doris:
			ctx.WriteRune('?')
		case PostgreSQL:
			ctx.WriteString("$")
			ctx.WriteInt(len(ctx.Values) + 1)
		case SQLServer:
			ctx.WriteString("@p")
			ctx.WriteInt(len(ctx.Values) + 1)
		case Oracle:
			ctx.WriteString(":")
			ctx.WriteInt(len(ctx.Values) + 1)
		default:
			panic(fmt.Errorf("Args.CompileWithFlavor: invalid flavor %v (%v)", ctx.Flavor, int(ctx.Flavor)))
		}

		ctx.Values = append(ctx.Values, arg)
	}
}

func (ctx *argsCompileContext) WriteValues(values []any, sep string) {
	if len(values) == 0 {
		return
	}

	ctx.WriteValue(values[0])

	for _, v := range values[1:] {
		ctx.WriteString(sep)
		ctx.WriteValue(v)
	}
}

type valueStore struct {
	Values []any
}

func init() {

	t := reflect.TypeFor[valueStore]()
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		values := old.FieldByName("Values")
		newValues := allocator.Clone(values)
		new.FieldByName("Values").Set(newValues)
	})
}

func (as *valueStore) Len() int {
	if as == nil {
		return 0
	}

	return len(as.Values)
}

func (as *valueStore) Add(arg any) int {
	as.Values = append(as.Values, arg)
	return len(as.Values) - 1
}

func (as *valueStore) Set(index int, arg any) {
	if as == nil || index < 0 || index >= len(as.Values) {
		return
	}

	as.Values[index] = arg
}

func (as *valueStore) Load(index int) any {
	if as == nil || index < 0 || index >= len(as.Values) {
		return nil
	}

	return as.Values[index]
}
