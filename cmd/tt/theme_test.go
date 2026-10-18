package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

func TestNewTcellColor(t *testing.T) {
	valid := map[string]tcell.Color{
		"#000000": tcell.NewRGBColor(0, 0, 0),
		"#ff8000": tcell.NewRGBColor(255, 128, 0),
		"#FF8000": tcell.NewRGBColor(255, 128, 0),
		"#1a2B3c": tcell.NewRGBColor(0x1a, 0x2b, 0x3c),
	}

	for in, want := range valid {
		got, err := newTcellColor(in)
		if err != nil {
			t.Errorf("newTcellColor(%q) returned error: %v", in, err)
		} else if got != want {
			t.Errorf("newTcellColor(%q) = %v, want %v", in, got, want)
		}
	}

	for _, in := range []string{"", "#fff", "ff8000", "#ff80000", "ff8000#"} {
		if _, err := newTcellColor(in); err == nil {
			t.Errorf("newTcellColor(%q) should fail", in)
		}
	}
}

func TestParseConfig(t *testing.T) {
	if cfg := parseConfig(nil); cfg != nil {
		t.Errorf("parseConfig(nil) = %v, want nil", cfg)
	}

	got := parseConfig([]byte("fgcol: #ffffff\nnot a pair\nhicol:   #000000  \nurl: http://x\n"))
	want := map[string]string{
		"fgcol": "#ffffff",
		"hicol": "#000000",
		"url":   "http://x",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseConfig() = %v, want %v", got, want)
	}
}

func TestBuiltinThemesAreValid(t *testing.T) {
	res, _ := newTestResources(t)

	for _, name := range res.List("themes") {
		if _, err := loadTheme(res, name, true); err != nil {
			t.Errorf("builtin theme %q is invalid: %v", name, err)
		}
	}
}

func TestLoadTheme(t *testing.T) {
	res, dir := newTestResources(t)

	got, err := loadTheme(res, "default", false)
	if err != nil {
		t.Fatal(err)
	}

	want := theme{
		fgcol:  tcell.NewRGBColor(0x8c, 0x8c, 0x8c),
		bgcol:  tcell.ColorDefault,
		hicol:  tcell.NewRGBColor(0xff, 0xff, 0xff),
		hicol2: tcell.NewRGBColor(0xb4, 0x80, 0x1b),
		hicol3: tcell.NewRGBColor(0xe8, 0xa5, 0x22),
		errcol: tcell.NewRGBColor(0xa1, 0x07, 0x05),
	}
	if got != want {
		t.Errorf("transparent default theme = %v, want %v", got, want)
	}

	if got, err = loadTheme(res, "default", true); err != nil {
		t.Fatal(err)
	} else if got.bgcol != tcell.NewRGBColor(0x28, 0x28, 0x28) {
		t.Errorf("opaque default theme bg = %v, want #282828", got.bgcol)
	}

	if _, err := loadTheme(res, "does-not-exist", false); err == nil {
		t.Error("loading a missing theme should fail")
	}

	writeUserResource(t, dir, "themes", "partial", "fgcol: #ffffff\n")
	if _, err := loadTheme(res, "partial", false); err == nil || !strings.Contains(err.Error(), "hicol") {
		t.Errorf("loading a theme without hicol: err = %v, want an error mentioning hicol", err)
	}

	writeUserResource(t, dir, "themes", "nobg", "fgcol: #ffffff\nhicol: #ffffff\nhicol2: #ffffff\nhicol3: #ffffff\nerrcol: #ffffff\n")
	if _, err := loadTheme(res, "nobg", false); err != nil {
		t.Errorf("bgcol should not be required for transparent themes: %v", err)
	}
	if _, err := loadTheme(res, "nobg", true); err == nil || !strings.Contains(err.Error(), "bgcol") {
		t.Errorf("loading an opaque theme without bgcol: err = %v, want an error mentioning bgcol", err)
	}
}

func TestNewTcellColorRejectsInvalidDigits(t *testing.T) {
	for _, in := range []string{"#gggggg", "#12345z", "#+12345", "# 12345", "#0x1234"} {
		if c, err := newTcellColor(in); err == nil {
			t.Errorf("newTcellColor(%q) = %v, want an error", in, c)
		}
	}
}

// listing lists the given resources of resourceReader, whatever their type.
type listing struct {
	resourceReader
	names []string
}

func (l listing) List(string) []string { return l.names }

func TestThemeCycler(t *testing.T) {
	res, dir := newTestResources(t)
	valid := "fgcol: #ffffff\nhicol: #ffffff\nhicol2: #ffffff\nhicol3: #ffffff\nerrcol: #ffffff\n"
	writeUserResource(t, dir, "themes", "a", valid)
	writeUserResource(t, dir, "themes", "bad", "fgcol: #ffffff\n")
	writeUserResource(t, dir, "themes", "c", valid)
	names := listing{res, []string{"a", "bad", "c"}}

	assertNext := func(c *themeCycler, direction int, want string) {
		t.Helper()

		name, _, err := c.next(direction)
		if err != nil || name != want {
			t.Errorf("next(%d) = %q, %v, want %q", direction, name, err, want)
		}
	}

	c := newThemeCycler(names, "a", false)
	assertNext(c, 1, "c")
	assertNext(c, 1, "a")
	assertNext(c, -1, "c")
	assertNext(c, -1, "a")

	assertNext(newThemeCycler(names, "/path/to/c", false), 1, "a")
	assertNext(newThemeCycler(names, "unknown", false), 1, "a")
	assertNext(newThemeCycler(names, "unknown", false), -1, "c")

	if name, _, err := newThemeCycler(listing{res, nil}, "a", false).next(1); name != "" || err != nil {
		t.Errorf("without themes next() = %q, %v, want no theme and no error", name, err)
	}

	if _, _, err := newThemeCycler(listing{res, []string{"bad"}}, "a", false).next(1); err == nil {
		t.Error("next() should fail when no theme is valid")
	}

	name, th, err := newThemeCycler(listing{res, []string{"default"}}, "", true).next(1)
	if want, _ := loadTheme(res, "default", true); name != "default" || th != want || err != nil {
		t.Errorf("next() = %q, %v, %v, want the opaque default theme", name, th, err)
	}
}
