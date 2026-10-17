package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// exporters print each kind of saved data as CSV, or as JSON, see -export.
var exporters = map[string]func(w io.Writer, asJSON bool) error{
	"results":  exportResults,
	"mistakes": exportMistakes,
}

// exportData prints the saved data of the given kind.
func exportData(w io.Writer, data string, asJSON bool) error {
	export, ok := exporters[data]
	if !ok {
		return fmt.Errorf("unknown data %s, use one of: %s", data, strings.Join(sortedKeys(exporters), ", "))
	}

	return export(w, asJSON)
}

func exportResults(w io.Writer, asJSON bool) error {
	results, err := readPersistedResults()
	if err != nil {
		return err
	}

	if asJSON {
		return writeJSON(w, nonNil(results))
	}

	rows := [][]string{{"timestamp", "wpm", "accuracy", "duration", "mode", "source", "seconds", "words", "punct", "numbers", "focus"}}
	for _, r := range results {
		rows = append(rows, []string{
			strconv.FormatInt(r.Timestamp, 10),
			strconv.Itoa(r.Wpm),
			strconv.FormatFloat(r.Accuracy, 'f', 2, 64),
			strconv.FormatFloat(r.Duration, 'f', 2, 64),
			r.Mode,
			r.Source,
			strconv.Itoa(r.Seconds),
			strconv.Itoa(r.Words),
			strconv.FormatBool(r.Punct),
			strconv.FormatBool(r.Numbers),
			strconv.Itoa(r.Focus),
		})
	}

	return csv.NewWriter(w).WriteAll(rows)
}

func exportMistakes(w io.Writer, asJSON bool) error {
	mistakes, err := readMistakes()
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
