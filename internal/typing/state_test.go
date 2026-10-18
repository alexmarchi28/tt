package typing

import (
	"reflect"
	"strings"
	"testing"
	"time"
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
	t.Parallel()

	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, "beta")

	positions := WordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "beta", 0)
	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func TestHighlightWordPositionsWhenCursorIsOnWrapWhitespace(t *testing.T) {
	t.Parallel()

	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, " \n")

	positions := WordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func TestExtractMistypedWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text, typed string
		want        []Mistake
	}{
		{"alpha beta", "alpha beta", nil},
		{"alpha beta", "alpha betx", []Mistake{{"beta", "betx"}}},
		{"alpha beta", "alxha bxta", []Mistake{{"alpha", "alxha"}, {"beta", "bxta"}}},
		{"ab cd", "a\x00 cd", []Mistake{{"ab", "a_"}}},
		{"ab cd", "ab cd"[:4] + "\x00", []Mistake{{"cd", "c_"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %v, want %v", tc.text, tc.typed, got, tc.want)
		}
	}
}

func TestExtractMistypedWordsAcrossLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text, typed string
		want        []Mistake
	}{
		{"ab \ncd", "ab \ncx", []Mistake{{"cd", "cx"}}},
		{"ab\ncd", "ab\ncx", []Mistake{{"cd", "cx"}}},
		{"ab\ncd", "xb\ncd", []Mistake{{"ab", "xb"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %q, want %q", tc.text, tc.typed, got, tc.want)
		}
	}
}

// typeKeys feeds keys to a new State for text, one every 100ms. '\b'
// stands for backspace and '\x17' (C-w) for deleting a word.
func typeKeys(text, keys string, skipWord bool) *State {
	s := NewState(text)

	for i, r := range []rune(keys) {
		switch r {
		case '\b':
			s.Backspace()
		case '\x17':
			s.DeleteWord()
		default:
			s.Type(r, skipWord, time.Duration(i)*100*time.Millisecond)
		}
	}

	return s
}

func TestTypingState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		text, keys   string
		noSkip       bool
		idx          int
		correctChars int
		correctKeys  int
		wrongKeys    int
		mistakes     []Mistake
	}{
		{"correct", "ab cd", "ab cd", false, 5, 5, 4, 0, nil},
		{"typo", "ab cd", "ax", false, 2, 1, 1, 1, []Mistake{{"ab", "ax"}}},
		{"backspace", "ab cd", "x\bab", false, 2, 2, 2, 1, nil},
		{"backspace at start", "ab", "\b\ba", false, 1, 1, 1, 0, nil},
		{"space skips word", "ab cd", " ", false, 3, 1, 0, 0, []Mistake{{"ab", "__"}}},
		{"space at word start", "ab cd", "ab  ", false, 3, 3, 2, 0, nil},
		{"space without skipping", "ab cd", "a ", true, 2, 1, 1, 1, []Mistake{{"ab", "a "}}},
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
			if got := s.CorrectChars(); got != tc.correctChars {
				t.Errorf("correctChars() = %d, want %d", got, tc.correctChars)
			}
			if s.correctKeys != tc.correctKeys || s.wrongKeys != tc.wrongKeys {
				t.Errorf("keystrokes = %d correct, %d wrong, want %d, %d", s.correctKeys, s.wrongKeys, tc.correctKeys, tc.wrongKeys)
			}
			if got := s.Mistakes(); !reflect.DeepEqual(got, tc.mistakes) {
				t.Errorf("mistakes() = %q, want %q", got, tc.mistakes)
			}
		})
	}
}

func TestTypingStateDone(t *testing.T) {
	t.Parallel()

	if !NewState("").Done() {
		t.Error("an empty text should be done")
	}

	s := typeKeys("a?", "", true)
	if s.Done() || !s.Expects('a') || s.Expects('?') {
		t.Errorf("at the start: done %t, expects a %t, expects ? %t", s.Done(), s.Expects('a'), s.Expects('?'))
	}

	s.Type('a', true, 0)
	if s.Done() || !s.Expects('?') {
		t.Errorf("after a: done %t, expects ? %t", s.Done(), s.Expects('?'))
	}

	s.Type('?', true, 0)
	if !s.Done() || s.Expects('?') {
		t.Errorf("at the end: done %t, expects ? %t", s.Done(), s.Expects('?'))
	}
}

func TestTypingStateTabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		text, keys   string
		noSkip       bool
		idx          int
		correctChars int
		mistakes     []Mistake
	}{
		{"tab typed", "a\n\tb", "a\tb", false, 4, 3, nil},
		{"tabs separate mistyped words", "ab\tcd", "ab\tcx", false, 5, 4, []Mistake{{"cd", "cx"}}},
		{"space at indentation is ignored", "a\n\tb", "a ", false, 2, 1, nil},
		{"space at indentation without skipping", "a\n\tb", "a ", true, 3, 1, nil},
		{"space skips to a tab", "ab\tcd", "a ", false, 3, 2, []Mistake{{"ab", "a_"}}},
		{"delete word stops at a tab", "ab\tcd", "ab\tc\x17", false, 3, 3, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := typeKeys(tc.text, tc.keys, !tc.noSkip)

			if s.idx != tc.idx {
				t.Errorf("idx = %d, want %d", s.idx, tc.idx)
			}
			if got := s.CorrectChars(); got != tc.correctChars {
				t.Errorf("correctChars() = %d, want %d", got, tc.correctChars)
			}
			if got := s.Mistakes(); !reflect.DeepEqual(got, tc.mistakes) {
				t.Errorf("mistakes() = %q, want %q", got, tc.mistakes)
			}
		})
	}
}

