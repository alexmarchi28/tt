package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

type cliResult struct {
	stdout, stderr string
	code           int
}

// runCLI runs tt with the given arguments, as a user whose home directory is
// home and whose environment variables are given as key=value. Nothing is
// piped to stdin, and the screen can't be used so that a test reaching the
// interactive part fails.
func runCLI(t *testing.T, home string, env []string, args ...string) cliResult {
	t.Helper()

	vars := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		vars[k] = v
	}

	var stdout, stderr bytes.Buffer
	code := run(args, environment{
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
		getenv: func(key string) string { return vars[key] },
		homeDir: func() (string, error) {
			if home == "" {
				return "", errors.New("$HOME is not defined")
			}
			return home, nil
		},
		newScreen: func(bool) (tcell.Screen, error) { return nil, errors.New("no screen in tests") },
	})

	return cliResult{stdout.String(), stderr.String(), code}
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

func TestCLIVersion(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-v")

	// The version itself comes from the build, see TestResolveVersion.
	if got.code != 0 || got.stderr != "" || !strings.HasPrefix(got.stdout, "tt version ") {
		t.Errorf("tt -v = %+v, want the version on stdout", got)
	}
}

func TestCLIHelp(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-help")

	if got.code != 0 || !strings.HasPrefix(got.stdout, "usage: tt [options] [file]\n") {
		t.Errorf("tt -help = %+v, want the usage on stdout", got)
	}
}

func TestCLIUnknownFlag(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-bogus")

	if got.code != 2 || !strings.Contains(got.stderr, "flag provided but not defined: -bogus") {
		t.Errorf("tt -bogus = %+v, want exit code 2 and an error", got)
	}
}

func TestCLIList(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".tt", "themes", "my-theme"), "")

	themes := runCLI(t, home, nil, "-list", "themes")
	if themes.code != 0 {
		t.Fatalf("tt -list themes = %+v", themes)
	}
	for _, want := range []string{"default\n", "gruvbox\n", "my-theme\n"} {
		if !strings.Contains(themes.stdout, want) {
			t.Errorf("tt -list themes output lacks %q", want)
		}
	}

	words := runCLI(t, home, nil, "-list", "words")
	if words.code != 0 || !strings.Contains("\n"+words.stdout, "\n1000en\n") {
		t.Errorf("tt -list words = %+v", words)
	}

	quotes := runCLI(t, home, nil, "-list", "quotes")
	if quotes != (cliResult{"en\nit\n", "", 0}) {
		t.Errorf("tt -list quotes = %+v", quotes)
	}
}

func TestCLIStatsWithoutResults(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-stats")

	want := cliResult{"No saved results yet.\n", "", 0}
	if got != want {
		t.Errorf("tt -stats = %+v, want %+v", got, want)
	}
}

func TestCLIStats(t *testing.T) {
	home := t.TempDir()
	now := time.Now().Unix()
	// Both results are from the same day, so that no progress chart is shown.
	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".results"), fmt.Sprintf(
		`[{"wpm":60,"accuracy":95,"timestamp":%d},{"wpm":40,"accuracy":85,"timestamp":%d}]`,
		now-60, now-60))

	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".errors"), `[{"word":"the","typed":"teh"}]`)

	got := runCLI(t, home, []string{"TZ=UTC"}, "-stats")

	want := cliResult{
		"Typing stats\n" +
			"Tests      2\n" +
			"Best       60 wpm (" + time.Unix(now-60, 0).UTC().Format("2006-01-02") + ")\n" +
			"Last hour  avg wpm 50.00, avg accuracy 90.00% (2 tests)\n" +
			"Last day   avg wpm 50.00, avg accuracy 90.00% (2 tests)\n" +
			"Last week  avg wpm 50.00, avg accuracy 90.00% (2 tests)\n" +
			"All time   avg wpm 50.00, avg accuracy 90.00% (2 tests)\n" +
			"\n" +
			"Most mistyped words\n" +
			"the        1\n",
		"", 0,
	}
	if got != want {
		t.Errorf("tt -stats = %+v, want %+v", got, want)
	}
}

func TestCLIStatsHonoursXDGDataHome(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, "xdg")
	writeFile(t, filepath.Join(data, "tt", ".results"), fmt.Sprintf(`[{"wpm":30,"accuracy":100,"timestamp":%d}]`, time.Now().Unix()))

	got := runCLI(t, home, []string{"XDG_DATA_HOME=" + data}, "-stats")
	if !strings.Contains(got.stdout, "avg wpm 30.00") {
		t.Errorf("tt -stats = %+v, want the results stored in XDG_DATA_HOME", got)
	}
}

