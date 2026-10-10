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
}

func (r persistedResult) kind() testKind {
	return testKind{r.Mode, r.Source}
}

func init() {
	var ok bool
	var data string
	var home string

	if home, ok = os.LookupEnv("HOME"); !ok {
		die("Could not resolve home directory.")
	}

	if data, ok = os.LookupEnv("XDG_DATA_HOME"); ok {
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

// unreadableResults describes the error of reading the saved results.
func unreadableResults(err error) error {
	return fmt.Errorf("Could not read the saved results in %s: %v. Fix or remove the file.", resultsDB, err)
}

func readPersistedResults() ([]persistedResult, error) {
	var stored []persistedResult
	err := readValue(resultsDB, &stored)
	return stored, err
}

func writePersistedResults(stored []persistedResult) error {
	return writeValue(resultsDB, stored)
}

func saveMistakes(mistakes []mistake) {
	var db []mistake

	if err := readValue(mistakeDB, &db); err != nil {
		db = nil
	}

	db = append(db, mistakes...)
	if err := writeValue(mistakeDB, db); err != nil {
		panic(err)
	}
}

// savePersistedResult adds r to the saved results and returns the ones saved
// before it.
func savePersistedResult(r persistedResult) []persistedResult {
	stored, err := readPersistedResults()
	if err != nil && !os.IsNotExist(err) {
		die("%v", unreadableResults(err))
	}

	if err := writePersistedResults(append(stored, r)); err != nil {
		die("Could not save the result: %v", err)
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