func TestPaceIndex(t *testing.T) {
	t.Parallel()

	// At 60 wpm a pacer types 5 characters per second.
	text := []rune("ab \ncd")

	tests := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{199 * time.Millisecond, 0},
		{200 * time.Millisecond, 1},
		{400 * time.Millisecond, 2},
		{600 * time.Millisecond, 4}, // The line break is not typed.
		{800 * time.Millisecond, 5},
		{time.Second, 6},
		{time.Hour, 6},
	}

	for _, tc := range tests {
		if got := PaceIndex(text, tc.elapsed, 60); got != tc.want {
			t.Errorf("PaceIndex(%v) = %d, want %d", tc.elapsed, got, tc.want)
		}
	}

	if got := PaceIndex(text, time.Second, 120); got != 6 {
		t.Errorf("at 120 wpm PaceIndex(1s) = %d, want the end of the text", got)
	}
}

func TestTypingStateRecordsKeystrokes(t *testing.T) {
	t.Parallel()

	ms := time.Millisecond
	brk := Keystroke{}

	tests := []struct {
		name       string
		text, keys string
		noSkip     bool
		want       []Keystroke
	}{
		{"typed", "ab cd", "ax c", false, []Keystroke{{'a', 'a', 0}, {'b', 'x', 100 * ms}, {' ', ' ', 200 * ms}, {'c', 'c', 300 * ms}}},
		{"corrections break the sequence", "ab cd", "x\ba\x17ab", false, []Keystroke{
			{'a', 'x', 0}, brk, {'a', 'a', 200 * ms}, brk, {'a', 'a', 400 * ms}, {'b', 'b', 500 * ms},
		}},
		{"skipped words break the sequence", "ab cd", "a c", false, []Keystroke{{'a', 'a', 0}, brk, {'c', 'c', 200 * ms}}},
		{"spaces typed without skipping", "ab cd", "a c", true, []Keystroke{{'a', 'a', 0}, {'b', ' ', 100 * ms}, {' ', 'c', 200 * ms}}},
		{"ignored space", "ab cd", "ab  c", false, []Keystroke{{'a', 'a', 0}, {'b', 'b', 100 * ms}, {' ', ' ', 200 * ms}, {'c', 'c', 400 * ms}}},
		{"line breaks are not keystrokes", "a\nb", "ab", false, []Keystroke{{'a', 'a', 0}, {'b', 'b', 100 * ms}}},
		{"space on a tab", "ab\tc", "ab c", false, []Keystroke{{'a', 'a', 0}, {'b', 'b', 100 * ms}, brk, {'c', 'c', 300 * ms}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := typeKeys(tc.text, tc.keys, !tc.noSkip).keystrokes
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("keystrokes = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTypingStateRelayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		from, to     string
		keys         string
		idx          int
		correctChars int
		mistakes     []Mistake
	}{
		{"line break added before the cursor", "ab cd ef", "ab \ncd ef", "ab cx", 6, 4, []Mistake{{"cd", "cx"}}},
		{"line break removed before the cursor", "ab \ncd ef", "ab cd ef", "ab cx", 5, 4, []Mistake{{"cd", "cx"}}},
		{"line break moved after the cursor", "ab \ncd ef", "ab cd \nef", "a", 1, 1, nil},
		{"skipped word", "ab cd ef", "ab \ncd \nef", "a c", 5, 3, []Mistake{{"ab", "a_"}}},
		{"nothing typed", "ab cd", "ab \ncd", "", 0, 0, nil},
		{"text complete", "ab cd", "ab \ncd", "ab cd", 6, 5, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := typeKeys(tc.from, tc.keys, true)
			keystrokes := append([]Keystroke(nil), s.keystrokes...)

			s.Relayout(tc.to)

			if string(s.text) != tc.to {
				t.Errorf("text = %q, want %q", string(s.text), tc.to)
			}
			if s.idx != tc.idx {
				t.Errorf("idx = %d, want %d", s.idx, tc.idx)
			}
			if got := s.CorrectChars(); got != tc.correctChars {
				t.Errorf("correctChars() = %d, want %d", got, tc.correctChars)
			}
			if got := s.Mistakes(); !reflect.DeepEqual(got, tc.mistakes) {
				t.Errorf("mistakes() = %q, want %q", got, tc.mistakes)
			}
			if !reflect.DeepEqual(s.keystrokes, keystrokes) {
				t.Errorf("keystrokes = %v, want %v", s.keystrokes, keystrokes)
			}
		})
	}
}

func TestTypingStateRelayoutKeepsTyping(t *testing.T) {
	t.Parallel()

	s := typeKeys("ab cd ef", "ab c", true)
	s.Relayout("ab \ncd \nef")

	for _, r := range "d ef" {
		s.Type(r, true, 0)
	}

	if !s.Done() || s.CorrectChars() != 8 || s.Mistakes() != nil {
		t.Errorf("done %t, %d correct characters, mistakes %q, want done, 8, none", s.Done(), s.CorrectChars(), s.Mistakes())
	}
}

func TestStateAccessors(t *testing.T) {
	t.Parallel()

	s := NewState("ab")
	s.Type('x', false, 0)

	if string(s.Text()) != "ab" || string(s.Typed()) != "x\x00" || s.Cursor() != 1 {
		t.Errorf("text %q, typed %q, cursor %d, want ab, x and 1", string(s.Text()), string(s.Typed()), s.Cursor())
	}
	if s.CorrectKeys() != 0 || s.WrongKeys() != 1 || !reflect.DeepEqual(s.Keystrokes(), []Keystroke{{'a', 'x', 0}}) {
		t.Errorf("correct keys %d, wrong keys %d, keystrokes %v", s.CorrectKeys(), s.WrongKeys(), s.Keystrokes())
	}
}
