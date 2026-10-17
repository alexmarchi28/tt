package main

import (
	"fmt"
	"strings"
	"time"
)

// Modes of tests, saved with their results.
const (
	modeWords    = "words"
	modeQuotes   = "quotes"
	modeMistakes = "mistakes"
	modeAdaptive = "adaptive"
	modeFile     = "file"
	modeText     = "text"
)

// testKind describes the tests whose results are compared with each other:
// where their text comes from and how it is made.
type testKind struct {
	mode   string // One of the modes above.
	source string // The word list, quote file or file used, if any.

	seconds int  // Time limit, 0 for none.
	words   int  // Words of untimed tests made of words, 0 for other tests.
	punct   bool // Whether the words are turned into sentences.
	numbers bool // Whether some of the words are replaced with numbers.
	focus   int  // Percentage of the words of adaptive tests with the pairs practiced.
}

func (k testKind) String() string {
	if k.mode == "" {
		return "unknown"
	}

	name := k.mode
	if k.source != "" {
		name += " " + k.source
	}

	return strings.Join(append([]string{name}, k.variant()...), ", ")
}

// variant describes how the text of the tests is made.
func (k testKind) variant() []string {
	var v []string
	if k.seconds > 0 {
		v = append(v, fmt.Sprintf("%ds", k.seconds))
	}
	if k.words > 0 {
		v = append(v, fmt.Sprintf("%d words", k.words))
	}
	if k.punct {
		v = append(v, "punctuation")
	}
	if k.numbers {
		v = append(v, "numbers")
	}
	if k.focus > 0 {
		v = append(v, fmt.Sprintf("focus %d%%", k.focus))
	}

	return v
}

// madeOfWords reports whether the tests are made of random words, see
// wordOptions.
func (k testKind) madeOfWords() bool {
	return k.mode == modeWords || k.mode == modeMistakes || k.mode == modeAdaptive
}

// withVariant returns k completed with how the tests are made according to
// the command line options.
func (k testKind) withVariant(opts options) testKind {
	if opts.timeout != -1 {
		k.seconds = int(opts.timeout / time.Second)
	}

	if k.madeOfWords() {
		k.punct, k.numbers = opts.punct, opts.numbers
		if opts.timeout == -1 {
			k.words = opts.n * opts.g
		}
	}

	if k.mode == modeAdaptive {
		k.focus = opts.focus
	}

	return k
}
