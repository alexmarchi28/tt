package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSavePersistedResult(t *testing.T) {
	useTempStorage(t)

	if _, err := readPersistedResults(); !os.IsNotExist(err) {
		t.Fatalf("reading missing results: err = %v, want a not-exist error", err)
	}

	savePersistedResult(60, 95.5, 1000)
	savePersistedResult(72, 100, 2000)

	got, err := readPersistedResults()
	if err != nil {
		t.Fatal(err)
	}

	want := []persistedResult{
		{Wpm: 60, Accuracy: 95.5, Timestamp: 1000},
		{Wpm: 72, Accuracy: 100, Timestamp: 2000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored results = %+v, want %+v", got, want)
	}
}

func TestSaveMistakes(t *testing.T) {
	useTempStorage(t)

	saveMistakes([]mistake{{"alpha", "alpah"}})
	saveMistakes(nil)
	saveMistakes([]mistake{{"beta", "bteta"}, {"gamma", "gama_"}})

	var got []mistake
	if err := readValue(mistakeDB, &got); err != nil {
		t.Fatal(err)
	}

	want := []mistake{{"alpha", "alpah"}, {"beta", "bteta"}, {"gamma", "gama_"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored mistakes = %v, want %v", got, want)
	}
}

func TestRememberedTheme(t *testing.T) {
	useTempStorage(t)

	if got := loadRememberedTheme(false); got != "" {
		t.Errorf("without a config file got %q, want no theme", got)
	}

	tests := map[string]string{
		"gruvbox":                 "gruvbox",
		"  nord \n":               "nord",
		"":                        "",
		"not-a-theme-that-exists": "",
	}

	for saved, want := range tests {
		if err := writeAppConfig(appConfig{Theme: saved}); err != nil {
			t.Fatal(err)
		}

		if got := loadRememberedTheme(false); got != want {
			t.Errorf("with saved theme %q got %q, want %q", saved, got, want)
		}
	}

	if err := writeAppConfig(appConfig{Theme: "dracula"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := readAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "dracula" {
		t.Errorf("readAppConfig().Theme = %q", cfg.Theme)
	}
}

func TestSavePersistedResultWithoutKeystrokes(t *testing.T) {
	useTempStorage(t)

	savePersistedResult(0, calcAccuracy(0, 0), 1000)

	got, err := readPersistedResults()
	if err != nil {
		t.Fatal(err)
	}
	if want := []persistedResult{{Wpm: 0, Accuracy: 0, Timestamp: 1000}}; !reflect.DeepEqual(got, want) {
		t.Errorf("stored results = %+v, want %+v", got, want)
	}
}

func TestWriteValueCreatesDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "value.json")

	if err := writeValue(path, map[string]int{"x": 1}); err != nil {
		t.Fatal(err)
	}

	var got map[string]int
	if err := readValue(path, &got); err != nil || got["x"] != 1 {
		t.Errorf("read back %v (%v), want x = 1", got, err)
	}
}
