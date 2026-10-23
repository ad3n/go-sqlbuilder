package sqlbuilder

import (
	"reflect"
	"testing"
)

var flattenedResult []any

func BenchmarkFlatten(b *testing.B) {
	for _, tc := range []struct {
		name  string
		input any
	}{
		{"Empty", []any{[]int{}, []int(nil)}},
		{"Flat", []any{1, "two", nil, 4, 5, 6, 7, 8}},
		{"Nested", []any{[]any{1, 2}, []any{3, nil}, []any{5, []any{6, 7, 8}}}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				flattenedResult = Flatten(tc.input)
			}
		})
	}
}

func BenchmarkTupleNames(b *testing.B) {
	for b.Loop() {
		scanSQL = TupleNames("id", "name", "", "active", "created_at")
	}
}

func TestFlattenOwnershipAndKinds(t *testing.T) {
	var pointer *int
	var channel chan int
	var function func()
	inputs := []struct {
		input any
		want  []any
	}{
		{nil, []any{nil}},
		{pointer, []any{pointer}},
		{channel, []any{channel}},
		{function, []any{function}},
		{[]any{}, nil},
		{[0]int{}, nil},
		{[]any{[]int(nil), [0]int{}}, nil},
		{[2]any{nil, [2]int{1, 2}}, []any{nil, 1, 2}},
		{[]any{pointer, []any{nil, 3}}, []any{pointer, nil, 3}},
	}
	for _, tc := range inputs {
		if got := Flatten(tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("Flatten(%#v) = %#v, want %#v", tc.input, got, tc.want)
		}
	}

	input := []any{1, 2, 3}
	got := Flatten(input)
	got[0] = 4
	input[1] = 5
	if got[1] != 2 || input[0] != 1 {
		t.Fatal("flattened output aliases input")
	}
}

func TestTupleNamesEmptyElements(t *testing.T) {
	for _, tc := range []struct {
		input []string
		want  string
	}{
		{nil, "()"},
		{[]string{"", ""}, "()"},
		{[]string{"", "a", "", "b", ""}, "(a, b)"},
		{[]string{"$id", "界"}, "($id, 界)"},
	} {
		if got := TupleNames(tc.input...); got != tc.want {
			t.Fatalf("TupleNames(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestEmptyModifiersAllocations(t *testing.T) {
	input := []any{[]int{}, []any(nil)}
	var boxed any = input
	allocations := testing.AllocsPerRun(100, func() {
		flattenedResult = Flatten(boxed)
		scanSQL = TupleNames("", "")
	})
	if allocations != 0 {
		t.Fatalf("empty modifiers allocated %v times", allocations)
	}
}
