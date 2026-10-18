package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

type appConfig struct {
	Theme string `json:"theme,omitempty"`
}

type persistedResult struct {
	Wpm       int     `json:"wpm"`
	Accuracy  float64 `json:"accuracy"`
	Timestamp int64   `json:"timestamp"`

	//The fields below are empty for results saved by older versions.
	Duration float64 `json:"duration,omitempty"` //Seconds spent typing.
	testKind         //Saved as fields of the result.
}

// String describes r, rather than only its kind as the String method of the
// embedded testKind would.
func (r persistedResult) String() string {
	return fmt.Sprintf("%d wpm, %.2f%% accuracy, %.1fs at %d (%s)", r.Wpm, r.Accuracy, r.Duration, r.Timestamp, r.testKind)
}

// The interfaces below give access to the data saved between runs of tt,
// each user of the data depending only on what it needs.

type resultStore interface {
	// Results returns the saved results, none if there are none yet.
	Results() ([]persistedResult, error)
	// AddResult saves r and returns the results saved before it.
	AddResult(r persistedResult) ([]persistedResult, error)
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
	Config() (appConfig, error)
	SaveConfig(cfg appConfig) error
}

// historyStore gives access to what was typed so far.
type historyStore interface {
	resultStore
	mistakeStore
}

// store gives access to all the saved data.
type store interface {
	resultStore
	mistakeStore
	keyStatsStore
	paragraphStore
	configStore
}

// The files of jsonStore in its data directory.
const (
	paragraphsFile = ".db"
	mistakesFile   = ".errors"
	resultsFile    = ".results"
	keyStatsFile   = ".keys"
)

// jsonStore saves the data as JSON files, in the data directory of the XDG
// specification and the configuration in ~/.config/tt.
type jsonStore struct {
	dataDir, configDir string
}

// newJSONStore returns the store of the user whose home directory is home.
// getenv gives the environment variables.
func newJSONStore(getenv func(string) string, home string) *jsonStore {
	data := filepath.Join(home, ".local", "share", "tt")
	if xdg := getenv("XDG_DATA_HOME"); xdg != "" {
		data = filepath.Join(xdg, "tt")
	}

	return &jsonStore{dataDir: data, configDir: filepath.Join(home, ".config", "tt")}
}

// path returns the path of the given file of the data directory.
func (s *jsonStore) path(file string) string {
	return filepath.Join(s.dataDir, file)
}

func (s *jsonStore) configPath() string {
	return filepath.Join(s.configDir, "config.json")
}

func (s *jsonStore) Results() ([]persistedResult, error) {
	var stored []persistedResult
	if err := readValue(s.path(resultsFile), &stored); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the saved results in %s: %v, fix or remove the file", s.path(resultsFile), err)
	}

	return stored, nil
}

func (s *jsonStore) AddResult(r persistedResult) ([]persistedResult, error) {
	stored, err := s.Results()
	if err != nil {
		return nil, err
	}

	if err := writeValue(s.path(resultsFile), append(stored, r)); err != nil {
		return nil, fmt.Errorf("could not save the result: %v", err)
	}

	return stored, nil
}

func (s *jsonStore) Mistakes() ([]typing.Mistake, error) {
	var mistakes []typing.Mistake
	if err := readValue(s.path(mistakesFile), &mistakes); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the recorded mistakes in %s: %v, fix or remove the file", s.path(mistakesFile), err)
	}

	return mistakes, nil
}

// AddMistakes adds mistakes to the recorded ones. A file which can't be read
// is left as is rather than replaced, which would lose its mistakes.
func (s *jsonStore) AddMistakes(mistakes []typing.Mistake) error {
	recorded, err := s.Mistakes()
	if err != nil {
		return err
	}

	if err := writeValue(s.path(mistakesFile), append(recorded, mistakes...)); err != nil {
		return fmt.Errorf("could not save the mistakes: %v", err)
	}

	return nil
}

func (s *jsonStore) KeyStats() keystats.Stats {
	ks := keystats.New()
	if err := readValue(s.path(keyStatsFile), &ks); err != nil || ks.Chars == nil || ks.Bigrams == nil {
		return keystats.New()
	}

	return ks
}

func (s *jsonStore) AddKeystrokes(strokes []typing.Keystroke) error {
	ks := s.KeyStats()
	ks.Add(strokes)

	if err := writeValue(s.path(keyStatsFile), ks); err != nil {
		return fmt.Errorf("could not save the key statistics: %v", err)
	}

	return nil
}

// paragraphs returns the paragraph reached in each file. They are only an aid,
// so they start over if they can't be read.
func (s *jsonStore) paragraphs() map[string]int {
	var p map[string]int
	if err := readValue(s.path(paragraphsFile), &p); err != nil || p == nil {
		return map[string]int{}
	}

	return p
}

func (s *jsonStore) Paragraph(path string) int {
	return s.paragraphs()[path]
}

func (s *jsonStore) SetParagraph(path string, paragraph int) error {
	p := s.paragraphs()
	p[path] = paragraph

	return writeValue(s.path(paragraphsFile), p)
}

func (s *jsonStore) Config() (appConfig, error) {
	var cfg appConfig
	err := readValue(s.configPath(), &cfg)
	return cfg, err
}

func (s *jsonStore) SaveConfig(cfg appConfig) error {
	return writeValue(s.configPath(), cfg)
}

func readValue(path string, o interface{}) error {
	b, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	return json.Unmarshal(b, o)
}

// writeValue stores o as JSON in path, creating its directory if needed. The
// value is written to a temporary file which then replaces path, so that an
// interrupted write never leaves a truncated file behind.
func writeValue(path string, o interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() //Fails harmlessly once the file is renamed.

	if _, err := f.Write(b); err != nil {
		_ = f.Close() //The write error is the one worth reporting.
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), path)
}
