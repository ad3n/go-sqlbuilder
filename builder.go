// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
)

type Builder interface {
	Build() (sql string, args []any)
	BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any)
	Flavor() Flavor
}

type compiledBuilder struct {
	args   *Args
	format string
}

var _ Builder = new(compiledBuilder)

func (cb *compiledBuilder) Build() (sql string, args []any) {
	return cb.args.Compile(cb.format)
}

func (cb *compiledBuilder) BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any) {
	return cb.args.CompileWithFlavor(cb.format, flavor, initialArg...)
}

// Flavor returns flavor of builder
// Always returns DefaultFlavor
func (cb *compiledBuilder) Flavor() Flavor {
	return cb.args.Flavor
}

type flavoredBuilder struct {
	builder Builder
	flavor  Flavor
}

func (fb *flavoredBuilder) Build() (sql string, args []any) {
	return fb.builder.BuildWithFlavor(fb.flavor)
}

func (fb *flavoredBuilder) BuildWithFlavor(flavor Flavor, initialArg ...any) (sql string, args []any) {
	return fb.builder.BuildWithFlavor(flavor, initialArg...)
}

// Flavor returns flavor of builder
func (fb *flavoredBuilder) Flavor() Flavor {
	return fb.flavor
}

// WithFlavor creates a new Builder based on builder with a default flavor.
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

	for _, a := range arg {
		args.Add(a)
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

	for n, v := range named {
		args.Add(Named(n, v))
	}

	return &compiledBuilder{
		args:   args,
		format: format,
	}
}
