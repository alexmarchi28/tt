// Package typing follows the typing of a text: what was typed at each
// position, the keystrokes and the mistakes, independently of how the text is
// displayed.
package typing

import "time"

// State tracks the progress through the text of a single segment,
// independently of how it is displayed.
type State struct {
	text  []rune
	typed []rune // What was typed at each position, 0 for skipped characters.
	idx   int    // Position of the cursor.

	correctKeys int // Keystrokes matching the expected character.
	wrongKeys   int // Keystrokes not matching the expected character.
	keystrokes  []Keystroke
}

// Keystroke is a key pressed to type a character of the text.
type Keystroke struct {
	Expected rune          // The character of the text, 0 for a break in the sequence (a correction or a skipped word).
	Typed    rune          // The character typed.
	At       time.Duration // Time since the start of the test.
}

// Mistake is a word mistyped.
type Mistake struct {
	Word  string `json:"word"`
	Typed string `json:"typed"`
}

// NewState starts the typing of s.
func NewState(s string) *State {
	text := []rune(s)

	return &State{
		text:  text,
		typed: make([]rune, len(text)),
	}
}

// Text returns the text typed, which must not be modified.
func (s *State) Text() []rune { return s.text }

// Typed returns what was typed at each position of the text, 0 for skipped
// characters. It must not be modified.
func (s *State) Typed() []rune { return s.typed }

// Cursor returns the position of the cursor in the text.
func (s *State) Cursor() int { return s.idx }

// CorrectKeys returns the number of keystrokes matching the expected character.
func (s *State) CorrectKeys() int { return s.correctKeys }

// WrongKeys returns the number of keystrokes not matching the expected
// character.
func (s *State) WrongKeys() int { return s.wrongKeys }

// Keystrokes returns the keystrokes typed, see Keystroke.
func (s *State) Keystrokes() []Keystroke { return s.keystrokes }

// Done reports whether the whole text was typed.
func (s *State) Done() bool {
	return s.idx == len(s.text)
}

// Expects reports whether r is the next character to be typed.
func (s *State) Expects(r rune) bool {
	return s.idx < len(s.text) && s.text[s.idx] == r
}

// Type handles a character typed at the given time since the start of the
// test. If skipWord is set a space moves the cursor to the next word, leaving
// the rest of the current one untyped.
func (s *State) Type(r rune, skipWord bool, at time.Duration) {
	if s.Done() {
		return
	}

	if skipWord && r == ' ' {
		if s.idx > 0 && IsWordSeparator(s.text[s.idx-1]) && s.text[s.idx] != ' ' { //Do nothing on word boundaries.
			return
		}

		start := s.idx
		for s.idx < len(s.text) && !IsWordSeparator(s.text[s.idx]) {
			s.typed[s.idx] = 0
			s.idx++
		}

		//A space typed on a space is a regular keystroke, skipping anything breaks the sequence.
		if s.idx == start && s.text[s.idx] == ' ' {
			s.keystrokes = append(s.keystrokes, Keystroke{' ', ' ', at})
		} else {
			s.keystrokes = append(s.keystrokes, Keystroke{})
		}

		if s.idx < len(s.text) {
			s.typed[s.idx] = s.text[s.idx]
			s.idx++
		}
	} else {
		s.keystrokes = append(s.keystrokes, Keystroke{s.text[s.idx], r, at})
		s.typed[s.idx] = r
		if r == s.text[s.idx] {
			s.correctKeys++
		} else {
			s.wrongKeys++
		}
		s.idx++
	}

	s.skipLineBreaks()
}

// Backspace moves the cursor back by one character, skipping line breaks.
func (s *State) Backspace() {
	if s.idx == 0 {
		return
	}

	s.keystrokes = append(s.keystrokes, Keystroke{})

	s.idx--

	for s.idx > 0 && s.text[s.idx] == '\n' {
		s.idx--
	}
}

