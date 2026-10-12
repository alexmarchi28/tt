package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSavePersistedResult(t *testing.T) {
	useTempStorage(t)

	if got, err := readPersistedResults(); got != nil || err != nil {
		t.Fatalf("reading missing results: %+v, %v, want none", got, err)
	}

	first := persistedResult{Wpm: 60, Accuracy: 95.5, Timestamp: 1000}
	if previous := savePersistedResult(first); previous != nil {
		t.Errorf("results saved before the first one: %+v", previous)
	}

	second := persistedResult{Wpm: 72, Accuracy: 100, Timestamp: 2000, Mode: "quotes", Source: "en", Duration: 12.5, Seconds: 30}
	if previous := savePersistedResult(second); !reflect.DeepEqual(previous, []persistedResult{first}) {
		t.Errorf("results saved before the second one: %+v, want the first one", previous)
	}

	got, err := readPersistedResults()
	if err != nil {
		t.Fatal(err)
	}

	want := []persistedResult{
		{Wpm: 60, Accuracy: 95.5, Timestamp: 1000},
		{Wpm: 72, Accuracy: 100, Timestamp: 2000, Mode: "quotes", Source: "en", Duration: 12.5, Seconds: 30},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored results = %+v, want %+v", got, want)
	}
}

func TestPersistedResultKind(t *testing.T) {
	kinds := []testKind{
		{},
		{mode: "quotes", source: "en"},
		{mode: "words", source: "1000en", words: 50, punct: true},
		{mode: "mistakes", seconds: 60, numbers: true},
	}

	for _, kind := range kinds {
		r := persistedResult{Wpm: 60, Accuracy: 95, Timestamp: 1000, Duration: 30}.withKind(kind)
		if got := r.kind(); got != kind {
			t.Errorf("kind() = %+v, want %+v", got, kind)
		}
		if r.Wpm != 60 || r.Accuracy != 95 || r.Timestamp != 1000 || r.Duration != 30 {
			t.Errorf("withKind() altered the result: %+v", r)
		}
	}
}

// The saved results must remain readable by other versions of tt.
func TestPersistedResultFormat(t *testing.T) {
	r := persistedResult{Wpm: 60, Accuracy: 95.5, Timestamp: 1000, Duration: 30}.
		withKind(testKind{mode: "words", source: "1000en", seconds: 15, words: 50, punct: true, numbers: true})

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"wpm":60,"accuracy":95.5,"timestamp":1000,"mode":"words","source":"1000en","duration":30,"seconds":15,"words":50,"punct":true,"numbers":true}`
	if string(b) != want {
		t.Errorf("saved as\n%s\nwant\n%s", b, want)
	}

	// Results saved by older versions have no kind or variant.
	var old persistedResult
	if err := json.Unmarshal([]byte(`{"wpm":60,"accuracy":95.5,"timestamp":1000}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.kind() != (testKind{}) {
		t.Errorf("kind of an old result = %+v, want none", old.kind())
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

	savePersistedResult(persistedResult{Wpm: 0, Accuracy: calcAccuracy(0, 0), Timestamp: 1000})

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

func TestReadResultsOfOlderVersions(t *testing.T) {
	useTempStorage(t)

	if err := os.WriteFile(resultsDB, []byte(`[{"wpm":60,"accuracy":95,"timestamp":1000}]`), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readPersistedResults()
	if want := []persistedResult{{Wpm: 60, Accuracy: 95, Timestamp: 1000}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("readPersistedResults() = %+v, %v, want %+v", got, err, want)
	}
}

func TestWriteValueReplacesFilesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "value.json")

	for _, v := range []int{1, 2} {
		if err := writeValue(path, v); err != nil {
			t.Fatal(err)
		}
	}

	var got int
	if err := readValue(path, &got); err != nil || got != 2 {
		t.Errorf("read back %v (%v), want 2", got, err)
	}

	// The value is written to a temporary file then moved in place, so that an
	// interrupted write never leaves a truncated file behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("files left in the directory: %v", entries)
	}

	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Errorf("file mode = %v (%v), want 0600", info.Mode().Perm(), err)
	}
}
