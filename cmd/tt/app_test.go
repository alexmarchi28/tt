package main

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

func newTestApp(t *testing.T, opts options) (*app, tcell.SimulationScreen) {
	t.Helper()

	res, _ := newTestResources(t)
	sim := newSimScreen(t)

	a, err := newApp(sim, opts, testKind{mode: "words", source: "test"}, appDeps{newTestStore(t), res, time.Now})
	if err != nil {
		t.Fatal(err)
	}
	a.typer.tty = io.Discard

	return a, sim
}

// testSequence returns a test generator producing the given texts, one test
// each, and then nil. The number of calls is stored in calls.
func testSequence(calls *int, texts ...string) func() []segment {
	return func() []segment {
		*calls++
		if *calls > len(texts) {
			return nil
		}

		return []segment{{texts[*calls-1], ""}}
	}
}

func startApp(a *app, testFn func() []segment) <-chan int {
	done := make(chan int, 1)
	go func() {
		rc, err := a.run(testFn)
		if err != nil {
			rc = -1
		}
		done <- rc
	}()
	return done
}

func waitExitCode(t *testing.T, done <-chan int) int {
	t.Helper()

	select {
	case rc := <-done:
		return rc
	case <-time.After(testWait):
		t.Fatal("app did not return")
	}

	return -1
}

func TestAppRecordsCompletedTests(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	a, sim := newTestApp(t, opts)
	a.kind = testKind{mode: "words", source: "test", seconds: 30, punct: true}

	var calls int
	done := startApp(a, testSequence(&calls, "ab", "cd"))
	sendKeys(t, sim, "x", tcell.KeyEscape, "ab", "cx")

	if rc := waitExitCode(t, done); rc != 0 {
		t.Errorf("exit code = %d, want 0", rc)
	}

	if len(a.results) != 2 || a.results[0].Mistakes != nil || !reflect.DeepEqual(a.results[1].Mistakes, []mistake{{"cd", "cx"}}) {
		t.Errorf("results = %+v, want two tests with a single mistake in the second", a.results)
	}

	stored, err := a.store.Results()
	if err != nil || len(stored) != 2 {
		t.Fatalf("persisted results = %+v (%v), want 2", stored, err)
	}
	for _, r := range stored {
		if r.kind() != a.kind || r.Duration <= 0 || r.Duration > 5 {
			t.Errorf("persisted result %+v, want the kind of test and its duration", r)
		}
	}

	if ks := a.store.KeyStats(); ks.Chars["a"].Count != 1 || ks.Chars["d"].ErrorRate != 1 {
		t.Errorf("key statistics = %+v, want those of the completed tests", ks)
	}

	// The mistake of the restarted attempt is not recorded.
	if mistakes, err := a.store.Mistakes(); err != nil || !reflect.DeepEqual(mistakes, []mistake{{"cd", "cx"}}) {
		t.Errorf("stored mistakes = %v (%v), want [{cd cx}]", mistakes, err)
	}
}

func TestAppNavigation(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "ab", "cd", "ef"))
	sendKeys(t, sim, tcell.KeyRight, tcell.KeyRight, tcell.KeyLeft, tcell.KeyLeft, tcell.KeyLeft, "ab", "cd", "ef")

	if rc := waitExitCode(t, done); rc != 0 {
		t.Errorf("exit code = %d, want 0", rc)
	}
	if calls != 4 {
		t.Errorf("tests generated %d times, want 4", calls)
	}
	if len(a.results) != 3 {
		t.Fatalf("got %d results, want 3", len(a.results))
	}
	for _, r := range a.results {
		if r.Mistakes != nil {
			t.Errorf("unexpected mistakes %v, the wrong test was shown", r.Mistakes)
		}
	}
}

func TestAppOneShot(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	opts.oneShot = true
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "ab", "cd"))
	sendKeys(t, sim, "ab")

	if rc := waitExitCode(t, done); rc != 0 || calls != 1 || len(a.results) != 1 {
		t.Errorf("exit code %d, %d tests generated, %d results, want 0, 1, 1", rc, calls, len(a.results))
	}
}

func TestAppQuit(t *testing.T) {
	a, sim := newTestApp(t, parseFlags(nil))

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	sendKeys(t, sim, "a", tcell.KeyCtrlC)

	if rc := waitExitCode(t, done); rc != 1 || len(a.results) != 0 {
		t.Errorf("exit code %d with %d results, want 1 and none", rc, len(a.results))
	}
}

func TestAppReport(t *testing.T) {
	a, sim := newTestApp(t, parseFlags(nil))

	var calls int
	done := startApp(a, func() []segment {
		calls++
		return []segment{{"ab cd", "Someone"}}
	})

	sendKeys(t, sim, "xb cd")
	waitForScreenText(t, sim, "Mistakes:    ab", true)
	waitForScreenText(t, sim, "Attribution: Someone", true)
	if text := simulationScreenText(sim); !strings.Contains(text, "WPM:") || !strings.Contains(text, "Accuracy:    75.00%") {
		t.Errorf("unexpected report:\n%s", text)
	}

	// Escape moves on to the next test, C-c in the report quits.
	sendKeys(t, sim, tcell.KeyEscape)
	waitForScreenText(t, sim, "Mistakes:", false)
	sendKeys(t, sim, "ab cd", tcell.KeyCtrlC)

	if rc := waitExitCode(t, done); rc != 1 || calls != 2 || len(a.results) != 2 {
		t.Errorf("exit code %d, %d tests generated, %d results, want 1, 2, 2", rc, calls, len(a.results))
	}
}

