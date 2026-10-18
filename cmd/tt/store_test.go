package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

// memStore keeps the saved data in memory. Every access fails with err if it
// is set.
type memStore struct {
	results    []persistedResult
	mistakes   []typing.Mistake
	keys       keystats.Stats
	paragraphs map[string]int
	config     appConfig
	err        error
}

func (s *memStore) Results() ([]persistedResult, error) { return s.results, s.err }

func (s *memStore) AddResult(r persistedResult) ([]persistedResult, error) {
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

func (s *memStore) Config() (appConfig, error) { return s.config, s.err }

func (s *memStore) SaveConfig(cfg appConfig) error {
	if s.err == nil {
		s.config = cfg
	}
	return s.err
}

// newTestStore returns a store of its own to the calling test, in an empty
// home directory.
func newTestStore(t *testing.T) *jsonStore {
	t.Helper()

	return newJSONStore(func(string) string { return "" }, t.TempDir())
}

func TestNewJSONStore(t *testing.T) {
	t.Parallel()

	home := "/home/typist"
	getenv := func(xdg string) func(string) string {
		return func(key string) string {
			if key == "XDG_DATA_HOME" {
				return xdg
			}
			return ""
		}
	}

	s := newJSONStore(getenv(""), home)
	if s.dataDir != filepath.Join(home, ".local", "share", "tt") || s.configDir != filepath.Join(home, ".config", "tt") {
		t.Errorf("store = %+v, want the default directories", s)
	}

	if s := newJSONStore(getenv("/xdg"), home); s.dataDir != filepath.Join("/xdg", "tt") {
		t.Errorf("store = %+v, want the data in XDG_DATA_HOME", s)
	}
}

func TestStoreResults(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if got, err := s.Results(); got != nil || err != nil {
		t.Fatalf("reading missing results: %+v, %v, want none", got, err)
	}

	first := persistedResult{Wpm: 60, Accuracy: 95.5, Timestamp: 1000}
	if previous, err := s.AddResult(first); previous != nil || err != nil {
		t.Errorf("results saved before the first one: %+v, %v", previous, err)
	}

	second := persistedResult{Wpm: 72, Accuracy: 100, Timestamp: 2000, Duration: 12.5, testKind: testKind{Mode: "quotes", Source: "en", Seconds: 30}}
	if previous, err := s.AddResult(second); err != nil || !reflect.DeepEqual(previous, []persistedResult{first}) {
		t.Errorf("results saved before the second one: %+v, %v, want the first one", previous, err)
	}

	got, err := s.Results()
	if want := []persistedResult{first, second}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("stored results = %+v, %v, want %+v", got, err, want)
	}
}

func TestStoreResultsOfOlderVersions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	writeFile(t, s.path(resultsFile), `[{"wpm":60,"accuracy":95,"timestamp":1000}]`)

	got, err := s.Results()
	if want := []persistedResult{{Wpm: 60, Accuracy: 95, Timestamp: 1000}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Results() = %+v, %v, want %+v", got, err, want)
	}
}

// Corrupt results are reported rather than replaced, which would lose them.
func TestStoreCorruptResults(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	path := s.path(resultsFile)
	writeFile(t, path, `[{"wpm":60,"accur`)

	want := "could not read the saved results in " + path + ": unexpected end of JSON input, fix or remove the file"
	if _, err := s.Results(); err == nil || err.Error() != want {
		t.Errorf("Results() = %v, want the error %q", err, want)
	}
	if _, err := s.AddResult(persistedResult{Wpm: 60}); err == nil || err.Error() != want {
		t.Errorf("AddResult() = %v, want the error %q", err, want)
	}
	if b, _ := os.ReadFile(path); string(b) != `[{"wpm":60,"accur` {
		t.Errorf("the results file was changed to %q", b)
	}
}

