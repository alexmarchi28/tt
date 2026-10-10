package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseFlagsDefaults(t *testing.T) {
	got := parseFlags(nil)

	if got.n != 50 || got.g != 1 || got.startParagraph != -1 || got.maxLineLen != 80 || got.timeout != -1 {
		t.Errorf("unexpected numeric defaults %+v", got)
	}
	if got.themeName != "default" || got.themeExplicit {
		t.Errorf("theme = %q (explicit %t), want the implicit default theme", got.themeName, got.themeExplicit)
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
	if got.themeName != "nord" || !got.themeExplicit {
		t.Errorf("theme = %q (explicit %t), want nord given explicitly", got.themeName, got.themeExplicit)
	}
	if !got.noHighlightNext || !got.noHighlightCurrent || !got.opaqueBg || !got.punct || !got.numbers {
		t.Errorf("flags not set: %+v", got)
	}
	if got.n != 10 || got.g != 3 || !reflect.DeepEqual(got.files, []string{"book.txt"}) {
		t.Errorf("unexpected options %+v", got)
	}

	if got := parseFlags([]string{"-theme", "default"}); !got.themeExplicit {
		t.Error("-theme default should count as an explicit theme")
	}
}

func TestOptionsValidate(t *testing.T) {
	valid := [][]string{nil, {"-n", "1", "-g", "1", "-w", "1", "-t", "1", "-pace", "0"}, {"-t", "-1"}}
	for _, args := range valid {
		if err := parseFlags(args).validate(); err != nil {
			t.Errorf("%q: unexpected error %v", args, err)
		}
	}
}
