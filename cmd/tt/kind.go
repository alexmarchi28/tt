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
// Its fields are saved with the results, see persistedResult, and are empty
// for results saved by older versions.
type testKind struct {
	Mode   string `json:"mode,omitempty"`   // One of the modes above.
	Source string `json:"source,omitempty"` // The word list, quote file or file used, if any.

	Seconds int  `json:"seconds,omitempty"` // Time limit, 0 for none.
	Words   int  `json:"words,omitempty"`   // Words of untimed tests made of words, 0 for other tests.
	Punct   bool `json:"punct,omitempty"`   // Whether the words are turned into sentences.
	Numbers bool `json:"numbers,omitempty"` // Whether some of the words are replaced with numbers.
	Focus   int  `json:"focus,omitempty"`   // Percentage of the words of adaptive tests with the pairs practiced.
}

func (k testKind) String() string {
	if k.Mode == "" {
		return "unknown"
	}

	name := k.Mode
	if k.Source != "" {
		name += " " + k.Source
	}

	return strings.Join(append([]string{name}, k.variant()...), ", ")
}

// variant describes how the text of the tests is made.
func (k testKind) variant() []string {
	var v []string
	if k.Seconds > 0 {
		v = append(v, fmt.Sprintf("%ds", k.Seconds))
	}
	if k.Words > 0 {
		v = append(v, fmt.Sprintf("%d words", k.Words))
	}
	if k.Punct {
		v = append(v, "punctuation")
	}
	if k.Numbers {
		v = append(v, "numbers")
	}
	if k.Focus > 0 {
		v = append(v, fmt.Sprintf("focus %d%%", k.Focus))
	}

	return v
}

// madeOfWords reports whether the tests are made of random words, see
// wordOptions.
func (k testKind) madeOfWords() bool {
	return k.Mode == modeWords || k.Mode == modeMistakes || k.Mode == modeAdaptive
}

// withVariant returns k completed with how the tests are made according to
// the command line options.
func (k testKind) withVariant(opts options) testKind {
	if opts.timeout != -1 {
		k.Seconds = int(opts.timeout / time.Second)
	}

	if k.madeOfWords() {
		k.Punct, k.Numbers = opts.punct, opts.numbers
		if opts.timeout == -1 {
			k.Words = opts.n * opts.g
		}
	}

	if k.Mode == modeAdaptive {
		k.Focus = opts.focus
	}

	return k
}
