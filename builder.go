// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
)

type (
	Builder interface {
		Build() (sql string, args []any)
		BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any)
		Flavor() Flavor
	}

	compiledBuilder struct {
		args   *Args
		format string
	}

	flavoredBuilder struct {
		builder Builder
		flavor  Flavor
	}
)

var _ Builder = new(compiledBuilder)

func (cb *compiledBuilder) Build() (sql string, args []any) {
	return cb.args.Compile(cb.format)
}

func (cb *compiledBuilder) BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any) {
	return cb.args.CompileWithFlavor(cb.format, flavor, initialArg...)
}

func (cb *compiledBuilder) Flavor() Flavor {
	return cb.args.Flavor
}

func (fb *flavoredBuilder) Build() (sql string, args []any) {
	return fb.builder.BuildWithFlavor(fb.flavor)
}

func (fb *flavoredBuilder) BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any) {
	return fb.builder.BuildWithFlavor(flavor, initialArg...)
}

func (fb *flavoredBuilder) Flavor() Flavor {
	return fb.flavor
}

func WithFlavor(builder Builder, flavor Flavor) Builder {
	return &flavoredBuilder{
		builder: builder,
		flavor:  flavor,
	}
}

func Buildf(format string, arg ...any) Builder {
	args := &Args{
		Flavor: DefaultFlavor,
	}
	if len(arg) > 0 {
		args.argValues = &valueStore{Values: make([]any, 0, len(arg))}
	}

	vars := make([]any, 0, len(arg))

	for _, a := range arg {
		vars = append(vars, args.Add(a))
	}

	return &compiledBuilder{
		args:   args,
		format: fmt.Sprintf(Escape(format), vars...),
	}
}

func Build(format string, arg ...any) Builder {
	args := &Args{
		Flavor: DefaultFlavor,
	}

	if len(arg) > 0 {
		args.argValues = &valueStore{Values: make([]any, 0, len(arg))}
	}

	for _, a := range arg {
		args.add(a)
	}

	return &compiledBuilder{
		args:   args,
		format: format,
	}
}

func BuildNamed(format string, named map[string]any) Builder {
	args := &Args{
		Flavor:    DefaultFlavor,
		onlyNamed: true,
	}

	if len(named) > 0 {
		args.argValues = &valueStore{Values: make([]any, 0, len(named))}
		args.namedArgs = make(map[string]int, len(named))
	}

	for n, v := range named {
		if index, ok := args.namedArgs[n]; ok {
			args.argValues.Add(args.argValues.Load(index))
			continue
		}

		index := args.add(v)
		args.namedArgs[n] = index
	}

	return &compiledBuilder{
		args:   args,
		format: format,
	}
}
