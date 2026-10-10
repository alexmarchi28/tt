package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/gdamore/tcell"
)

type result struct {
	Wpm       int       `json:"wpm"`
	Cpm       int       `json:"cpm"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp int64     `json:"timestamp"`
	Mistakes  []mistake `json:"mistakes"`
}

// showReport shows the results of a test until escape is pressed. It
// returns true if the user asked to quit instead.
func showReport(scr tcell.Screen, cpm, wpm int, accuracy float64, attribution string, mistakes []mistake) (quit bool) {
	mistakeStr := ""
	if attribution != "" {
		attribution = "\n\nAttribution: " + attribution
	}

	if len(mistakes) > 0 {
		mistakeStr = "\nMistakes:    "
		for i, m := range mistakes {
			mistakeStr += m.Word
			if i != len(mistakes)-1 {
				mistakeStr += ", "
			}
		}
	}

	report := fmt.Sprintf("WPM:         %d\nCPM:         %d\nAccuracy:    %.2f%%%s%s", wpm, cpm, accuracy, mistakeStr, attribution)

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
		//Avoid null in serialized JSON.
		if results == nil {
			results = []result{}
		}
		for i := range results {
			if results[i].Mistakes == nil {
				results[i].Mistakes = []mistake{}
			}
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
