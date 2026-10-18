package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/alexmarchi28/tt/internal/typing"
	"github.com/gdamore/tcell"
)

type result struct {
	Wpm       int              `json:"wpm"`
	Cpm       int              `json:"cpm"`
	Accuracy  float64          `json:"accuracy"`
	Timestamp int64            `json:"timestamp"`
	Mistakes  []typing.Mistake `json:"mistakes"`
}

// personalBest tells whether a test beat the best WPM previously achieved on
// the same kind of test.
type personalBest struct {
	beaten   bool
	previous int
}

// formatReport describes the outcome of a completed test.
func formatReport(res testResult, attribution string, best personalBest) string {
	var b strings.Builder

	if best.beaten {
		fmt.Fprintf(&b, "New personal best! (previous best: %d wpm)\n\n", best.previous)
	}

	fmt.Fprintf(&b, "WPM:         %d\nCPM:         %d\nAccuracy:    %.2f%%", res.wpm(), res.cpm(), res.accuracy())

	if len(res.mistakes) > 0 {
		words := make([]string, len(res.mistakes))
		for i, m := range res.mistakes {
			words[i] = m.Word
		}

		fmt.Fprintf(&b, "\nMistakes:    %s", strings.Join(words, ", "))
	}

	if attribution != "" {
		fmt.Fprintf(&b, "\n\nAttribution: %s", attribution)
	}

	return b.String()
}

// reporter shows the report of a completed test.
type reporter interface {
	// show shows report until the user moves on, and returns true if they
	// asked to quit instead.
	show(report string) (quit bool)
}

// noReporter shows no report (-noreport).
type noReporter struct{}

func (noReporter) show(string) bool { return false }

// screenReporter shows the report on the screen of view, in the colours of
// its current theme.
type screenReporter struct {
	view *view
}

func (r screenReporter) show(report string) bool {
	return showReport(r.view.scr, report, r.view.defaultStyle)
}

// showReport shows a report in the given style until escape is pressed,
// centering it again when the screen is resized. It returns true if the user
// asked to quit instead.
func showReport(scr tcell.Screen, report string, style tcell.Style) (quit bool) {
	draw := func() {
		scr.SetStyle(style)
		scr.Clear()
		drawStringAtCenter(scr, report, style)
		scr.HideCursor()
		scr.Show()
	}
	draw()

	for {
		switch ev := scr.PollEvent().(type) {
		case *tcell.EventResize:
			scr.Sync()
			draw()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				return false
			} else if ev.Key() == tcell.KeyCtrlC {
				return true
			}
		}
	}
}

// resultWriters print the results of the completed tests in each output
// format, requested by the flag of its name.
var resultWriters = map[string]func(w io.Writer, results []result) error{
	"csv":  writeResultsCSV,
	"json": writeResultsJSON,
}

// writeResults prints the results of the completed tests in the requested
// format, if any.
func writeResults(w io.Writer, results []result, format string) error {
	if format == "" {
		return nil
	}

	return resultWriters[format](w, results)
}

func writeResultsJSON(w io.Writer, results []result) error {
	results = nonNil(results)
	for i := range results {
		results[i].Mistakes = nonNil(results[i].Mistakes)
	}

	b, err := json.Marshal(results)
	if err != nil {
		return err
	}

	_, err = w.Write(b)
	return err
}

func writeResultsCSV(w io.Writer, results []result) error {
	var rows [][]string
	for _, r := range results {
		rows = append(rows, []string{
			"test",
			strconv.Itoa(r.Wpm),
			strconv.Itoa(r.Cpm),
			strconv.FormatFloat(r.Accuracy, 'f', 2, 64),
			strconv.FormatInt(r.Timestamp, 10),
		})
		for _, m := range r.Mistakes {
			rows = append(rows, []string{"mistake", m.Word, m.Typed})
		}
	}

	return csv.NewWriter(w).WriteAll(rows)
}
