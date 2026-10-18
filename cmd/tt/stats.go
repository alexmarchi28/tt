package main

import (
	"fmt"
	"io"
	"time"

	"github.com/alexmarchi28/tt/internal/stats"
)

func printStats(w io.Writer, st historyStore, now time.Time) error {
	stored, err := st.Results()
	if err != nil {
		return err
	}

	if len(stored) == 0 {
		_, err := fmt.Fprintln(w, "No saved results yet.")
		return err
	}

	//Mistakes are only an addition to the results, ignore them if they can't be read.
	mistakes, _ := st.Mistakes()

	_, err = fmt.Fprint(w, stats.Format(stored, mistakes, now))
	return err
}

// printKeys prints how each key is typed on the given keyboard layout.
func printKeys(w io.Writer, st keyStatsStore, layout string, color bool) error {
	rows, err := stats.KeyboardLayout(layout)
	if err != nil {
		return err
	}

	ks := st.KeyStats()
	if len(ks.Chars) == 0 {
		_, err := fmt.Fprintln(w, "No keystrokes recorded yet.")
		return err
	}

	_, err = fmt.Fprint(w, stats.FormatKeys(ks, rows, color))
	return err
}
