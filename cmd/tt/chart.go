package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// chartRows is the height of the progress chart.
const chartRows = 8

// chartBlocks draw the top of a column, by eighths of a row.
var chartBlocks = []rune(" ▁▂▃▄▅▆▇█")

type dayAverage struct {
	date  time.Time
	wpm   float64 // Average WPM of the day.
	count int     // Number of tests, 0 if there were none.
}

// dailyAverages returns the average WPM of each of the n days up to now,
// oldest first. Days are those of the time zone of now.
func dailyAverages(results []persistedResult, n int, now time.Time) []dayAverage {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := make([]dayAverage, n)
	index := map[string]int{}

	for i := range days {
		days[i].date = today.AddDate(0, 0, i-n+1)
		index[days[i].date.Format("2006-01-02")] = i
	}

	for _, r := range results {
		if i, ok := index[time.Unix(r.Timestamp, 0).In(now.Location()).Format("2006-01-02")]; ok {
			days[i].wpm += float64(r.Wpm)
			days[i].count++
		}
	}

	for i := range days {
		if days[i].count > 0 {
			days[i].wpm /= float64(days[i].count)
		}
	}

	return days
}

func daysWithTests(days []dayAverage) int {
	n := 0
	for _, d := range days {
		if d.count > 0 {
			n++
		}
	}

	return n
}

// progressChart draws a column for each day with tests, from the first to the
// last day.
func progressChart(days []dayAverage) []string {
	values := make([]float64, len(days))
	shown := make([]bool, len(days))
	for i, d := range days {
		values[i], shown[i] = d.wpm, d.count > 0
	}

	lines, width := columnChart(values, shown, 0)
	first, last := days[0].date.Format("01-02"), days[len(days)-1].date.Format("01-02")

	return append(lines, axisLabels(width, len(days), first, last))
}

// columnChart draws a column for each value, the shown ones only, above a
// horizontal axis. The scale goes from the ten below the lowest shown value to
// the ten above the highest one so that small differences remain visible.
// Labels take at least minWidth cells, the width of the labels is returned
// with the lines.
func columnChart(values []float64, shown []bool, minWidth int) (lines []string, width int) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, v := range values {
		if shown[i] {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}

	lo, hi = math.Floor(lo/10)*10, math.Ceil(hi/10)*10
	if hi == lo {
		hi += 10
	}

	//Height of each column in eighths of a row, shown values are always visible.
	heights := make([]int, len(values))
	for i, v := range values {
		if shown[i] {
			heights[i] = int(math.Round((v - lo) / (hi - lo) * chartRows * 8))
			if heights[i] < 1 {
				heights[i] = 1
			}
		}
	}

	hiLabel, loLabel := fmt.Sprintf("%.0f", hi), fmt.Sprintf("%.0f", lo)
	width = len(hiLabel)
	if width < minWidth {
		width = minWidth
	}

	for row := chartRows - 1; row >= 0; row-- {
		label := ""
		switch row {
		case chartRows - 1:
			label = hiLabel
		case 0:
			label = loLabel
		}

		cols := make([]rune, len(values))
		for i, h := range heights {
			cols[i] = chartBlocks[clamp(h-row*8, 0, 8)]
		}

		lines = append(lines, strings.TrimRight(fmt.Sprintf("%*s │%s", width, label, string(cols)), " "))
	}

	lines = append(lines, strings.Repeat(" ", width)+" └"+strings.Repeat("─", len(values)))

	return lines, width
}

// axisLabels places first under the first of n columns of a chart whose labels
// are width cells wide, and last under the last one. They are kept apart when
// the chart is too narrow.
func axisLabels(width, n int, first, last string) string {
	gap := n - len(first) - len(last)
	if gap < 1 {
		gap = 1
	}

	return strings.Repeat(" ", width+2) + first + strings.Repeat(" ", gap) + last
}

// clamp returns v limited to [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}

	return v
}
