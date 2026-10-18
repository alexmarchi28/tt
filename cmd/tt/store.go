package main

import (
	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/store"
	"github.com/alexmarchi28/tt/internal/typing"
)

// The interfaces below give access to the data saved between runs of tt,
// each user of the data depending only on what it needs.

type resultStore interface {
	// Results returns the saved results, none if there are none yet.
	Results() ([]history.Result, error)
	// AddResult saves r and returns the results saved before it.
	AddResult(r history.Result) ([]history.Result, error)
}

type mistakeStore interface {
	// Mistakes returns the recorded mistakes, none if there are none yet.
	Mistakes() ([]typing.Mistake, error)
	AddMistakes(mistakes []typing.Mistake) error
}

type keyStatsStore interface {
	// KeyStats returns the saved key statistics. They are only an aid which
	// typing rebuilds, so they start over if they can't be read.
	KeyStats() keystats.Stats
	// AddKeystrokes adds the keystrokes of a test to the key statistics.
	AddKeystrokes(strokes []typing.Keystroke) error
}

type paragraphStore interface {
	// Paragraph returns the paragraph reached in the file at path, 0 for a
	// file never typed.
	Paragraph(path string) int
	SetParagraph(path string, paragraph int) error
}

type configStore interface {
	Config() (store.Config, error)
	SaveConfig(cfg store.Config) error
}

// historyStore gives access to what was typed so far.
type historyStore interface {
	resultStore
	mistakeStore
}

// dataStore gives access to all the saved data.
type dataStore interface {
	resultStore
	mistakeStore
	keyStatsStore
	paragraphStore
	configStore
}
