package main

import (
	"testing"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/store"
	"github.com/alexmarchi28/tt/internal/typing"
)

// memStore keeps the saved data in memory. Every access fails with err if it
// is set.
type memStore struct {
	results    []history.Result
	mistakes   []typing.Mistake
	keys       keystats.Stats
	paragraphs map[string]int
	config     store.Config
	err        error
}

func (s *memStore) Results() ([]history.Result, error) { return s.results, s.err }

func (s *memStore) AddResult(r history.Result) ([]history.Result, error) {
	if s.err != nil {
		return nil, s.err
	}

	previous := s.results
	s.results = append(s.results[:len(s.results):len(s.results)], r)
	return previous, nil
}

func (s *memStore) Mistakes() ([]typing.Mistake, error) { return s.mistakes, s.err }

func (s *memStore) AddMistakes(mistakes []typing.Mistake) error {
	if s.err == nil {
		s.mistakes = append(s.mistakes, mistakes...)
	}
	return s.err
}

func (s *memStore) KeyStats() keystats.Stats {
	if s.keys.Chars == nil {
		return keystats.New()
	}
	return s.keys
}

func (s *memStore) AddKeystrokes(strokes []typing.Keystroke) error {
	if s.err == nil {
		s.keys = s.KeyStats()
		s.keys.Add(strokes)
	}
	return s.err
}

func (s *memStore) Paragraph(path string) int { return s.paragraphs[path] }

func (s *memStore) SetParagraph(path string, paragraph int) error {
	if s.err == nil {
		if s.paragraphs == nil {
			s.paragraphs = map[string]int{}
		}
		s.paragraphs[path] = paragraph
	}
	return s.err
}

func (s *memStore) Config() (store.Config, error) { return s.config, s.err }

func (s *memStore) SaveConfig(cfg store.Config) error {
	if s.err == nil {
		s.config = cfg
	}
	return s.err
}

// newTestStore returns a store of its own to the calling test, in an empty
// home directory.
func newTestStore(t *testing.T) *store.JSON {
	t.Helper()

	return store.New(func(string) string { return "" }, t.TempDir())
}

func TestRememberedTheme(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	res, _ := newTestResources(t)

	if got := loadRememberedTheme(s, res, false); got != "" {
		t.Errorf("without a config file got %q, want no theme", got)
	}

	tests := map[string]string{
		"gruvbox":                 "gruvbox",
		"  nord \n":               "nord",
		"":                        "",
		"not-a-theme-that-exists": "",
	}

	for saved, want := range tests {
		if err := s.SaveConfig(store.Config{Theme: saved}); err != nil {
			t.Fatal(err)
		}

		if got := loadRememberedTheme(s, res, false); got != want {
			t.Errorf("with saved theme %q got %q, want %q", saved, got, want)
		}
	}
}
