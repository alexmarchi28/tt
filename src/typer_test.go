package main

import (
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