func TestCLIStatsWithCorruptResults(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".results"), "not json")

	got := runCLI(t, home, nil, "-stats")
	if got.code != 1 || !strings.HasPrefix(got.stderr, "ERROR: could not read the saved results in ") || !strings.HasSuffix(got.stderr, "fix or remove the file\n") {
		t.Errorf("tt -stats = %+v, want an error", got)
	}
}

func TestCLIExport(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".errors"), `[{"word":"the","typed":"teh"}]`)

	if got := runCLI(t, home, nil, "-export", "mistakes"); got != (cliResult{"word,typed\nthe,teh\n", "", 0}) {
		t.Errorf("tt -export mistakes = %+v, want the mistakes as CSV", got)
	}

	if got := runCLI(t, home, nil, "-export", "mistakes", "-json"); got != (cliResult{`[{"word":"the","typed":"teh"}]` + "\n", "", 0}) {
		t.Errorf("tt -export mistakes -json = %+v, want the mistakes as JSON", got)
	}

	if got := runCLI(t, home, nil, "-export", "nope"); got != (cliResult{"", "ERROR: unknown data nope, use one of: mistakes, results\n", 1}) {
		t.Errorf("tt -export nope = %+v, want an error", got)
	}
}

func TestCLIInvalidResources(t *testing.T) {
	home := t.TempDir()
	badQuotes := filepath.Join(home, "quotes.json")
	writeFile(t, badQuotes, "not json")

	tests := []struct {
		args []string
		want string
	}{
		{
			[]string{"-words", "nope"},
			"ERROR: nope does not appear to be a valid word list, see '-list words' for a list of builtin word lists\n",
		},
		{
			[]string{"-quotes", "nope"},
			"ERROR: nope does not appear to be a valid quote file, see '-list quotes' for a list of builtin quotes\n",
		},
		{
			[]string{"-quotes", badQuotes},
			"ERROR: improperly formatted quote file: invalid character 'o' in literal null (expecting 'u')\n",
		},
	}

	for _, tc := range tests {
		got := runCLI(t, home, nil, tc.args...)
		if want := (cliResult{"", tc.want, 1}); got != want {
			t.Errorf("tt %s = %+v, want %+v", strings.Join(tc.args, " "), got, want)
		}
	}
}

func TestCLIEmptyResources(t *testing.T) {
	home := t.TempDir()
	words := filepath.Join(home, "words.txt")
	quotes := filepath.Join(home, "quotes.json")
	writeFile(t, words, " \n\n")
	writeFile(t, quotes, "[]")

	got := runCLI(t, home, nil, "-words", words)
	if want := (cliResult{"", "ERROR: " + words + " does not contain any words\n", 1}); got != want {
		t.Errorf("tt -words <empty file> = %+v, want %+v", got, want)
	}

	got = runCLI(t, home, nil, "-quotes", quotes)
	if want := (cliResult{"", "ERROR: " + quotes + " does not contain any quotes\n", 1}); got != want {
		t.Errorf("tt -quotes <empty file> = %+v, want %+v", got, want)
	}
}

func TestCLIHelpDescribesLineWidth(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-help")

	want := "" +
		"    -w                  The maximum line length in characters. This option is\n" +
		"                        ignored if -raw is present.\n"
	if !strings.Contains(got.stdout, want) {
		t.Errorf("tt -help does not describe -w correctly:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "Embolden typed text.\n                        ignored") {
		t.Errorf("tt -help attaches part of the -w description to -bold:\n%s", got.stdout)
	}
}

func TestCLIDoesNotCreateDirectoriesNeedlessly(t *testing.T) {
	home := t.TempDir()

	runCLI(t, home, nil, "-v")
	runCLI(t, home, nil, "-stats")

	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("unexpected %s in the home directory", e.Name())
	}
}

func TestCLIMistakesWithoutMistakes(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-mistakes")

	want := cliResult{"", "ERROR: there are no recorded mistakes to practice yet\n", 1}
	if got != want {
		t.Errorf("tt -mistakes = %+v, want %+v", got, want)
	}
}

