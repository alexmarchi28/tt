// Package history describes the results of the tests saved between runs,
// and the kinds of tests whose results are compared.
package history

import (
	"fmt"
	"strings"
)

// Modes of tests, saved with their results.
const (
	ModeWords    = "words"
	ModeQuotes   = "quotes"
	ModeMistakes = "mistakes"
	ModeAdaptive = "adaptive"
	ModeFile     = "file"
	ModeText     = "text"
)

// Kind describes the tests whose results are compared with each other:
// where their text comes from and how it is made.
// Its fields are saved with the results, see Result, and are empty
// for results saved by older versions.
type Kind struct {
	Mode   string `json:"mode,omitempty"`   // One of the modes above.
	Source string `json:"source,omitempty"` // The word list, quote file or file used, if any.

	Seconds int  `json:"seconds,omitempty"` // Time limit, 0 for none.
	Words   int  `json:"words,omitempty"`   // Words of untimed tests made of words, 0 for other tests.
	Punct   bool `json:"punct,omitempty"`   // Whether the words are turned into sentences.
	Numbers bool `json:"numbers,omitempty"` // Whether some of the words are replaced with numbers.
	Focus   int  `json:"focus,omitempty"`   // Percentage of the words of adaptive tests with the pairs practiced.
}

func (k Kind) String() string {
	if k.Mode == "" {
		return "unknown"
	}

	name := k.Mode
	if k.Source != "" {
		name += " " + k.Source
	}

	return strings.Join(append([]string{name}, k.Variant()...), ", ")
}

// Variant describes how the text of the tests is made.
func (k Kind) Variant() []string {
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

// MadeOfWords reports whether the tests are made of random words, which
// the options of word tests (-n, -g, -punct, -numbers) shape.
func (k Kind) MadeOfWords() bool {
	return k.Mode == ModeWords || k.Mode == ModeMistakes || k.Mode == ModeAdaptive
}

// Result is the outcome of a completed test, as saved.
type Result struct {
	Wpm       int     `json:"wpm"`
	Accuracy  float64 `json:"accuracy"`
	Timestamp int64   `json:"timestamp"`

	//The fields below are empty for results saved by older versions.
	Duration float64 `json:"duration,omitempty"` //Seconds spent typing.
	Kind         //Saved as fields of the result.
}

// String describes r, rather than only its kind as the String method of the
// embedded Kind would.
func (r Result) String() string {
	return fmt.Sprintf("%d wpm, %.2f%% accuracy, %.1fs at %d (%s)", r.Wpm, r.Accuracy, r.Duration, r.Timestamp, r.Kind)
}