func TestAppReflowsText(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	opts.maxLineLen = 10
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "the  quick\nbrown fox"))
	waitForScreenText(t, sim, "brown fox", true)

	lines := strings.Split(simulationScreenText(sim), "\n")
	for _, l := range lines {
		if strings.Contains(l, "quick") && strings.Contains(l, "brown") {
			t.Errorf("text was not wrapped at 10 characters: %q", l)
		}
	}

	sendKeys(t, sim, "the quick brown fox")
	waitExitCode(t, done)

	if len(a.results) != 1 || a.results[0].Mistakes != nil {
		t.Errorf("results = %+v, want one test without mistakes", a.results)
	}
}

func TestAppResizeKeepsTheTest(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	opts.maxLineLen = 80
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "alpha beta gamma delta"))
	waitForScreenText(t, sim, "beta gamma", true)

	sendKeys(t, sim, "alpha be")
	resizeScreen(t, sim, 20, 25)
	waitForScreenText(t, sim, "gamma delta", true)
	waitForScreenText(t, sim, "beta gamma", false)
	sendKeys(t, sim, "ta gamma delta")
	waitExitCode(t, done)

	if calls != 2 || len(a.results) != 1 || a.results[0].Mistakes != nil {
		t.Errorf("%d tests generated, results = %+v, want 2 and one test without mistakes", calls, a.results)
	}
}

func TestAppRawMode(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	opts.raw = true
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "ab\ncd"))
	sendKeys(t, sim, "abcd")
	waitExitCode(t, done)

	if len(a.results) != 1 || a.results[0].Mistakes != nil {
		t.Errorf("results = %+v, want one test without mistakes", a.results)
	}
}

func TestAppCyclesThemes(t *testing.T) {
	a, sim := newTestApp(t, parseFlags(nil))

	res, _ := newTestResources(t)
	names := res.list("themes")
	next := names[indexResourceName(names, "default")+1]

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	sendKeys(t, sim, tcell.KeyCtrlN)
	waitForScreenText(t, sim, "Theme: "+next, true)
	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)

	if a.theme != next {
		t.Errorf("theme to remember = %q, want %q", a.theme, next)
	}
}

func TestAppWithoutTheme(t *testing.T) {
	opts := parseFlags(nil)
	opts.noTheme = true
	a, sim := newTestApp(t, opts)

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	sendKeys(t, sim, tcell.KeyCtrlN, tcell.KeyCtrlC)
	waitExitCode(t, done)

	if a.theme != "" || a.typer.CycleTheme != nil {
		t.Errorf("themes should not be used with -notheme, theme to remember %q", a.theme)
	}
}

func TestNewAppAppliesOptions(t *testing.T) {
	opts := parseFlags([]string{"-noskip", "-nobackspace", "-showwpm", "-showaccuracy", "-nohighlight", "-blockcursor", "-blind", "-pace", "70"})
	a, _ := newTestApp(t, opts)

	ty := a.typer
	if ty.SkipWord || !ty.DisableBackspace || !ty.ShowWpm || !ty.ShowAccuracy || ty.highlightCurrent || ty.highlightNext || !ty.BlockCursor || !ty.Blind || ty.Pace != 70 {
		t.Errorf("options not applied to the typer: %+v", ty)
	}
	if a.theme != "default" {
		t.Errorf("theme to remember = %q, want default", a.theme)
	}
}

func TestAppReportsPersonalBest(t *testing.T) {
	tests := []struct {
		name     string
		previous []persistedResult
		want     bool
	}{
		{"first test", nil, false},
		{"beaten", []persistedResult{{Wpm: 1, Mode: "words", Source: "test"}}, true},
		{"not beaten", []persistedResult{{Wpm: 1_000_000_000, Mode: "words", Source: "test"}}, false},
		{"other kind of test", []persistedResult{{Wpm: 1, Mode: "quotes", Source: "en"}}, false},
		{"other variant of the test", []persistedResult{{Wpm: 1, Mode: "words", Source: "test", Punct: true}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, sim := newTestApp(t, parseFlags(nil))
			if err := writeValue(a.store.(*jsonStore).path(resultsFile), tc.previous); err != nil {
				t.Fatal(err)
			}

			var calls int
			done := startApp(a, testSequence(&calls, "ab"))
			sendKeys(t, sim, "ab")
			waitForScreenText(t, sim, "WPM:", true)

			if got := strings.Contains(simulationScreenText(sim), "New personal best!"); got != tc.want {
				t.Errorf("personal best shown = %t, want %t:\n%s", got, tc.want, simulationScreenText(sim))
			}

			sendKeys(t, sim, tcell.KeyCtrlC)
			waitExitCode(t, done)
		})
	}
}

func TestAppRecordsTheTimeOfTests(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	a, sim := newTestApp(t, opts)
	a.now = func() time.Time { return time.Unix(1700000000, 0) }

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	sendKeys(t, sim, "ab")
	waitExitCode(t, done)

	if stored, err := a.store.Results(); err != nil || len(stored) != 1 || stored[0].Timestamp != 1700000000 {
		t.Errorf("saved results = %+v, %v, want the test typed at the time of the clock", stored, err)
	}
	if len(a.results) != 1 || a.results[0].Timestamp != 1700000000 {
		t.Errorf("results = %+v, want the test typed at the time of the clock", a.results)
	}
}