func TestCLIKeys(t *testing.T) {
	home := t.TempDir()

	if got := runCLI(t, home, nil, "-keys"); got != (cliResult{"No keystrokes recorded yet.\n", "", 0}) {
		t.Errorf("tt -keys without data = %+v", got)
	}

	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".keys"),
		`{"chars":{"a":{"count":20,"errors":0.1,"timed":10,"latency":200},"è":{"count":20,"errors":0.5,"timed":10,"latency":300}},"bigrams":{}}`)

	got := runCLI(t, home, nil, "-keys", "-layout", "it")
	if got.code != 0 || !strings.Contains(got.stdout, "  q  w  e  r  t  y  u  i  o  p  è█ +\n") || strings.Contains(got.stdout, "\x1b[") {
		t.Errorf("tt -keys -layout it = %+v, want an uncoloured Italian keyboard", got)
	}

	if got := runCLI(t, home, nil, "-keys", "-layout", "nope"); got.code != 1 || !strings.Contains(got.stderr, "unknown layout nope") {
		t.Errorf("tt -keys -layout nope = %+v, want an error", got)
	}
}

func TestCLIRejectsInvalidNumbers(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-n", "-5"}, "-n must be at least 1"},
		{[]string{"-n", "0"}, "-n must be at least 1"},
		{[]string{"-g", "-1"}, "-g must be at least 1"},
		{[]string{"-g", "0"}, "-g must be at least 1"},
		{[]string{"-w", "0"}, "-w must be at least 1"},
		{[]string{"-t", "0"}, "-t must be at least 1"},
		{[]string{"-t", "-5"}, "-t must be at least 1"},
		{[]string{"-pace", "-10"}, "-pace can't be negative"},
		{[]string{"-pace", "fast"}, "-pace must be a speed in wpm, best or avg"},
		{[]string{"-focus", "-1"}, "-focus must be between 0 and 100"},
		{[]string{"-focus", "101"}, "-focus must be between 0 and 100"},
	}

	for _, tc := range tests {
		got := runCLI(t, t.TempDir(), nil, tc.args...)
		if want := (cliResult{"", "ERROR: " + tc.want + "\n", 1}); got != want {
			t.Errorf("tt %s = %+v, want %+v", strings.Join(tc.args, " "), got, want)
		}
	}
}

func TestCLIPaceWithoutResults(t *testing.T) {
	got := runCLI(t, t.TempDir(), nil, "-words", "200en", "-n", "10", "-pace", "best")

	if want := (cliResult{"", "ERROR: -pace best needs a saved result of words 200en, 10 words\n", 1}); got != want {
		t.Errorf("tt -pace best = %+v, want %+v", got, want)
	}
}

func TestCLIWithoutText(t *testing.T) {
	// runCLI pipes nothing to stdin.
	got := runCLI(t, t.TempDir(), nil)

	if want := (cliResult{"", "ERROR: there is no text to type\n", 1}); got != want {
		t.Errorf("tt < /dev/null = %+v, want %+v", got, want)
	}
}

func TestCLIRefusesToStartWithCorruptResults(t *testing.T) {
	home := t.TempDir()
	results := filepath.Join(home, ".local", "share", "tt", ".results")
	writeFile(t, results, `[{"wpm":60,"accur`)

	// Checked before typing a test whose result could not be saved.
	got := runCLI(t, home, nil, "-n", "5")

	want := "ERROR: could not read the saved results in " + results + ": unexpected end of JSON input, fix or remove the file\n"
	if got.code != 1 || got.stderr != want {
		t.Errorf("tt with corrupt results = %+v, want the error %q", got, want)
	}

	if b, _ := os.ReadFile(results); string(b) != `[{"wpm":60,"accur` {
		t.Errorf("the results file was changed to %q", b)
	}
}

func TestCLIRefusesToStartWithCorruptMistakes(t *testing.T) {
	home := t.TempDir()
	mistakes := filepath.Join(home, ".local", "share", "tt", ".errors")
	writeFile(t, mistakes, `[{"word":"old",`)

	// Checked before typing a test whose mistakes could not be saved.
	got := runCLI(t, home, nil, "-n", "5")

	want := "ERROR: could not read the recorded mistakes in " + mistakes + ": unexpected end of JSON input, fix or remove the file\n"
	if got.code != 1 || got.stderr != want {
		t.Errorf("tt with corrupt mistakes = %+v, want the error %q", got, want)
	}
}

func TestCLIWithoutHome(t *testing.T) {
	got := runCLI(t, "", nil, "-v")

	if want := (cliResult{"", "ERROR: could not resolve the home directory\n", 1}); got != want {
		t.Errorf("tt -v without a home directory = %+v, want %+v", got, want)
	}
}
