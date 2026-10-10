package main

import (
	"reflect"
	"strings"
	"testing"
)

func assertWordPosition(t *testing.T, positions []int, text, word string, want int) {
	t.Helper()

	start := strings.Index(text, word)
	if start == -1 {
		t.Fatalf("word %q not found in %q", word, text)
	}

	for i := start; i < start+len(word); i++ {
		if positions[i] != want {
			t.Fatalf("word %q at char %d: got %d, want %d", word, i-start, positions[i], want)
		}
	}
}

func TestHighlightWordPositionsAcrossWrappedLineBreak(t *testing.T) {
	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, "beta")

	positions := highlightWordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "beta", 0)
	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func TestHighlightWordPositionsWhenCursorIsOnWrapWhitespace(t *testing.T) {
	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, " \n")

	positions := highlightWordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func TestExtractMistypedWords(t *testing.T) {
	tests := []struct {
		text, typed string
		want        []mistake
	}{
		{"alpha beta", "alpha beta", nil},
		{"alpha beta", "alpha betx", []mistake{{"beta", "betx"}}},
		{"alpha beta", "alxha bxta", []mistake{{"alpha", "alxha"}, {"beta", "bxta"}}},
		{"ab cd", "a\x00 cd", []mistake{{"ab", "a_"}}},
		{"ab cd", "ab cd"[:4] + "\x00", []mistake{{"cd", "c_"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %v, want %v", tc.text, tc.typed, got, tc.want)
		}
	}
}

func TestExtractMistypedWordsAcrossLines(t *testing.T) {
	tests := []struct {
		text, typed string
		want        []mistake
	}{
		{"ab \ncd", "ab \ncx", []mistake{{"cd", "cx"}}},
		{"ab\ncd", "ab\ncx", []mistake{{"cd", "cx"}}},
		{"ab\ncd", "xb\ncd", []mistake{{"ab", "xb"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %q, want %q", tc.text, tc.typed, got, tc.want)
		}
	}
}

// typeKeys feeds keys to a new typingState for text. '\b' stands for
// backspace and '\x17' (C-w) for deleting a word.
func typeKeys(text, keys string, skipWord bool) *typingState {
	s := newTypingState(text)

	for _, r := range keys {
		switch r {
		case '\b':
			s.backspace()
		case '\x17':
			s.deleteWord()
		default:
			s.typeRune(r, skipWord)
		}
	}

	return s
}

func TestTypingState(t *testing.T) {
	tests := []struct {
		name         string
		text, keys   string
		noSkip       bool
		idx          int
		correctChars int
		correctKeys  int
		wrongKeys    int
		mistakes     []mistake
	}{
		{"correct", "ab cd", "ab cd", false, 5, 5, 4, 0, nil},
		{"typo", "ab cd", "ax", false, 2, 1, 1, 1, []mistake{{"ab", "ax"}}},
		{"backspace", "ab cd", "x\bab", false, 2, 2, 2, 1, nil},
		{"backspace at start", "ab", "\b\ba", false, 1, 1, 1, 0, nil},
		{"space skips word", "ab cd", " ", false, 3, 1, 0, 0, []mistake{{"ab", "__"}}},
		{"space at word start", "ab cd", "ab  ", false, 3, 3, 2, 0, nil},
		{"space without skipping", "ab cd", "a ", true, 2, 1, 1, 1, []mistake{{"ab", "a "}}},
		{"line breaks are skipped", "ab \ncd", "ab ", false, 4, 3, 2, 0, nil},
		{"backspace over line break", "ab \ncd", "ab \b", false, 2, 2, 2, 0, nil},
		{"delete current word", "ab cd", "ab c\x17", false, 3, 3, 3, 0, nil},
		{"delete previous word", "ab cd", "ab \x17", false, 0, 0, 2, 0, nil},
		{"delete word across line break", "ab \ncd", "ab \x17", false, 0, 0, 2, 0, nil},
		{"keys after the end are ignored", "ab", "abc", false, 2, 2, 2, 0, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := typeKeys(tc.text, tc.keys, !tc.noSkip)

			if s.idx != tc.idx {
				t.Errorf("idx = %d, want %d", s.idx, tc.idx)
			}
			if got := s.correctChars(); got != tc.correctChars {
				t.Errorf("correctChars() = %d, want %d", got, tc.correctChars)
			}
			if s.correctKeys != tc.correctKeys || s.wrongKeys != tc.wrongKeys {
				t.Errorf("keystrokes = %d correct, %d wrong, want %d, %d", s.correctKeys, s.wrongKeys, tc.correctKeys, tc.wrongKeys)
			}
			if got := s.mistakes(); !reflect.DeepEqual(got, tc.mistakes) {
				t.Errorf("mistakes() = %q, want %q", got, tc.mistakes)
			}
		})
	}
}

func TestTypingStateDone(t *testing.T) {
	if !newTypingState("").done() {
		t.Error("an empty text should be done")
	}

	s := typeKeys("a?", "", true)
	if s.done() || !s.expects('a') || s.expects('?') {
		t.Errorf("at the start: done %t, expects a %t, expects ? %t", s.done(), s.expects('a'), s.expects('?'))
	}

	s.typeRune('a', true)
	if s.done() || !s.expects('?') {
		t.Errorf("after a: done %t, expects ? %t", s.done(), s.expects('?'))
	}

	s.typeRune('?', true)
	if !s.done() || s.expects('?') {
		t.Errorf("at the end: done %t, expects ? %t", s.done(), s.expects('?'))
	}
}

func TestTypingStateTabs(t *testing.T) {
	tests := []struct {
		name         string
		text, keys   string
		noSkip       bool
		idx          int
		correctChars int
		mistakes     []mistake
	}{
		{"tab typed", "a\n\tb", "a\tb", false, 4, 3, nil},
		{"tabs separate mistyped words", "ab\tcd", "ab\tcx", false, 5, 4, []mistake{{"cd", "cx"}}},
		{"space at indentation is ignored", "a\n\tb", "a ", false, 2, 1, nil},
		{"space at indentation without skipping", "a\n\tb", "a ", true, 3, 1, nil},
		{"space skips to a tab", "ab\tcd", "a ", false, 3, 2, []mistake{{"ab", "a_"}}},
		{"delete word stops at a tab", "ab\tcd", "ab\tc\x17", false, 3, 3, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := typeKeys(tc.text, tc.keys, !tc.noSkip)

			if s.idx != tc.idx {
				t.Errorf("idx = %d, want %d", s.idx, tc.idx)
			}
			if got := s.correctChars(); got != tc.correctChars {
				t.Errorf("correctChars() = %d, want %d", got, tc.correctChars)
			}
			if got := s.mistakes(); !reflect.DeepEqual(got, tc.mistakes) {
				t.Errorf("mistakes() = %q, want %q", got, tc.mistakes)
			}
		})
	}
}
