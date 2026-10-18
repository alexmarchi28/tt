package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var fileStateDB string
var mistakeDB string
var resultsDB string
var keyStatsDB string
var appConfigPath string

type appConfig struct {
	Theme string `json:"theme,omitempty"`
}

type persistedResult struct {
	Wpm       int     `json:"wpm"`
	Accuracy  float64 `json:"accuracy"`
	Timestamp int64   `json:"timestamp"`

	//The fields below are empty for results saved by older versions.
	Mode     string  `json:"mode,omitempty"`     //See testKind.
	Source   string  `json:"source,omitempty"`   //See testKind.
	Duration float64 `json:"duration,omitempty"` //Seconds spent typing.
	Seconds  int     `json:"seconds,omitempty"`  //See testKind.
	Words    int     `json:"words,omitempty"`    //See testKind.
	Punct    bool    `json:"punct,omitempty"`    //See testKind.
	Numbers  bool    `json:"numbers,omitempty"`  //See testKind.
	Focus    int     `json:"focus,omitempty"`    //See testKind.
}

func (r persistedResult) kind() testKind {
	return testKind{
		mode:    r.Mode,
		source:  r.Source,
		seconds: r.Seconds,
		words:   r.Words,
		punct:   r.Punct,
		numbers: r.Numbers,
		focus:   r.Focus,
	}
}

// withKind returns r as the result of the given kind of test.
func (r persistedResult) withKind(k testKind) persistedResult {
	r.Mode, r.Source = k.mode, k.source
	r.Seconds, r.Words, r.Punct, r.Numbers, r.Focus = k.seconds, k.words, k.punct, k.numbers, k.focus

	return r
}

func init() {
	//$HOME on unix, %USERPROFILE% on windows.
	home, err := os.UserHomeDir()
	if err != nil {
		die("could not resolve the home directory")
	}

	data, ok := os.LookupEnv("XDG_DATA_HOME")
	if ok {
		data = filepath.Join(data, "/tt")
	} else {
		data = filepath.Join(home, "/.local/share/tt")
	}

	config := filepath.Join(home, ".config", "tt")

	fileStateDB = filepath.Join(data, ".db")
	mistakeDB = filepath.Join(data, ".errors")
	resultsDB = filepath.Join(data, ".results")
	keyStatsDB = filepath.Join(data, ".keys")
	appConfigPath = filepath.Join(config, "config.json")
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
	defer os.Remove(f.Name()) //Fails harmlessly once the file is renamed.

	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), path)
}

func readAppConfig() (appConfig, error) {
	var cfg appConfig
	err := readValue(appConfigPath, &cfg)
	return cfg, err
}

func writeAppConfig(cfg appConfig) error {
	return writeValue(appConfigPath, cfg)
}

// readPersistedResults returns the saved results, none if there are none yet.
func readPersistedResults() ([]persistedResult, error) {
	var stored []persistedResult
	if err := readValue(resultsDB, &stored); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the saved results in %s: %v, fix or remove the file", resultsDB, err)
	}

	return stored, nil
}

func writePersistedResults(stored []persistedResult) error {
	return writeValue(resultsDB, stored)
}

// readMistakes returns the recorded mistakes, none if there are none yet.
func readMistakes() ([]mistake, error) {
	var mistakes []mistake
	if err := readValue(mistakeDB, &mistakes); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read the recorded mistakes in %s: %v, fix or remove the file", mistakeDB, err)
	}

	return mistakes, nil
}

// saveMistakes adds mistakes to the recorded ones. A file which can't be read
// is left as is rather than replaced, which would lose its mistakes.
func saveMistakes(mistakes []mistake) error {
	db, err := readMistakes()
	if err != nil {
		return err
	}

	return writeValue(mistakeDB, append(db, mistakes...))
}

// savePersistedResult adds r to the saved results and returns the ones saved
// before it.
func savePersistedResult(r persistedResult) []persistedResult {
	stored, err := readPersistedResults()
	if err != nil {
		die("%v", err)
	}

	if err := writePersistedResults(append(stored, r)); err != nil {
		die("could not save the result: %v", err)
	}

	return stored
}

// loadKeyStats returns the saved key statistics. They are only an aid which
// typing rebuilds, so they start over if they can't be read.
func loadKeyStats() keyStats {
	ks := newKeyStats()
	if err := readValue(keyStatsDB, &ks); err != nil || ks.Chars == nil || ks.Bigrams == nil {
		return newKeyStats()
	}

	return ks
}

// saveKeyStats adds the keystrokes of a test to the saved key statistics.
func saveKeyStats(strokes []keystroke) {
	ks := loadKeyStats()
	ks.add(strokes)

	if err := writeValue(keyStatsDB, ks); err != nil {
		panic(err)
	}
}
