package main

import (
	"strings"
	"testing"
	"time"
)

// resolveFlags resolves command line arguments known to be valid, stdin
// being a terminal.
func resolveFlags(args []string) settings {
	s, err := resolve(parseFlags(args), true)
	if err != nil {
		panic(err)
	}

	return s
}

// The mode of the tests is resolved once, explicit modes taking precedence
// over the text piped to stdin.
func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		stdinIsTerminal bool
		kind            testKind
		path            string
	}{
		{"default", nil, true, testKind{Mode: "words", Source: "1000en", Words: 50}, "1000en"},
		{"word list", []string{"-words", "dir/200en"}, true, testKind{Mode: "words", Source: "200en", Words: 50}, "dir/200en"},
		{"quotes", []string{"-quotes", "en"}, true, testKind{Mode: "quotes", Source: "en"}, "en"},
		{"mistakes", []string{"-mistakes"}, true, testKind{Mode: "mistakes", Words: 50}, ""},
		{"adaptive", []string{"-adaptive"}, true, testKind{Mode: "adaptive", Source: "1000en", Words: 50, Focus: 40}, "1000en"},
		{"adaptive word list", []string{"-adaptive", "-words", "200en"}, true, testKind{Mode: "adaptive", Source: "200en", Words: 50, Focus: 40}, "200en"},
		{"file", []string{"dir/book.txt"}, true, testKind{Mode: "file", Source: "book.txt"}, "dir/book.txt"},
		{"stdin", nil, false, testKind{Mode: "text"}, ""},
		{"file over stdin", []string{"book.txt"}, false, testKind{Mode: "file", Source: "book.txt"}, "book.txt"},
		{"quotes over stdin", []string{"-quotes", "en"}, false, testKind{Mode: "quotes", Source: "en"}, "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := resolve(parseFlags(tc.args), tc.stdinIsTerminal)
			if err != nil {
				t.Fatal(err)
			}
			if s.source.kind != tc.kind || s.source.path != tc.path {
				t.Errorf("source = %+v, want kind %+v and path %q", s.source, tc.kind, tc.path)
			}
			if s.command != nil {
				t.Errorf("command -%s, want typing tests", s.command.flag)
			}
		})
	}
}

// Options which would be silently ignored are rejected.
func TestResolveRejectsConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-adaptive", "-quotes", "en"}, "-adaptive and -quotes can't be used together"},
		{[]string{"-adaptive", "-mistakes"}, "-adaptive and -mistakes can't be used together"},
		{[]string{"-words", "200en", "-quotes", "en"}, "-words and -quotes can't be used together"},
		{[]string{"-words", "200en", "-mistakes"}, "-words and -mistakes can't be used together"},
		{[]string{"-quotes", "en", "-mistakes"}, "-quotes and -mistakes can't be used together"},
		{[]string{"-mistakes", "book.txt"}, "-mistakes and FILE can't be used together"},
		{[]string{"-adaptive", "book.txt"}, "-adaptive and FILE can't be used together"},
		{[]string{"a.txt", "b.txt"}, "only one FILE can be typed at a time"},
		{[]string{"-focus", "70"}, "-focus only applies to -adaptive"},
		{[]string{"-stats", "-keys"}, "-stats and -keys can't be used together"},
		{[]string{"-list", "themes", "-export", "results"}, "-list and -export can't be used together"},
		{[]string{"-list", "colours"}, "unknown resource type colours, use one of: quotes, themes, words"},
		{[]string{"-layout", "nope"}, "unknown layout nope, use one of: "},
		{[]string{"-csv", "-json"}, "-csv and -json can't be used together"},
	}

	for _, tc := range tests {
		if _, err := resolve(parseFlags(tc.args), true); err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("tt %s: error %v, want %q", strings.Join(tc.args, " "), err, tc.want)
		}
	}
}

func TestResolveAcceptsValidOptions(t *testing.T) {
	t.Parallel()

	valid := [][]string{
		nil,
		{"-n", "1", "-g", "1", "-w", "1", "-t", "1", "-pace", "0"},
		{"-t", "-1"},
		{"-pace", "best"},
		{"-pace", "avg"},
		{"-adaptive", "-focus", "0"},
		{"-adaptive", "-focus", "100"},
		{"-list", "words"},
		{"-keys", "-layout", "it"},
		{"-export", "results", "-json"},
	}

	for _, args := range valid {
		if _, err := resolve(parseFlags(args), true); err != nil {
			t.Errorf("%q: unexpected error %v", args, err)
		}
	}
}

func TestResolveCommand(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"list":   {"-list", "themes"},
		"v":      {"-v"},
		"stats":  {"-stats"},
		"keys":   {"-keys"},
		"export": {"-export", "results"},
	}

	for flag, args := range tests {
		if s := resolveFlags(args); s.command == nil || s.command.flag != flag {
			t.Errorf("tt %s: command %+v, want -%s", strings.Join(args, " "), s.command, flag)
		}
	}
}

// Each component gets the group of settings it concerns.
func TestResolveGroups(t *testing.T) {
	t.Parallel()

	s := resolveFlags([]string{
		"-theme", "nord", "-notransparent", "-bold", "-highlight1", "-blockcursor", "-showwpm", "-showaccuracy", "-w", "60",
		"-t", "30", "-noskip", "-nobackspace", "-blind", "-pace", "best",
		"-noreport", "-oneshot", "-csv",
		"-n", "5", "-g", "2", "-punct", "-numbers",
	})

	wantDisplay := displaySettings{
		theme: "nord", themeExplicit: true, opaqueBg: true, bold: true,
		highlightCurrent: true, blockCursor: true, showWpm: true, showAccuracy: true,
		maxLineLen: 60, reflow: true,
	}
	if s.display != wantDisplay {
		t.Errorf("display = %+v, want %+v", s.display, wantDisplay)
	}

	wantRules := ruleSettings{timeout: 30 * time.Second, noSkip: true, noBackspace: true, blind: true, paceTarget: "best"}
	if s.rules != wantRules {
		t.Errorf("rules = %+v, want %+v", s.rules, wantRules)
	}

	if want := (outputSettings{noReport: true, oneShot: true, format: "csv"}); s.output != want {
		t.Errorf("output = %+v, want %+v", s.output, want)
	}

	wantWords := wordOptions{n: 5, g: 2, punctuation: true, numbers: true, minChars: 30 * fastestCpm / 60}
	if s.source.words != wantWords {
		t.Errorf("words = %+v, want %+v", s.source.words, wantWords)
	}

	if d := resolveFlags(nil).display; d.theme != "default" || d.themeExplicit || !d.highlightCurrent || !d.highlightNext {
		t.Errorf("default display = %+v, want the implicit default theme and both highlights", d)
	}
	if d := resolveFlags([]string{"-nohighlight", "-raw"}).display; d.highlightCurrent || d.highlightNext || d.reflow {
		t.Errorf("display = %+v, want no highlight and no reflow", d)
	}
	if r := resolveFlags([]string{"-pace", "70"}).rules; r.pace != 70 || r.paceTarget != "" {
		t.Errorf("rules = %+v, want a pace of 70 wpm", r)
	}
}
