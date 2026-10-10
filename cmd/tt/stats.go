package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const week = 7 * 24 * time.Hour

// progressDays is the number of days shown in the progress chart.
const progressDays = 30

// summary aggregates a set of results.
type summary struct {
	count    int
	wpm      float64       // Average WPM.
	accuracy float64       // Average accuracy.
	duration time.Duration // Total time spent typing, as far as it is known.
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
		s.duration += time.Duration(r.Duration * float64(time.Second))
	}

	if s.count > 0 {
		s.wpm /= float64(s.count)
		s.accuracy /= float64(s.count)
	}

	return s
}

// filterResults returns the results for which keep returns true.
func filterResults(results []persistedResult, keep func(persistedResult) bool) []persistedResult {
	var out []persistedResult
	for _, r := range results {
		if keep(r) {
			out = append(out, r)
		}
	}

	return out
}

// since returns the results saved at or after t.
func since(results []persistedResult, t time.Time) []persistedResult {
	return filterResults(results, func(r persistedResult) bool { return r.Timestamp >= t.Unix() })
}

// before returns the results saved before t.
func before(results []persistedResult, t time.Time) []persistedResult {
	return filterResults(results, func(r persistedResult) bool { return r.Timestamp < t.Unix() })
}

// bestWpm returns the best WPM achieved on the given kind of test, and false
// if no such test was completed.
func bestWpm(results []persistedResult, kind testKind) (int, bool) {
	s := summarize(filterResults(results, func(r persistedResult) bool { return r.kind() == kind }))
	return s.best.Wpm, s.count > 0
}

type kindSummary struct {
	kind testKind
	summary
}

// summarizeByKind summarizes the results of each kind of test, the most
// frequent first.
func summarizeByKind(results []persistedResult) []kindSummary {
	groups := map[testKind][]persistedResult{}
	for _, r := range results {
		groups[r.kind()] = append(groups[r.kind()], r)
	}

	summaries := make([]kindSummary, 0, len(groups))
	for kind, group := range groups {
		summaries = append(summaries, kindSummary{kind, summarize(group)})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].count != summaries[j].count {
			return summaries[i].count > summaries[j].count
		}
		return summaries[i].kind.String() < summaries[j].kind.String()
	})

	return summaries
}

// formatTypingTime formats d with a precision suited to its magnitude.
func formatTypingTime(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm %02ds", int(d/time.Minute), int(d%time.Minute/time.Second))
	default:
		return fmt.Sprintf("%dh %02dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	}
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

	tests := fmt.Sprint(all.count)
	if all.duration > 0 {
		tests += fmt.Sprintf(" (%s of typing)", formatTypingTime(all.duration))
	}

	best := bestDate.Format("2006-01-02")
	if all.best.Mode != "" {
		best += ", " + all.best.kind().String()
	}

	fmt.Fprintln(&b, "Typing stats")
	fmt.Fprintf(&b, "%-10s %s\n", "Tests", tests)
	fmt.Fprintf(&b, "%-10s %d wpm (%s)\n", "Best", all.best.Wpm, best)
	fmt.Fprintln(&b, formatSummaryLine("Last hour", summarize(since(stored, now.Add(-time.Hour)))))
	fmt.Fprintln(&b, formatSummaryLine("Last day", summarize(since(stored, now.Add(-24*time.Hour)))))
	lastWeek := summarize(since(stored, now.Add(-week)))
	fmt.Fprintln(&b, formatSummaryLine("Last week", lastWeek))
	fmt.Fprintln(&b, formatSummaryLine("All time", all))

	previousWeek := summarize(before(since(stored, now.Add(-2*week)), now.Add(-week)))
	if lastWeek.count > 0 && previousWeek.count > 0 {
		fmt.Fprintf(&b, "%-10s %+.2f wpm compared to the previous week\n", "Trend", lastWeek.wpm-previousWeek.wpm)
	}

	days := dailyAverages(stored, progressDays, now)
	if daysWithTests(days) > 1 {
		fmt.Fprintln(&b, "\nProgress (average wpm per day)")
		for _, line := range progressChart(days) {
			fmt.Fprintln(&b, line)
		}
	}

	if kinds := summarizeByKind(stored); len(kinds) > 1 {
		width := 0
		for _, k := range kinds {
			if l := len(k.kind.String()); l > width {
				width = l
			}
		}

		fmt.Fprintln(&b, "\nBy test")
		for _, k := range kinds {
			fmt.Fprintf(&b, "%-*s  avg wpm %.2f, avg accuracy %.2f%% (%d tests), best %d wpm\n",
				width, k.kind, k.wpm, k.accuracy, k.count, k.best.Wpm)
		}
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
		die("%v", unreadableResults(err))
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
