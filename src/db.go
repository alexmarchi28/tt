package main

import (
	"encoding/json"
	"io/ioutil"
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

	os.MkdirAll(data, 0700)
	os.MkdirAll(config, 0700)

	FILE_STATE_DB = filepath.Join(data, ".db")
	MISTAKE_DB = filepath.Join(data, ".errors")
	RESULTS_DB = filepath.Join(data, ".results")
	APP_CONFIG_PATH = filepath.Join(config, "config.json")
}

func readValue(path string, o interface{}) error {
	b, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	return json.Unmarshal(b, o)
}

func writeValue(path string, o interface{}) {
	b, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(path, b, 0600)
	if err != nil {
		panic(err)
	}
}

func readAppConfig() (appConfig, error) {
	var cfg appConfig
	err := readValue(APP_CONFIG_PATH, &cfg)
	return cfg, err
}

func writeAppConfig(cfg appConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(APP_CONFIG_PATH, b, 0600)
}

func readPersistedResults() ([]persistedResult, error) {
	var stored []persistedResult
	err := readValue(RESULTS_DB, &stored)
	return stored, err
}

func writePersistedResults(stored []persistedResult) error {
	b, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(RESULTS_DB, b, 0600)
}
