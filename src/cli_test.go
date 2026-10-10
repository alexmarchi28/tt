package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cliResult struct {
	stdout, stderr string
	code           int
}

// runCLI runs tt with the given arguments in a separate process whose home
// directory is home. TERM is set to an unknown terminal so that a test
// reaching the interactive part fails instead of taking over the terminal.
func runCLI(t *testing.T, home string, env []string, args ...string) cliResult {
	t.Helper()

	cmd := exec.Command(os.Args[0], args...)
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "HOME", "XDG_DATA_HOME", "TERM":
		default:
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "TT_TEST_RUN_MAIN=1", "HOME="+home, "TERM=tt-test-unknown-terminal")
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = strings.NewReader("")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running tt: %v", err)
	}

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

	want := cliResult{"tt version 0.4.4\n", "", 0}
	if got != want {
		t.Errorf("tt -v = %+v, want %+v", got, want)
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
	if quotes != (cliResult{"en\n", "", 0}) {
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
	writeFile(t, filepath.Join(home, ".local", "share", "tt", ".results"), fmt.Sprintf(
		`[{"wpm":60,"accuracy":95,"timestamp":%d},{"wpm":40,"accuracy":85,"timestamp":%d}]`,
		now-60, now-3*3600))

	got := runCLI(t, home, nil, "-stats")

	want := cliResult{
		"Typing stats\n" +
			"Last hour  avg wpm 60.00, avg accuracy 95.00% (1 tests)\n" +
			"Last day   avg wpm 50.00, avg accuracy 90.00% (2 tests)\n" +
			"Last week  avg wpm 50.00, avg accuracy 90.00% (2 tests)\n",
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
	if got.code != 1 || !strings.HasPrefix(got.stderr, "ERROR: Could not read saved results: ") {
		t.Errorf("tt -stats = %+v, want an error", got)
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
			"ERROR: nope does not appear to be a valid word list. See '-list words' for a list of builtin word lists.\n",
		},
		{
			[]string{"-quotes", "nope"},
			"ERROR: nope does not appear to be a valid quote file. See '-list quotes' for a list of builtin quotes.\n",
		},
		{
			[]string{"-quotes", badQuotes},
			"ERROR: Improperly formatted quote file: invalid character 'o' in literal null (expecting 'u')\n",
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
	if want := (cliResult{"", "ERROR: " + words + " does not contain any words.\n", 1}); got != want {
		t.Errorf("tt -words <empty file> = %+v, want %+v", got, want)
	}

	got = runCLI(t, home, nil, "-quotes", quotes)
	if want := (cliResult{"", "ERROR: " + quotes + " does not contain any quotes.\n", 1}); got != want {
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
