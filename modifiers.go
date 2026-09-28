// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"reflect"
	"strings"
)

func Escape(ident string) string {
	return strings.ReplaceAll(ident, "$", "$$")
}

// EscapeAll replaces `$` with `$$` in all strings of ident.
func EscapeAll(ident ...string) []string {
	escaped := make([]string, 0, len(ident))

	for _, i := range ident {
		escaped = append(escaped, Escape(i))
	}

	return escaped
}

func Flatten(slices any) (flattened []any) {
	v := reflect.ValueOf(slices)
	slices, flattened = flatten(v)

	if slices != nil {
		return []any{slices}
	}

	return flattened
}

func flatten(v reflect.Value) (elem any, flattened []any) {
	k := v.Kind()

	for k == reflect.Interface {
		v = v.Elem()
		k = v.Kind()
	}

	if k != reflect.Slice && k != reflect.Array {
		if !v.IsValid() || !v.CanInterface() {
			return
		}

		elem = v.Interface()
		return elem, nil
	}

	for i, l := 0, v.Len(); i < l; i++ {
		e, f := flatten(v.Index(i))

		if e == nil {
			flattened = append(flattened, f...)
		} else {
			flattened = append(flattened, e)
		}
	}

	return
}

type rawArgs struct {
	expr string
}

func Raw(expr string) any {
	return rawArgs{expr}
}

type listArgs struct {
	args    []any
	isTuple bool
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

// TupleNames joins names with tuple format.
// The names is not escaped. Use `EscapeAll` to escape them if necessary.
func TupleNames(names ...string) string {
	buf := newStringBuilder()
	buf.WriteRune('(')
	buf.WriteStrings(names, ", ")
	buf.WriteRune(')')

	return buf.String()
}

type namedArgs struct {
	name string
	arg  any
}

func Named(name string, arg any) any {
	return namedArgs{
		name: name,
		arg:  arg,
	}
}
