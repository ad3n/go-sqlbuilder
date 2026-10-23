// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"reflect"
	"strings"
)

type (
	rawArgs struct {
		expr string
	}

	listArgs struct {
		args    []any
		isTuple bool
	}

	namedArgs struct {
		name string
		arg  any
	}
)

func Escape(ident string) string {
	return strings.ReplaceAll(ident, "$", "$$")
}

func EscapeAll(ident ...string) []string {
	escaped := make([]string, 0, len(ident))

	for _, i := range ident {
		escaped = append(escaped, Escape(i))
	}

	return escaped
}

func Flatten(slices any) (flattened []any) {
	return flatten(reflect.ValueOf(slices), nil)
}

func flatten(v reflect.Value, flattened []any) []any {
	k := v.Kind()

	for k == reflect.Interface {
		v = v.Elem()
		k = v.Kind()
	}

	if k != reflect.Slice && k != reflect.Array {
		if !v.IsValid() {
			return append(flattened, nil)
		}

		if !v.CanInterface() {
			return flattened
		}

		return append(flattened, v.Interface())
	}

	for i, l := 0, v.Len(); i < l; i++ {
		flattened = flatten(v.Index(i), flattened)
	}

	return flattened
}

func Raw(expr string) any {
	return rawArgs{expr}
}

func List(arg any) any {
	return listArgs{
		args: Flatten(arg),
	}
}

func Tuple(values ...any) any {
	return listArgs{
		args:    values,
		isTuple: true,
	}
}

func TupleNames(names ...string) string {
	size, count := 2, 0
	for _, name := range names {
		if name == "" {
			continue
		}

		size += len(name)
		count++
	}

	if count == 0 {
		return "()"
	}

	buf := newStringBuilder()

	buf.Grow(size + (count-1)*2)
	buf.WriteRune('(')
	buf.WriteStrings(names, ", ")
	buf.WriteRune(')')

	return buf.String()
}

func Named(name string, arg any) any {
	return namedArgs{
		name: name,
		arg:  arg,
	}
}
