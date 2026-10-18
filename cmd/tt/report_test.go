package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
	"github.com/gdamore/tcell"
)

var testResults = []result{
	{Wpm: 60, Cpm: 300, Accuracy: 97.5, Timestamp: 1000},
	{Wpm: 50, Cpm: 250, Accuracy: 100, Timestamp: 2000, Mistakes: []typing.Mistake{{Word: "alpha", Typed: "alpah"}, {Word: "beta", Typed: "bet_"}}},
}

func TestWriteResults(t *testing.T) {
	tests := []struct {
		name    string
		results []result
		format  string
		want    string
	}{
		{"nothing requested", testResults, "", ""},
		{
			"json", testResults, "json",
			`[{"wpm":60,"cpm":300,"accuracy":97.5,"timestamp":1000,"mistakes":[]},` +
				`{"wpm":50,"cpm":250,"accuracy":100,"timestamp":2000,"mistakes":[{"word":"alpha","typed":"alpah"},{"word":"beta","typed":"bet_"}]}]`,
		},
		{
			"csv", testResults, "csv",
			"test,60,300,97.50,1000\n" +
				"test,50,250,100.00,2000\n" +
				"mistake,alpha,alpah\n" +
				"mistake,beta,bet_\n",
		},
		{"csv without results", nil, "csv", ""},
		{"json without results", nil, "json", "[]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := writeResults(&b, tc.results, tc.format); err != nil {
				t.Fatal(err)
			}

			if b.String() != tc.want {
				t.Errorf("got\n%s\nwant\n%s", b.String(), tc.want)
			}
		})
	}
}

// Every output format can be requested with the flag of its name.
func TestResultWritersHaveFlags(t *testing.T) {
	for format := range resultWriters {
		if s := resolveFlags([]string{"-" + format}); s.output.format != format {
			t.Errorf("tt -%s: output format %q, want %q", format, s.output.format, format)
		}
	}
	if s := resolveFlags(nil); s.output.format != "" {
		t.Errorf("output format %q by default, want none", s.output.format)
	}
}

// failingWriter fails every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteResultsReportsWriteErrors(t *testing.T) {
	for format := range resultWriters {
		if err := writeResults(failingWriter{}, testResults, format); err == nil {
			t.Errorf("%s: no error on a failed write", format)
		}
	}
}

func TestWriteResultsEscapesCSV(t *testing.T) {
	results := []result{{Wpm: 40, Cpm: 200, Accuracy: 90, Timestamp: 3000, Mistakes: []typing.Mistake{
		{Word: "world,", Typed: "worl,"},
		{Word: `"quoted"`, Typed: `"quoted_`},
	}}}

	var b bytes.Buffer
	if err := writeResults(&b, results, "csv"); err != nil {
		t.Fatal(err)
	}

	want := "test,40,200,90.00,3000\n" +
		"mistake,\"world,\",\"worl,\"\n" +
		"mistake,\"\"\"quoted\"\"\",\"\"\"quoted_\"\n"
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}

func TestFormatReport(t *testing.T) {
	res := testResult{
		duration:     time.Minute,
		correctChars: 300,
		correctKeys:  39,
		wrongKeys:    1,
		mistakes:     []typing.Mistake{{Word: "alpha", Typed: "alpah"}, {Word: "beta", Typed: "bet_"}},
	}

	tests := []struct {
		name        string
		res         testResult
		attribution string
		best        personalBest
		want        string
	}{
		{"plain", testResult{duration: time.Minute, correctChars: 250, correctKeys: 10}, "", personalBest{}, "" +
			"WPM:         50\n" +
			"CPM:         250\n" +
			"Accuracy:    100.00%"},
		{"mistakes and attribution", res, "Someone", personalBest{}, "" +
			"WPM:         60\n" +
			"CPM:         300\n" +
			"Accuracy:    97.50%\n" +
			"Mistakes:    alpha, beta\n" +
			"\n" +
			"Attribution: Someone"},
		{"personal best", res, "", personalBest{beaten: true, previous: 55}, "" +
			"New personal best! (previous best: 55 wpm)\n" +
			"\n" +
			"WPM:         60\n" +
			"CPM:         300\n" +
			"Accuracy:    97.50%\n" +
			"Mistakes:    alpha, beta"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatReport(tc.res, tc.attribution, tc.best); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// The report is shown in the colours of the theme, and centered again when
// the screen is resized.
func TestShowReport(t *testing.T) {
	sim := newSimScreen(t)
	sim.SetSize(40, 10)
	style := tcell.StyleDefault.Foreground(testFg).Background(testBg)

	done := make(chan bool, 1)
	go func() { done <- showReport(sim, "WPM: 50\nCPM: 250", style) }()

	waitForScreenText(t, sim, "WPM: 50", true)
	if line := screenLine(sim, 4); line != strings.Repeat(" ", 16)+"WPM: 50" {
		t.Errorf("report line = %q, want it centered", line)
	}
	if fg, bg, _ := cellColors(sim, 0, 0); fg != testFg || bg != testBg {
		t.Errorf("background drawn in (%v, %v), want the colours of the theme", fg, bg)
	}

	resizeScreen(t, sim, 20, 6)
	waitFor(t, sim, func() bool { return screenLine(sim, 2) == strings.Repeat(" ", 6)+"WPM: 50" })

	sendKeys(t, sim, tcell.KeyEscape)
	select {
	case quit := <-done:
		if quit {
			t.Error("escape asked to quit")
		}
	case <-time.After(testWait):
		t.Fatal("the report was not closed")
	}
}

// The report is shown in the colours of the current theme of the view.
func TestScreenReporter(t *testing.T) {
	v, sim := newTestView(t, defaultViewOptions)
	v.setTheme(theme{testHi, testHi2, testHi3, testHi, testHi, testErrCol})

	done := make(chan bool, 1)
	go func() { done <- screenReporter{v}.show("WPM: 50") }()

	waitForScreenText(t, sim, "WPM: 50", true)
	if fg, bg, _ := cellColors(sim, 0, 0); fg != testHi || bg != testHi2 {
		t.Errorf("background drawn in (%v, %v), want the colours of the current theme", fg, bg)
	}

	sendKeys(t, sim, tcell.KeyCtrlC)
	select {
	case quit := <-done:
		if !quit {
			t.Error("C-c did not ask to quit")
		}
	case <-time.After(testWait):
		t.Fatal("the report was not closed")
	}
}
