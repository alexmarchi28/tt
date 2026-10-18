package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/alexmarchi28/tt/internal/history"
)

// exporters print each kind of saved data as CSV, or as JSON, see -export.
var exporters = map[string]func(w io.Writer, st historyStore, asJSON bool) error{
	"results":  exportResults,
	"mistakes": exportMistakes,
}

// exportData prints the saved data of the given kind.
func exportData(w io.Writer, st historyStore, data string, asJSON bool) error {
	export, ok := exporters[data]
	if !ok {
		return fmt.Errorf("unknown data %s, use one of: %s", data, strings.Join(sortedKeys(exporters), ", "))
	}

	return export(w, st, asJSON)
}

func exportResults(w io.Writer, st historyStore, asJSON bool) error {
	results, err := st.Results()
	if err != nil {
		return err
	}

	if asJSON {
		return writeJSON(w, nonNil(results))
	}

	header := make([]string, len(resultColumns))
	for i, c := range resultColumns {
		header[i] = c.name
	}

	rows := [][]string{header}
	for _, r := range results {
		row := make([]string, len(resultColumns))
		for i, c := range resultColumns {
			row[i] = c.value(r)
		}
		rows = append(rows, row)
	}

	return csv.NewWriter(w).WriteAll(rows)
}

// resultColumns are the columns of the results exported as CSV.
var resultColumns = []struct {
	name  string
	value func(r history.Result) string
}{
	{"timestamp", func(r history.Result) string { return strconv.FormatInt(r.Timestamp, 10) }},
	{"wpm", func(r history.Result) string { return strconv.Itoa(r.Wpm) }},
	{"accuracy", func(r history.Result) string { return strconv.FormatFloat(r.Accuracy, 'f', 2, 64) }},
	{"duration", func(r history.Result) string { return strconv.FormatFloat(r.Duration, 'f', 2, 64) }},
	{"mode", func(r history.Result) string { return r.Mode }},
	{"source", func(r history.Result) string { return r.Source }},
	{"seconds", func(r history.Result) string { return strconv.Itoa(r.Seconds) }},
	{"words", func(r history.Result) string { return strconv.Itoa(r.Words) }},
	{"punct", func(r history.Result) string { return strconv.FormatBool(r.Punct) }},
	{"numbers", func(r history.Result) string { return strconv.FormatBool(r.Numbers) }},
	{"focus", func(r history.Result) string { return strconv.Itoa(r.Focus) }},
}

func exportMistakes(w io.Writer, st historyStore, asJSON bool) error {
	mistakes, err := st.Mistakes()
	if err != nil {
		return err
	}

	if asJSON {
		return writeJSON(w, nonNil(mistakes))
	}

	rows := [][]string{{"word", "typed"}}
	for _, m := range mistakes {
		rows = append(rows, []string{m.Word, m.Typed})
	}

	return csv.NewWriter(w).WriteAll(rows)
}

// writeJSON prints v as JSON on a line.
func writeJSON(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// nonNil returns s, or an empty slice if s is nil, so that it is serialized
// as an empty list rather than null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
