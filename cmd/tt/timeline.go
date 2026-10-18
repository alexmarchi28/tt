package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// maxSpeedColumns is the width of the speed chart beyond which seconds are
// grouped.
const maxSpeedColumns = 60

// second is what was typed during a second of a test.
type second struct {
	keys   int           // Keystrokes, breaks excluded.
	errors int           // Wrong keystrokes.
	length time.Duration // Usually a second, the last one can be shorter or longer.
}

// rawWpm returns the speed of all the keystrokes, wrong ones included.
func (s second) rawWpm() float64 {
	return rawWpm(s.keys, s.length)
}

// rawWpm returns the speed of the given number of keystrokes typed in d.
func rawWpm(keys int, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}

	return float64(keys) / 5 / d.Minutes()
}

// timeline splits the keystrokes of a test lasting d into seconds. The last
// second is merged into the previous one when it is shorter than half a
// second, so that a few keystrokes at the very end don't make for a spike.
func timeline(keystrokes []keystroke, d time.Duration) []second {
	n := int(d.Round(time.Second) / time.Second)
	if n == 0 && d > 0 {
		n = 1
	}
	if n == 0 {
		return nil
	}

	seconds := make([]second, n)
	for i := range seconds {
		seconds[i].length = time.Second
	}
	seconds[n-1].length = d - time.Duration(n-1)*time.Second

	for _, k := range keystrokes {
		if k.expected == 0 {
			continue
		}

		i := clamp(int(k.at/time.Second), 0, n-1)
		seconds[i].keys++
		if k.typed != k.expected {
			seconds[i].errors++
		}
	}

	return seconds
}

// typedSeconds returns the number of seconds with keystrokes.
func typedSeconds(seconds []second) int {
	n := 0
	for _, s := range seconds {
		if s.keys > 0 {
			n++
		}
	}

	return n
}

// consistency rates how steady the raw speed is over the seconds, from 0 to
// 100, like Monkeytype: the coefficient of variation is mapped to a percentage
// so that small variations barely count and large ones don't go below 0.
func consistency(seconds []second) float64 {
	mean := 0.0
	for _, s := range seconds {
		mean += s.rawWpm()
	}
	mean /= float64(len(seconds))

	if mean == 0 {
		return 0
	}

	variance := 0.0
	for _, s := range seconds {
		variance += math.Pow(s.rawWpm()-mean, 2)
	}
	cov := math.Sqrt(variance/float64(len(seconds))) / mean

	return 100 * (1 - math.Tanh(cov+math.Pow(cov, 3)/3+math.Pow(cov, 5)/5))
}

// groupSeconds merges consecutive seconds so that there are at most n of them.
func groupSeconds(seconds []second, n int) []second {
	size := (len(seconds) + n - 1) / n
	if size <= 1 {
		return seconds
	}

	var groups []second
	for i := 0; i < len(seconds); i += size {
		end := i + size
		if end > len(seconds) {
			end = len(seconds)
		}

		var g second
		for _, s := range seconds[i:end] {
			g.keys += s.keys
			g.errors += s.errors
			g.length += s.length
		}
		groups = append(groups, g)
	}

	return groups
}

// errorMark shows the number of errors in a column of the speed chart.
func errorMark(errors int) rune {
	switch {
	case errors == 0:
		return ' '
	case errors > 9:
		return '+'
	default:
		return rune('0' + errors)
	}
}

// speedChart draws the raw speed of each second, pauses left blank, with the
// errors of each second below. Long tests are drawn by groups of seconds.
func speedChart(seconds []second) []string {
	const errLabel = "err"

	columns := groupSeconds(seconds, maxSpeedColumns)
	values := make([]float64, len(columns))
	shown := make([]bool, len(columns))
	errors := make([]rune, len(columns))
	var total time.Duration

	for i, c := range columns {
		values[i], shown[i], errors[i] = c.rawWpm(), c.keys > 0, errorMark(c.errors)
		total += c.length
	}

	lines, width := columnChart(values, shown, len(errLabel))
	end := fmt.Sprintf("%.0fs", total.Seconds())

	return append(lines,
		strings.TrimRight(fmt.Sprintf("%*s │%s", width, errLabel, string(errors)), " "),
		axisLabels(width, len(columns), "0s", end))
}
