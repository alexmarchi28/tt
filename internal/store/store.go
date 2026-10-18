// Package store saves the data of tt between its runs as JSON files: the
// results, the mistakes, the key statistics, the paragraph reached in each
// file and the configuration.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

// Config is the configuration remembered between runs.
type Config struct {
	Theme string `json:"theme,omitempty"`
}

// The files of jsonStore in its data directory.
const (
	paragraphsFile = ".db"
	mistakesFile   = ".errors"
	resultsFile    = ".results"
	keyStatsFile   = ".keys"
)

// JSON saves the data as JSON files, in the data directory of the XDG
// specification and the configuration in ~/.config/tt.
type JSON struct {
	dataDir, configDir string
}

// New returns the store of the user whose home directory is home.
// getenv gives the environment variables.
func New(getenv func(string) string, home string) *JSON {
	data := filepath.Join(home, ".local", "share", "tt")
	if xdg := getenv("XDG_DATA_HOME"); xdg != "" {
		data = filepath.Join(xdg, "tt")
	}

	return &JSON{dataDir: data, configDir: filepath.Join(home, ".config", "tt")}
}

// path returns the path of the given file of the data directory.
func (s *JSON) path(file string) string {
	return filepath.Join(s.dataDir, file)
}

func (s *JSON) configPath() string {
	return filepath.Join(s.configDir, "config.json")
}

func (s *JSON) Results() ([]history.Result, error) {
	var stored []history.Result
	if err := readValue(s.path(resultsFile), &stored); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the saved results in %s: %v, fix or remove the file", s.path(resultsFile), err)
	}

	return stored, nil
}

func (s *JSON) AddResult(r history.Result) ([]history.Result, error) {
	stored, err := s.Results()
	if err != nil {
		return nil, err
	}

	if err := writeValue(s.path(resultsFile), append(stored, r)); err != nil {
		return nil, fmt.Errorf("could not save the result: %v", err)
	}

	return stored, nil
}

func (s *JSON) Mistakes() ([]typing.Mistake, error) {
	var mistakes []typing.Mistake
	if err := readValue(s.path(mistakesFile), &mistakes); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the recorded mistakes in %s: %v, fix or remove the file", s.path(mistakesFile), err)
	}

	return mistakes, nil
}

// AddMistakes adds mistakes to the recorded ones. A file which can't be read
// is left as is rather than replaced, which would lose its mistakes.
func (s *JSON) AddMistakes(mistakes []typing.Mistake) error {
	recorded, err := s.Mistakes()
	if err != nil {
		return err
	}

	if err := writeValue(s.path(mistakesFile), append(recorded, mistakes...)); err != nil {
		return fmt.Errorf("could not save the mistakes: %v", err)
	}

	return nil
}

func (s *JSON) KeyStats() keystats.Stats {
	ks := keystats.New()
	if err := readValue(s.path(keyStatsFile), &ks); err != nil || ks.Chars == nil || ks.Bigrams == nil {
		return keystats.New()
	}

	return ks
}

func (s *JSON) AddKeystrokes(strokes []typing.Keystroke) error {
	ks := s.KeyStats()
	ks.Add(strokes)

	if err := writeValue(s.path(keyStatsFile), ks); err != nil {
		return fmt.Errorf("could not save the key statistics: %v", err)
	}

	return nil
}

// paragraphs returns the paragraph reached in each file. They are only an aid,
// so they start over if they can't be read.
func (s *JSON) paragraphs() map[string]int {
	var p map[string]int
	if err := readValue(s.path(paragraphsFile), &p); err != nil || p == nil {
		return map[string]int{}
	}

	return p
}

func (s *JSON) Paragraph(path string) int {
	return s.paragraphs()[path]
}

func (s *JSON) SetParagraph(path string, paragraph int) error {
	p := s.paragraphs()
	p[path] = paragraph

	return writeValue(s.path(paragraphsFile), p)
}

func (s *JSON) Config() (Config, error) {
	var cfg Config
	err := readValue(s.configPath(), &cfg)
	return cfg, err
}

func (s *JSON) SaveConfig(cfg Config) error {
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
