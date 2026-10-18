package typing

import (
	"reflect"
	"testing"
)

func TestMostMistyped(t *testing.T) {
	t.Parallel()

	mistakes := []Mistake{
		{Word: "the", Typed: "teh"}, {Word: "zebra", Typed: "zerba"}, {Word: "because", Typed: "becuase"}, {Word: "the", Typed: "th_"},
		{Word: "a", Typed: "s"}, {Word: "because", Typed: "bec"}, {Word: "the", Typed: "hte"}, {Word: "a", Typed: "q"},
	}

	want := []WordCount{{"the", 3}, {"a", 2}, {"because", 2}, {"zebra", 1}}
	if got := MostMistyped(mistakes, 10); !reflect.DeepEqual(got, want) {
		t.Errorf("MostMistyped() = %v, want %v", got, want)
	}

	if got := MostMistyped(mistakes, 2); !reflect.DeepEqual(got, want[:2]) {
		t.Errorf("MostMistyped(2) = %v, want %v", got, want[:2])
	}

	if got := MostMistyped(nil, 10); len(got) != 0 {
		t.Errorf("MostMistyped(nil) = %v, want nothing", got)
	}
}
