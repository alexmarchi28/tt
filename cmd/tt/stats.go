package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const week = 7 * 24 * time.Hour

// summary aggregates a set of results.
type summary struct {
	count    int
	wpm      float64 // Average WPM.
	accuracy float64 // Average accuracy.
	best     persistedResult
}

func summarize(results []persistedResult) summary {
	var s summary

	for _, r := range results {
		if s.count == 0 || r.Wpm > s.best.Wpm {
			s.best = r
		}

		s.count++
		s.wpm += float64(r.Wpm)
		s.accuracy += r.Accuracy
	}

	if s.count > 0 {
		s.wpm /= float64(s.count)
		s.accuracy /= float64(s.count)
	}

	return s
}

// since returns the results saved at or after t.
func since(results []persistedResult, t time.Time) []persistedResult {
	var out []persistedResult
	for _, r := range results {
		if r.Timestamp >= t.Unix() {
			out = append(out, r)
		}
	}

	return out
}

// before returns the results saved before t.
func before(results []persistedResult, t time.Time) []persistedResult {
	var out []persistedResult
	for _, r := range results {
		if r.Timestamp < t.Unix() {
			out = append(out, r)
		}
	}

	return out
}

type wordCount struct {
	word  string
	count int
}

// mostMistyped returns up to n of the most frequently mistyped words, the most
// frequent first.
func mostMistyped(mistakes []mistake, n int) []wordCount {
	counts := map[string]int{}
	for _, m := range mistakes {
		counts[m.Word]++
	}

	words := make([]wordCount, 0, len(counts))
	for w, c := range counts {
		words = append(words, wordCount{w, c})
	}

	sort.Slice(words, func(i, j int) bool {
		if words[i].count != words[j].count {
			return words[i].count > words[j].count
		}
		return words[i].word < words[j].word
	})

	if len(words) > n {
		words = words[:n]
	}

	return words
}

func formatSummaryLine(label string, s summary) string {
	if s.count == 0 {
		return fmt.Sprintf("%-10s no results", label)
	}

	return fmt.Sprintf("%-10s avg wpm %.2f, avg accuracy %.2f%% (%d tests)", label, s.wpm, s.accuracy, s.count)
}

// formatStats describes the saved results and mistakes as of now. There must
// be at least one result.
func formatStats(stored []persistedResult, mistakes []mistake, now time.Time) string {
	var b strings.Builder
	all := summarize(stored)
	bestDate := time.Unix(all.best.Timestamp, 0).In(now.Location())

	fmt.Fprintln(&b, "Typing stats")
	fmt.Fprintf(&b, "%-10s %d\n", "Tests", all.count)
	fmt.Fprintf(&b, "%-10s %d wpm (%s)\n", "Best", all.best.Wpm, bestDate.Format("2006-01-02"))
	fmt.Fprintln(&b, formatSummaryLine("Last hour", summarize(since(stored, now.Add(-time.Hour)))))
	fmt.Fprintln(&b, formatSummaryLine("Last day", summarize(since(stored, now.Add(-24*time.Hour)))))
	lastWeek := summarize(since(stored, now.Add(-week)))
	fmt.Fprintln(&b, formatSummaryLine("Last week", lastWeek))
	fmt.Fprintln(&b, formatSummaryLine("All time", all))

	previousWeek := summarize(before(since(stored, now.Add(-2*week)), now.Add(-week)))
	if lastWeek.count > 0 && previousWeek.count > 0 {
		fmt.Fprintf(&b, "%-10s %+.2f wpm compared to the previous week\n", "Trend", lastWeek.wpm-previousWeek.wpm)
	}

	if words := mostMistyped(mistakes, 10); len(words) > 0 {
		fmt.Fprintln(&b, "\nMost mistyped words")
		for _, w := range words {
			fmt.Fprintf(&b, "%-10s %d\n", w.word, w.count)
		}
	}

	return b.String()
}

func printStats() {
	stored, err := readPersistedResults()
	if err != nil && !os.IsNotExist(err) {
		die("Could not read saved results: %v", err)
	}

	if len(stored) == 0 {
		fmt.Println("No saved results yet.")
		return
	}

	//Mistakes are only an addition to the results, ignore them if they can't be read.
	var mistakes []mistake
	if err := readValue(mistakeDB, &mistakes); err != nil {
		mistakes = nil
	}

	fmt.Print(formatStats(stored, mistakes, time.Now()))
}
