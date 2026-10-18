package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/typing"
)

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

	s := New(getenv(""), home)
	if s.dataDir != filepath.Join(home, ".local", "share", "tt") || s.configDir != filepath.Join(home, ".config", "tt") {
		t.Errorf("store = %+v, want the default directories", s)
	}

	if s := New(getenv("/xdg"), home); s.dataDir != filepath.Join("/xdg", "tt") {
		t.Errorf("store = %+v, want the data in XDG_DATA_HOME", s)
	}
}

func TestStoreResults(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	if got, err := s.Results(); got != nil || err != nil {
		t.Fatalf("reading missing results: %+v, %v, want none", got, err)
	}

	first := history.Result{Wpm: 60, Accuracy: 95.5, Timestamp: 1000}
	if previous, err := s.AddResult(first); previous != nil || err != nil {
		t.Errorf("results saved before the first one: %+v, %v", previous, err)
	}

	second := history.Result{Wpm: 72, Accuracy: 100, Timestamp: 2000, Duration: 12.5, Kind: history.Kind{Mode: "quotes", Source: "en", Seconds: 30}}
	if previous, err := s.AddResult(second); err != nil || !reflect.DeepEqual(previous, []history.Result{first}) {
		t.Errorf("results saved before the second one: %+v, %v, want the first one", previous, err)
	}

	got, err := s.Results()
	if want := []history.Result{first, second}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("stored results = %+v, %v, want %+v", got, err, want)
	}
}

func TestStoreResultsOfOlderVersions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	writeFile(t, s.path(resultsFile), `[{"wpm":60,"accuracy":95,"timestamp":1000}]`)

	got, err := s.Results()
	if want := []history.Result{{Wpm: 60, Accuracy: 95, Timestamp: 1000}}; err != nil || !reflect.DeepEqual(got, want) {
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
	if _, err := s.AddResult(history.Result{Wpm: 60}); err == nil || err.Error() != want {
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

	if err := s.SaveConfig(Config{Theme: "dracula"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err := s.Config(); err != nil || cfg.Theme != "dracula" {
		t.Errorf("Config() = %+v, %v, want the theme saved", cfg, err)
	}
}

func TestStoreReadsWithoutCreatingDirectories(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	s := New(func(string) string { return "" }, home)

	s.Results()
	s.Mistakes()
	s.KeyStats()
	s.Paragraph("/a")
	s.Config()

	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("unexpected entries in the home directory: %v", entries)
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// newTestStore returns a store of its own to the calling test, in an empty
// home directory.
func newTestStore(t *testing.T) *JSON {
	t.Helper()

	return New(func(string) string { return "" }, t.TempDir())
}

// Key statistics which can't be saved are reported as such.
func TestStoreKeyStatsWriteError(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	// A directory can't be replaced by the file of the key statistics.
	if err := os.MkdirAll(s.path(keyStatsFile), 0700); err != nil {
		t.Fatal(err)
	}

	if err := s.AddKeystrokes([]typing.Keystroke{{Expected: 'a', Typed: 'a'}}); err == nil || !strings.HasPrefix(err.Error(), "could not save the key statistics: ") {
		t.Errorf("AddKeystrokes() error %v, want the key statistics not saved", err)
	}
}

func TestWriteValueErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := writeValue(filepath.Join(dir, "v.json"), make(chan int)); err == nil {
		t.Error("a value which isn't JSON was written")
	}

	// A directory can't be replaced by the file, which is left out.
	path := filepath.Join(dir, "taken")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeValue(path, 1); err == nil {
		t.Error("a directory was replaced by the value")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("entries %v, want the temporary file removed", entries)
	}
}
