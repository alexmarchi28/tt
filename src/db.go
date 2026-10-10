package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var FILE_STATE_DB string
var MISTAKE_DB string
var RESULTS_DB string
var APP_CONFIG_PATH string

type appConfig struct {
	Theme string `json:"theme,omitempty"`
}

type persistedResult struct {
	Wpm       int     `json:"wpm"`
	Accuracy  float64 `json:"accuracy"`
	Timestamp int64   `json:"timestamp"`
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

	FILE_STATE_DB = filepath.Join(data, ".db")
	MISTAKE_DB = filepath.Join(data, ".errors")
	RESULTS_DB = filepath.Join(data, ".results")
	APP_CONFIG_PATH = filepath.Join(config, "config.json")
}

func readValue(path string, o interface{}) error {
	b, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	return json.Unmarshal(b, o)
}

// writeValue stores o as JSON in path, creating its directory if needed.
func writeValue(path string, o interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	return os.WriteFile(path, b, 0600)
}

func readAppConfig() (appConfig, error) {
	var cfg appConfig
	err := readValue(APP_CONFIG_PATH, &cfg)
	return cfg, err
}

func writeAppConfig(cfg appConfig) error {
	return writeValue(APP_CONFIG_PATH, cfg)
}

func readPersistedResults() ([]persistedResult, error) {
	var stored []persistedResult
	err := readValue(RESULTS_DB, &stored)
	return stored, err
}

func writePersistedResults(stored []persistedResult) error {
	return writeValue(RESULTS_DB, stored)
}

func saveMistakes(mistakes []mistake) {
	var db []mistake

	if err := readValue(MISTAKE_DB, &db); err != nil {
		db = nil
	}

	db = append(db, mistakes...)
	if err := writeValue(MISTAKE_DB, db); err != nil {
		panic(err)
	}
}

func savePersistedResult(wpm int, accuracy float64, timestamp int64) {
	stored, err := readPersistedResults()
	if err != nil {
		if os.IsNotExist(err) {
			stored = nil
		} else {
			panic(err)
		}
	}

	stored = append(stored, persistedResult{
		Wpm:       wpm,
		Accuracy:  accuracy,
		Timestamp: timestamp,
	})

	if err := writePersistedResults(stored); err != nil {
		panic(err)
	}
}
