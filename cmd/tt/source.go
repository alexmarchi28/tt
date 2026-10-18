package main

import (
	"fmt"
	"io"
	"math/rand"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/textgen"
)

// generatorStore gives the generators the saved data they make tests from.
type generatorStore interface {
	mistakeStore
	keyStatsStore
	paragraphStore
}

// textSources are what the generators make the text of tests from.
type textSources struct {
	store     generatorStore
	resources resourceReader
	rng       *rand.Rand
	stdin     io.Reader // Typed in mode text.
}

// newTestSource returns the source of the tests described by s.
func newTestSource(s sourceSettings, src textSources) (textgen.Source, error) {
	res, rng, words := src.resources, src.rng, s.words

	switch s.kind.Mode {
	case history.ModeAdaptive:
		tests, err := textgen.NewAdaptive(rng, res, s.path, words, s.focus, src.store)
		if err != nil {
			return nil, err
		}

		return tests, nil
	case history.ModeWords:
		return textgen.Words(rng, res, s.path, words)
	case history.ModeQuotes:
		return textgen.Quotes(rng, res, s.path)
	case history.ModeMistakes:
		return textgen.Mistakes(rng, words, src.store)
	case history.ModeText:
		b, err := io.ReadAll(src.stdin)
		if err != nil {
			return nil, err
		}

		return textgen.Text(b, s.raw, s.multi)
	case history.ModeFile:
		tests, err := textgen.NewFile(s.path, s.startParagraph, src.store)
		if err != nil {
			return nil, err
		}

		return tests, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", s.kind.Mode)
	}
}
