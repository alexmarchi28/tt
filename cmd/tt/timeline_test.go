package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

// keysAt returns a correct keystroke at each of the given times in
// milliseconds.
func keysAt(ms ...int) []keystroke {
	keys := make([]keystroke, len(ms))
	for i, m := range ms {
		keys[i] = keystroke{'a', 'a', time.Duration(m) * time.Millisecond}
	}

	return keys
}

func TestTimeline(t *testing.T) {
	keys := []keystroke{
		{'a', 'a', 100 * time.Millisecond},
		{'b', 'x', 900 * time.Millisecond},
		{}, // Breaks aren't keystrokes.
		{'b', 'b', 1500 * time.Millisecond},
		{'c', 'c', 2600 * time.Millisecond},
		{'d', 'e', 2700 * time.Millisecond},
	}

	// The last second is only 700ms long.
	want := []second{
		{keys: 2, errors: 1, length: time.Second},
		{keys: 1, errors: 0, length: time.Second},
		{keys: 2, errors: 1, length: 700 * time.Millisecond},
	}

	if got := timeline(keys, 2700*time.Millisecond); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestTimelineMergesShortLastSecond(t *testing.T) {
	// A last second shorter than half a second is part of the previous one.
	got := timeline(keysAt(500, 1200, 1300), 1300*time.Millisecond)
	want := []second{{keys: 3, length: 1300 * time.Millisecond}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestTimelineWithoutDuration(t *testing.T) {
	if got := timeline(nil, 0); got != nil {
		t.Errorf("got %+v, want no seconds", got)
	}
}

func TestSecondRawWpm(t *testing.T) {
	tests := []struct {
		s    second
		want float64
	}{
		{second{keys: 5, length: time.Second}, 60},
		{second{keys: 5, length: 500 * time.Millisecond}, 120},
		{second{keys: 0, length: time.Second}, 0},
		{second{}, 0},
	}

	for _, tc := range tests {
		if got := tc.s.rawWpm(); got != tc.want {
			t.Errorf("%+v.rawWpm() = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestTestResultRawWpm(t *testing.T) {
	// Wrong keystrokes count towards the raw speed.
	r := testResult{duration: time.Minute, correctChars: 200, correctKeys: 250, wrongKeys: 50}
	if got := r.rawWpm(); got != 60 {
		t.Errorf("rawWpm() = %d, want 60", got)
	}

	if got := (testResult{correctKeys: 10}).rawWpm(); got != 0 {
		t.Errorf("rawWpm() without duration = %d, want 0", got)
	}
}

func TestConsistency(t *testing.T) {
	steady := []second{{keys: 5, length: time.Second}, {keys: 5, length: time.Second}}
	if got := consistency(steady); got != 100 {
		t.Errorf("consistency of a steady speed = %v, want 100", got)
	}

	// 30 and 90 wpm: a coefficient of variation of 0.5.
	uneven := []second{{keys: 3, length: time.Second}, {keys: 9, length: time.Second}}
	want := 100 * (1 - math.Tanh(0.5+math.Pow(0.5, 3)/3+math.Pow(0.5, 5)/5))
	if got := consistency(uneven); math.Abs(got-want) > 1e-9 {
		t.Errorf("consistency of an uneven speed = %v, want %v", got, want)
	}

	if got := consistency([]second{{length: time.Second}}); got != 0 {
		t.Errorf("consistency without keystrokes = %v, want 0", got)
	}
}

func TestTypedSeconds(t *testing.T) {
	if got := typedSeconds(secondsOf(5, 0, 0, 0, 1, 1)); got != 2 {
		t.Errorf("typedSeconds() = %d, want 2 as pauses don't count", got)
	}
}

// secondsOf returns seconds with the given keystrokes and errors, as pairs.
func secondsOf(keysAndErrors ...int) []second {
	var seconds []second
	for i := 0; i < len(keysAndErrors); i += 2 {
		seconds = append(seconds, second{keys: keysAndErrors[i], errors: keysAndErrors[i+1], length: time.Second})
	}

	return seconds
}

func TestSpeedChart(t *testing.T) {
	// 60, 72, 0 (a pause) and 48 wpm, the scale goes from 40 to 80 wpm.
	want := []string{
		" 80 │",
		"    │ ▃",
		"    │ █",
		"    │ █",
		"    │██",
		"    │██",
		"    │██ ▅",
		" 40 │██ █",
		"    └────",
		"err │ 1 3",
		"     0s 4s",
	}

	if got := speedChart(secondsOf(5, 0, 6, 1, 0, 0, 4, 3)); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSpeedChartErrorMarks(t *testing.T) {
	got := speedChart(secondsOf(5, 9, 5, 10))
	if row := got[chartRows+1]; !strings.HasSuffix(row, "│9+") {
		t.Errorf("error row = %q, want counts above 9 shown as +", row)
	}
}

func TestSpeedChartGroupsLongTests(t *testing.T) {
	// 150 seconds fit in 50 columns of 3 seconds.
	seconds := make([]second, 150)
	for i := range seconds {
		seconds[i] = second{keys: 5, length: time.Second}
	}
	seconds[3].errors, seconds[5].errors = 1, 1

	got := speedChart(seconds)
	if axis := got[chartRows]; axis != "    └"+strings.Repeat("─", 50) {
		t.Errorf("axis = %q, want 50 columns", axis)
	}
	if row := got[chartRows+1]; row != "err │ 2" {
		t.Errorf("error row = %q, want the errors of each group of seconds", row)
	}
	if labels := got[chartRows+2]; !strings.HasSuffix(labels, " 150s") {
		t.Errorf("time labels = %q, want the test to end at 150s", labels)
	}
}

func TestSpeedChartShortTestLabels(t *testing.T) {
	got := speedChart(secondsOf(5, 0, 5, 0))
	if labels := got[len(got)-1]; labels != "     0s 2s" {
		t.Errorf("time labels = %q, want them apart when the chart is narrow", labels)
	}
}
