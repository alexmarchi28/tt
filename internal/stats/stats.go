// Package stats summarizes the saved results and key statistics: the
// statistics of -stats, the keyboard of -keys and the speed of -pace best
// and avg.
package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/typing"
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
	best     history.Result
}

func summarize(results []history.Result) summary {
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
func filterResults(results []history.Result, keep func(history.Result) bool) []history.Result {
	var out []history.Result
	for _, r := range results {
		if keep(r) {
			out = append(out, r)
		}
	}

	return out
}

// since returns the results saved at or after t.
func since(results []history.Result, t time.Time) []history.Result {
	return filterResults(results, func(r history.Result) bool { return r.Timestamp >= t.Unix() })
}

// before returns the results saved before t.
func before(results []history.Result, t time.Time) []history.Result {
	return filterResults(results, func(r history.Result) bool { return r.Timestamp < t.Unix() })
}

// ofKind returns the results of the given kind of test.
func ofKind(results []history.Result, kind history.Kind) []history.Result {
	return filterResults(results, func(r history.Result) bool { return r.Kind == kind })
}

// BestWpm returns the best WPM achieved on the given kind of test, and false
// if no such test was completed.
func BestWpm(results []history.Result, kind history.Kind) (int, bool) {
	s := summarize(ofKind(results, kind))
	return s.best.Wpm, s.count > 0
}

type kindSummary struct {
	kind history.Kind
	summary
}

// summarizeByKind summarizes the results of each kind of test, the most
// frequent first.
func summarizeByKind(results []history.Result) []kindSummary {
	groups := map[history.Kind][]history.Result{}
	for _, r := range results {
		groups[r.Kind] = append(groups[r.Kind], r)
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

func formatSummaryLine(label string, s summary) string {
	if s.count == 0 {
		return fmt.Sprintf("%-10s no results", label)
	}

	return fmt.Sprintf("%-10s avg wpm %.2f, avg accuracy %.2f%% (%d tests)", label, s.wpm, s.accuracy, s.count)
}

// Format describes the saved results and mistakes as of now. There must
// be at least one result.
func Format(stored []history.Result, mistakes []typing.Mistake, now time.Time) string {
	var b strings.Builder
	all := summarize(stored)
	bestDate := time.Unix(all.best.Timestamp, 0).In(now.Location())

	tests := fmt.Sprint(all.count)
	if all.duration > 0 {
		tests += fmt.Sprintf(" (%s of typing)", formatTypingTime(all.duration))
	}

	best := bestDate.Format("2006-01-02")
	if all.best.Mode != "" {
		best += ", " + all.best.Kind.String()
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

	if words := typing.MostMistyped(mistakes, 10); len(words) > 0 {
		fmt.Fprintln(&b, "\nMost mistyped words")
		for _, w := range words {
			fmt.Fprintf(&b, "%-10s %d\n", w.Word, w.Count)
		}
	}

	return b.String()
}

// Speeds of the pacer taken from the saved results, see -pace.
const (
	PaceBest    = "best" // The best result.
	PaceAverage = "avg"  // The average of the recent results, see recentPaceResults.
)

// recentPaceResults is the number of latest results averaged by -pace avg.
const recentPaceResults = 10

// Pace returns the speed of the pacer set by target (paceBest or
// paceAverage) from the saved results of the given kind of test.
func Pace(target string, results []history.Result, kind history.Kind) (int, error) {
	results = ofKind(results, kind)
	if len(results) == 0 {
		return 0, fmt.Errorf("-pace %s needs a saved result of %s", target, kind)
	}

	if target == PaceBest {
		return summarize(results).best.Wpm, nil
	}

	if len(results) > recentPaceResults {
		results = results[len(results)-recentPaceResults:]
	}

	return int(math.Round(summarize(results).wpm)), nil
}
