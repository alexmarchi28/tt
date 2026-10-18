package main

import (
	"io"
	"reflect"
	"testing"
	"time"
)

// parseFlags parses command line arguments known to be valid.
func parseFlags(args []string) options {
	opts, err := parseArgs(args, io.Discard, io.Discard)
	if err != nil {
		panic(err)
	}

	return opts
}

func TestParseFlagsDefaults(t *testing.T) {
	got := parseFlags(nil)

	if got.n != 50 || got.g != 1 || got.startParagraph != -1 || got.maxLineLen != 80 || got.timeout != -1 || got.focus != 40 {
		t.Errorf("unexpected numeric defaults %+v", got)
	}
	if got.themeName != "default" || len(got.given) != 0 {
		t.Errorf("theme = %q, flags given %v, want the default theme and no flag", got.themeName, got.given)
	}
	if len(got.files) != 0 {
		t.Errorf("files = %q, want none", got.files)
	}
}

func TestParseFlags(t *testing.T) {
	got := parseFlags([]string{"-t", "30", "-theme", "nord", "-highlight1", "-highlight2", "-notransparent", "-punct", "-numbers", "-n", "10", "-g", "3", "book.txt"})

	if got.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", got.timeout)
	}
	if got.themeName != "nord" || !got.given["theme"] || !got.given["t"] || got.given["w"] {
		t.Errorf("theme = %q, flags given %v, want nord given explicitly", got.themeName, got.given)
	}
	if !got.noHighlightNext || !got.noHighlightCurrent || !got.opaqueBg || !got.punct || !got.numbers {
		t.Errorf("flags not set: %+v", got)
	}
	if got.n != 10 || got.g != 3 || !reflect.DeepEqual(got.files, []string{"book.txt"}) {
		t.Errorf("unexpected options %+v", got)
	}

	if got := parseFlags([]string{"-theme", "default"}); !got.given["theme"] {
		t.Error("-theme default should count as an explicit theme")
	}
}

func TestParsePace(t *testing.T) {
	tests := []struct {
		arg    string
		pace   int
		target string
	}{
		{"", 0, ""},
		{"70", 70, ""},
		{"best", 0, "best"},
		{"avg", 0, "avg"},
	}

	for _, tc := range tests {
		args := []string{}
		if tc.arg != "" {
			args = []string{"-pace", tc.arg}
		}

		if got := parseFlags(args); got.pace != tc.pace || got.paceTarget != tc.target {
			t.Errorf("-pace %q: pace %d, target %q, want %d and %q", tc.arg, got.pace, got.paceTarget, tc.pace, tc.target)
		}
	}
}
