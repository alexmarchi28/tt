package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// testRun is a run of tt in the test process.
type testRun struct {
	stdout, stderr bytes.Buffer
	sim            tcell.SimulationScreen
	trueColor      bool          // As requested by run.
	initialized    chan struct{} // Closed once run initializes the screen.
}

// initSignallingScreen tells when it is initialized, before which it can't
// be used by tests.
type initSignallingScreen struct {
	tcell.SimulationScreen
	initialized chan struct{}
}

func (s initSignallingScreen) Init() error {
	defer close(s.initialized)
	return s.SimulationScreen.Init()
}

// environment returns the environment of a run reading stdin from text, as
// if piped unless text is empty, and typing on a simulation screen.
func (r *testRun) environment(text string) environment {
	r.sim = tcell.NewSimulationScreen("")
	r.initialized = make(chan struct{})

	return environment{
		stdin:           strings.NewReader(text),
		stdout:          &r.stdout,
		stderr:          &r.stderr,
		stdinIsTerminal: text == "",
		getenv:          func(string) string { return "" },
		newScreen: func(trueColor bool) (tcell.Screen, error) {
			r.trueColor = trueColor
			return initSignallingScreen{r.sim, r.initialized}, nil
		},
	}
}

// startRun runs tt in the background with the given stdin text and
// arguments, and waits for its screen. The block cursor keeps the typer from
// writing to the terminal.
func startRun(t *testing.T, text string, args ...string) (*testRun, <-chan int) {
	t.Helper()

	r := &testRun{}
	env := r.environment(text)
	done := make(chan int, 1)
	go func() { done <- run(append([]string{"-blockcursor"}, args...), env) }()

	select {
	case <-r.initialized:
	case code := <-done:
		t.Fatalf("tt exited with %d before showing a test, stderr %q", code, r.stderr.String())
	case <-time.After(testWait):
		t.Fatal("tt did not initialize the screen")
	}

	return r, done
}

func TestRunCommands(t *testing.T) {
	useTempStorage(t)

	tests := []struct {
		args         []string
		code         int
		stdout       string // Prefix of the output.
		stderrSubstr string
	}{
		{[]string{"-v"}, 0, "tt version ", ""},
		{[]string{"-help"}, 0, "usage: tt [options] [file]\n", ""},
		{[]string{"-bogus"}, 2, "usage: tt", "flag provided but not defined: -bogus"},
		{[]string{"-n", "0"}, 1, "", "ERROR: -n must be at least 1\n"},
		{[]string{"-stats"}, 0, "No saved results yet.\n", ""},
		{[]string{"-keys", "-layout", "nope"}, 1, "", "ERROR: unknown layout nope"},
		{[]string{"-list", "quotes"}, 0, "en\nit\n", ""},
	}

	for _, tc := range tests {
		var r testRun
		code := run(tc.args, r.environment(""))

		if code != tc.code || !strings.HasPrefix(r.stdout.String(), tc.stdout) || !strings.Contains(r.stderr.String(), tc.stderrSubstr) {
			t.Errorf("tt %s = %d, stdout %q, stderr %q, want %d, stdout starting with %q and stderr containing %q",
				strings.Join(tc.args, " "), code, r.stdout.String(), r.stderr.String(), tc.code, tc.stdout, tc.stderrSubstr)
		}
	}
}

func TestRunTypesTests(t *testing.T) {
	useTempStorage(t)
	r, done := startRun(t, "hi there", "-oneshot", "-noreport", "-csv")

	waitForScreenText(t, r.sim, "hi there", true)
	sendKeys(t, r.sim, "hi there")

	if code := waitExitCode(t, done); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, r.stderr.String())
	}
	if out := r.stdout.String(); !strings.HasPrefix(out, "test,") {
		t.Errorf("stdout = %q, want the result as CSV", out)
	}
	if !r.trueColor {
		t.Error("true colour disabled without -notheme")
	}

	stored, err := readPersistedResults()
	if err != nil || len(stored) != 1 || stored[0].Mode != modeText {
		t.Errorf("saved results = %+v, %v, want the text test", stored, err)
	}
}

func TestRunWithoutTheme(t *testing.T) {
	useTempStorage(t)
	r, done := startRun(t, "hi", "-oneshot", "-noreport", "-notheme")

	sendKeys(t, r.sim, "hi")
	waitExitCode(t, done)

	if r.trueColor {
		t.Error("true colour enabled with -notheme")
	}
}

// An outcome which can't be saved ends the session with an error rather than
// a crash, the results typed so far still being printed.
func TestRunReportsSaveErrors(t *testing.T) {
	useTempStorage(t)
	notDir := filepath.Join(t.TempDir(), "file")
	writeFile(t, notDir, "")
	keyStatsDB = filepath.Join(notDir, ".keys")

	r, done := startRun(t, "hi", "-noreport", "-csv")

	sendKeys(t, r.sim, "hi")

	if code := waitExitCode(t, done); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if !strings.HasPrefix(r.stderr.String(), "ERROR: could not save the key statistics: ") {
		t.Errorf("stderr = %q, want the error", r.stderr.String())
	}
	if !strings.HasPrefix(r.stdout.String(), "test,") {
		t.Errorf("stdout = %q, want the result typed", r.stdout.String())
	}
}

func TestRunReportsScreenErrors(t *testing.T) {
	useTempStorage(t)

	var r testRun
	env := r.environment("hi")
	env.newScreen = func(bool) (tcell.Screen, error) { return nil, errors.New("no terminal") }

	if code := run(nil, env); code != 1 || r.stderr.String() != "ERROR: no terminal\n" {
		t.Errorf("run without a screen = %d, %q, want the error", code, r.stderr.String())
	}
}

func TestNewAppWithInvalidTheme(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "themes", "broken", "fgcol: nope")

	_, err := newApp(newSimScreen(t), parseFlags([]string{"-theme", "broken"}), testKind{})
	if err == nil || !strings.HasPrefix(err.Error(), "broken theme is invalid: ") {
		t.Errorf("newApp with an invalid theme: %v, want an error", err)
	}
}

func TestProcessEnvironment(t *testing.T) {
	env := processEnvironment()

	if env.stdin != os.Stdin || env.stdout != os.Stdout || env.stderr != os.Stderr || env.getenv == nil || env.newScreen == nil {
		t.Errorf("processEnvironment() = %+v", env)
	}
}
