package main

import "time"

// typingState tracks the progress through the text of a single segment,
// independently of how it is displayed.
type typingState struct {
	text  []rune
	typed []rune // What was typed at each position, 0 for skipped characters.
	idx   int    // Position of the cursor.

	correctKeys int // Keystrokes matching the expected character.
	wrongKeys   int // Keystrokes not matching the expected character.
	keystrokes  []keystroke
}

// keystroke is a key pressed to type a character of the text.
type keystroke struct {
	expected rune          // The character of the text, 0 for a break in the sequence (a correction or a skipped word).
	typed    rune          // The character typed.
	at       time.Duration // Time since the start of the test.
}

func newTypingState(s string) *typingState {
	text := []rune(s)

	return &typingState{
		text:  text,
		typed: make([]rune, len(text)),
	}
}

func (s *typingState) done() bool {
	return s.idx == len(s.text)
}

// expects reports whether r is the next character to be typed.
func (s *typingState) expects(r rune) bool {
	return s.idx < len(s.text) && s.text[s.idx] == r
}

// typeRune handles a character typed at the given time since the start of the
// test. If skipWord is set a space moves the cursor to the next word, leaving
// the rest of the current one untyped.
func (s *typingState) typeRune(r rune, skipWord bool, at time.Duration) {
	if s.done() {
		return
	}

	if skipWord && r == ' ' {
		if s.idx > 0 && isWordSeparator(s.text[s.idx-1]) && s.text[s.idx] != ' ' { //Do nothing on word boundaries.
			return
		}

		start := s.idx
		for s.idx < len(s.text) && !isWordSeparator(s.text[s.idx]) {
			s.typed[s.idx] = 0
			s.idx++
		}

		//A space typed on a space is a regular keystroke, skipping anything breaks the sequence.
		if s.idx == start && s.text[s.idx] == ' ' {
			s.keystrokes = append(s.keystrokes, keystroke{' ', ' ', at})
		} else {
			s.keystrokes = append(s.keystrokes, keystroke{})
		}

		if s.idx < len(s.text) {
			s.typed[s.idx] = s.text[s.idx]
			s.idx++
		}
	} else {
		s.keystrokes = append(s.keystrokes, keystroke{s.text[s.idx], r, at})
		s.typed[s.idx] = r
		if r == s.text[s.idx] {
			s.correctKeys++
		} else {
			s.wrongKeys++
		}
		s.idx++
	}

	//Line breaks are never typed.
	for s.idx < len(s.text) && s.text[s.idx] == '\n' {
		s.typed[s.idx] = s.text[s.idx]
		s.idx++
	}
}

// backspace moves the cursor back by one character, skipping line breaks.
func (s *typingState) backspace() {
	if s.idx == 0 {
		return
	}

	s.keystrokes = append(s.keystrokes, keystroke{})

	s.idx--

	for s.idx > 0 && s.text[s.idx] == '\n' {
		s.idx--
	}
}

// deleteWord moves the cursor back to the start of the current word, or of
// the previous one if the cursor is already at the start of a word.
func (s *typingState) deleteWord() {
	if s.idx == 0 {
		return
	}

	s.keystrokes = append(s.keystrokes, keystroke{})

	s.idx--

	for s.idx > 0 && isWordSeparator(s.text[s.idx]) {
		s.idx--
	}

	for s.idx > 0 && !isWordSeparator(s.text[s.idx]) {
		s.idx--
	}

	if isWordSeparator(s.text[s.idx]) {
		s.typed[s.idx] = s.text[s.idx]
		s.idx++
	}
}

// correctChars returns the number of correctly typed characters before the
// cursor, line breaks excluded.
func (s *typingState) correctChars() int {
	n := 0

	for i := 0; i < s.idx; i++ {
		if s.text[i] != '\n' && s.text[i] == s.typed[i] {
			n++
		}
	}

	return n
}

// mistakes returns the mistyped words before the cursor.
func (s *typingState) mistakes() []mistake {
	return extractMistypedWords(s.text[:s.idx], s.typed[:s.idx])
}

func extractMistypedWords(text []rune, typed []rune) (mistakes []mistake) {
	var w []rune
	var t []rune
	f := false

	for i := range text {
		if isWordSeparator(text[i]) {
			if f {
				mistakes = append(mistakes, mistake{string(w), string(t)})
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
		mistakes = append(mistakes, mistake{string(w), string(t)})
	}

	return
}

func isWordSeparator(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t'
}

// highlightWordPositions returns, for each character of text, the position
// relative to the cursor of the word it belongs to (0 for the current word, 1
// for the next one and so on), or -1 for characters before the cursor and
// separators.
func highlightWordPositions(text []rune, idx int) []int {
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
			if isWordSeparator(r) {
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

		if isWordSeparator(r) {
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

// paceIndex returns the position in text of a typist starting at its
// beginning and typing at wpm (5 characters per word) for the elapsed time.
// Line breaks, which are never typed, are skipped.
func paceIndex(text []rune, elapsed time.Duration, wpm int) int {
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