func TestStoreMistakes(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if got, err := s.Mistakes(); got != nil || err != nil {
		t.Fatalf("reading missing mistakes: %+v, %v, want none", got, err)
	}

	for _, m := range [][]typing.Mistake{{{Word: "alpha", Typed: "alpah"}}, nil, {{Word: "beta", Typed: "bteta"}, {Word: "gamma", Typed: "gama_"}}} {
		if err := s.AddMistakes(m); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Mistakes()
	if want := []typing.Mistake{{Word: "alpha", Typed: "alpah"}, {Word: "beta", Typed: "bteta"}, {Word: "gamma", Typed: "gama_"}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("stored mistakes = %v, %v, want %v", got, err, want)
	}
}

// A corrupt file of mistakes is reported rather than replaced, which would
// lose the mistakes recorded so far.
func TestStoreCorruptMistakes(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	path := s.path(mistakesFile)
	writeFile(t, path, `[{"word":"old","typed":"odl"},`)

	want := "could not read the recorded mistakes in " + path + ": unexpected end of JSON input, fix or remove the file"
	if err := s.AddMistakes([]typing.Mistake{{Word: "new", Typed: "nwe"}}); err == nil || err.Error() != want {
		t.Errorf("AddMistakes over a corrupt file: %v, want the error %q", err, want)
	}
	if b, _ := os.ReadFile(path); string(b) != `[{"word":"old","typed":"odl"},` {
		t.Errorf("the mistakes file was changed to %q", b)
	}
}

func TestStoreKeyStats(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if got := s.KeyStats(); len(got.Chars) != 0 || len(got.Bigrams) != 0 {
		t.Errorf("KeyStats() without a file = %+v, want empty statistics", got)
	}

	for _, strokes := range [][]typing.Keystroke{{{Expected: 'a', Typed: 'a', At: 0}, {Expected: 'b', Typed: 'x', At: 100 * time.Millisecond}}, {{Expected: 'a', Typed: 'a', At: 0}}} {
		if err := s.AddKeystrokes(strokes); err != nil {
			t.Fatal(err)
		}
	}

	got := s.KeyStats()
	if got.Chars["a"].Count != 2 || got.Chars["b"].ErrorRate != 1 || got.Bigrams["ab"].Count != 1 {
		t.Errorf("saved statistics = %+v", got)
	}
}

func TestStoreKeyStatsOverCorruptFile(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	writeFile(t, s.path(keyStatsFile), "not json")

	// The statistics are only an aid, they start over rather than stopping tt.
	if err := s.AddKeystrokes([]typing.Keystroke{{Expected: 'a', Typed: 'a', At: 0}}); err != nil {
		t.Fatal(err)
	}
	if got := s.KeyStats(); got.Chars["a"].Count != 1 {
		t.Errorf("statistics = %+v, want them to start over", got)
	}
}

func TestStoreParagraphs(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if got := s.Paragraph("/a"); got != 0 {
		t.Errorf("Paragraph() of a new file = %d, want 0", got)
	}

	for path, n := range map[string]int{"/a": 3, "/b": 1} {
		if err := s.SetParagraph(path, n); err != nil {
			t.Fatal(err)
		}
	}

	if a, b := s.Paragraph("/a"), s.Paragraph("/b"); a != 3 || b != 1 {
		t.Errorf("paragraphs = %d, %d, want 3, 1", a, b)
	}

	// The positions are only an aid, they start over if they can't be read.
	writeFile(t, s.path(paragraphsFile), "not json")
	if got := s.Paragraph("/a"); got != 0 {
		t.Errorf("Paragraph() with a corrupt file = %d, want 0", got)
	}
}

func TestStoreConfig(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if _, err := s.Config(); !os.IsNotExist(err) {
		t.Errorf("Config() without a file: %v, want it not to exist", err)
	}

	if err := s.SaveConfig(appConfig{Theme: "dracula"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err := s.Config(); err != nil || cfg.Theme != "dracula" {
		t.Errorf("Config() = %+v, %v, want the theme saved", cfg, err)
	}
}

func TestStoreReadsWithoutCreatingDirectories(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	s := newJSONStore(func(string) string { return "" }, home)

	s.Results()
	s.Mistakes()
	s.KeyStats()
	s.Paragraph("/a")
	s.Config()

	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("unexpected entries in the home directory: %v", entries)
	}
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
		if err := s.SaveConfig(appConfig{Theme: saved}); err != nil {
			t.Fatal(err)
		}

		if got := loadRememberedTheme(s, res, false); got != want {
			t.Errorf("with saved theme %q got %q, want %q", saved, got, want)
		}
	}
}

// The saved results must remain readable by other versions of tt.
func TestPersistedResultFormat(t *testing.T) {
	t.Parallel()

	r := persistedResult{Wpm: 60, Accuracy: 95.5, Timestamp: 1000, Duration: 30, testKind: testKind{Mode: "adaptive", Source: "1000en", Seconds: 15, Words: 50, Punct: true, Numbers: true, Focus: 40}}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	// The order of the fields doesn't matter.
	want := `{"wpm":60,"accuracy":95.5,"timestamp":1000,"mode":"adaptive","source":"1000en","duration":30,"seconds":15,"words":50,"punct":true,"numbers":true,"focus":40}`
	var gotFields, wantFields map[string]interface{}
	if err := json.Unmarshal(b, &gotFields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantFields); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Errorf("saved as\n%s\nwant\n%s", b, want)
	}

	// Results saved by older versions have no kind or variant.
	var old persistedResult
	if err := json.Unmarshal([]byte(`{"wpm":60,"accuracy":95.5,"timestamp":1000}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.testKind != (testKind{}) {
		t.Errorf("kind of an old result = %+v, want none", old.testKind)
	}
}

// Results are printed whole, not only their kind as the embedded testKind
// would.
func TestPersistedResultString(t *testing.T) {
	t.Parallel()

	r := persistedResult{Wpm: 60, Accuracy: 95.5, Timestamp: 1000, Duration: 30, testKind: testKind{Mode: "quotes", Source: "en"}}

	if got, want := fmt.Sprint(r), "60 wpm, 95.50% accuracy, 30.0s at 1000 (quotes en)"; got != want {
		t.Errorf("printed as %q, want %q", got, want)
	}
}

func TestWriteValueCreatesDirectories(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "a", "b", "value.json")

	if err := writeValue(path, map[string]int{"x": 1}); err != nil {
		t.Fatal(err)
	}

	var got map[string]int
	if err := readValue(path, &got); err != nil || got["x"] != 1 {
		t.Errorf("read back %v (%v), want x = 1", got, err)
	}
}

func TestWriteValueReplacesFilesAtomically(t *testing.T) {
	t.Parallel()

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
