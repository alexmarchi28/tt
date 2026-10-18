package textgen

import (
	"reflect"
	"testing"
)

func TestGetParagraphs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one\ntwo"}},
		{"one\n\ntwo", []string{"one", "two"}},
		{"\n\none\r\n\r\n\r\n\ntwo\nthree\n\n", []string{"one", "two\nthree"}},
		{"one\n\n  \t\n\ntwo", []string{"one", "two"}},
		{"", nil},
		{" \n\n\t\n", nil},
	}

	for _, tc := range tests {
		if got := getParagraphs(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("getParagraphs(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