// DeleteWord moves the cursor back to the start of the current word, or of
// the previous one if the cursor is already at the start of a word.
func (s *State) DeleteWord() {
	if s.idx == 0 {
		return
	}

	s.keystrokes = append(s.keystrokes, Keystroke{})

	s.idx--

	for s.idx > 0 && IsWordSeparator(s.text[s.idx]) {
		s.idx--
	}

	for s.idx > 0 && !IsWordSeparator(s.text[s.idx]) {
		s.idx--
	}

	if IsWordSeparator(s.text[s.idx]) {
		s.typed[s.idx] = s.text[s.idx]
		s.idx++
	}
}

// Relayout replaces the text with the same characters arranged on different
// lines, as when it is wrapped at another width, keeping what was typed. The
// texts may differ only in their line breaks.
func (s *State) Relayout(text string) {
	var typed []rune // What was typed at each position before the cursor, line breaks excluded.
	for i := 0; i < s.idx; i++ {
		if s.text[i] != '\n' {
			typed = append(typed, s.typed[i])
		}
	}

	s.text = []rune(text)
	s.typed = make([]rune, len(s.text))
	s.idx = 0

	for _, r := range typed {
		s.skipLineBreaks()
		s.typed[s.idx] = r
		s.idx++
	}

	if s.idx > 0 {
		s.skipLineBreaks()
	}
}

// skipLineBreaks moves the cursor past the line breaks at its position, which
// are never typed.
func (s *State) skipLineBreaks() {
	for s.idx < len(s.text) && s.text[s.idx] == '\n' {
		s.typed[s.idx] = s.text[s.idx]
		s.idx++
	}
}

// CorrectChars returns the number of correctly typed characters before the
// cursor, line breaks excluded.
func (s *State) CorrectChars() int {
	n := 0

	for i := 0; i < s.idx; i++ {
		if s.text[i] != '\n' && s.text[i] == s.typed[i] {
			n++
		}
	}

	return n
}

// Mistakes returns the mistyped words before the cursor.
func (s *State) Mistakes() []Mistake {
	return extractMistypedWords(s.text[:s.idx], s.typed[:s.idx])
}

func extractMistypedWords(text []rune, typed []rune) (mistakes []Mistake) {
	var w []rune
	var t []rune
	f := false

	for i := range text {
		if IsWordSeparator(text[i]) {
			if f {
				mistakes = append(mistakes, Mistake{string(w), string(t)})
			}

			w = w[:0]
			t = t[:0]
			f = false
			continue
		}

		if text[i] != typed[i] {
			f = true
		}

		if text[i] == 0 {
			w = append(w, '_')
		} else {
			w = append(w, text[i])
		}

		if typed[i] == 0 {
			t = append(t, '_')
		} else {
			t = append(t, typed[i])
		}
	}

	if f {
		mistakes = append(mistakes, Mistake{string(w), string(t)})
	}

	return
}

// IsWordSeparator reports whether r separates words.
func IsWordSeparator(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t'
}

// WordPositions returns, for each character of text, the position
// relative to the cursor of the word it belongs to (0 for the current word, 1
// for the next one and so on), or -1 for characters before the cursor and
// separators.
func WordPositions(text []rune, idx int) []int {
	positions := make([]int, len(text))
	for i := range positions {
		positions[i] = -1
	}

	if idx < 0 || idx >= len(text) {
		return positions
	}

	wordPos := -1
	insideWord := false

	for i, r := range text {
		if i == idx {
			if IsWordSeparator(r) {
				wordPos = 1
				insideWord = false
			} else {
				wordPos = 0
				insideWord = true
			}
		}

		if i < idx || wordPos == -1 {
			continue
		}

		if IsWordSeparator(r) {
			if insideWord {
				wordPos++
				insideWord = false
			}
			continue
		}

		insideWord = true
		positions[i] = wordPos
	}

	return positions
}

// PaceIndex returns the position in text of a typist starting at its
// beginning and typing at wpm (5 characters per word) for the elapsed time.
// Line breaks, which are never typed, are skipped.
func PaceIndex(text []rune, elapsed time.Duration, wpm int) int {
	chars := int(int64(elapsed) * int64(wpm) * 5 / int64(time.Minute))

	for i, r := range text {
		if r == '\n' {
			continue
		}
		if chars == 0 {
			return i
		}
		chars--
	}

	return len(text)
}
