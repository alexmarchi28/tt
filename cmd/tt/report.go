package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gdamore/tcell"
)

type result struct {
	Wpm       int       `json:"wpm"`
	Cpm       int       `json:"cpm"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp int64     `json:"timestamp"`
	Mistakes  []mistake `json:"mistakes"`
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

// showReport shows a report until escape is pressed. It returns true if the
// user asked to quit instead.
func showReport(scr tcell.Screen, report string) (quit bool) {
	scr.Clear()
	drawStringAtCenter(scr, report, tcell.StyleDefault)
	scr.HideCursor()
	scr.Show()

	for {
		if key, ok := scr.PollEvent().(*tcell.EventKey); ok && key.Key() == tcell.KeyEscape {
			return false
		} else if ok && key.Key() == tcell.KeyCtrlC {
			return true
		}
	}
}

// writeResults prints the results of the completed tests in the requested
// formats.
func writeResults(w io.Writer, results []result, asJSON, asCSV bool) error {
	if asJSON {
		results = nonNil(results)
		for i := range results {
			results[i].Mistakes = nonNil(results[i].Mistakes)
		}

		b, err := json.Marshal(results)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}

	if asCSV {
		cw := csv.NewWriter(w)
		for _, r := range results {
			cw.Write([]string{
				"test",
				strconv.Itoa(r.Wpm),
				strconv.Itoa(r.Cpm),
				strconv.FormatFloat(r.Accuracy, 'f', 2, 64),
				strconv.FormatInt(r.Timestamp, 10),
			})
			for _, m := range r.Mistakes {
				cw.Write([]string{"mistake", m.Word, m.Typed})
			}
		}

		cw.Flush()
		return cw.Error()
	}

	return nil
}
