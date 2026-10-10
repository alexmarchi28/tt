package main

import (
	"reflect"
	"testing"
	"time"
)

// statsNow is just after midnight in a time zone where the date differs from
// most others, so that dates must be shown in the time zone of now.
var statsNow = time.Date(2026, 10, 10, 1, 0, 0, 0, time.FixedZone("UTC+14", 14*60*60))

func resultAt(wpm int, accuracy float64, ago time.Duration) persistedResult {
	return persistedResult{Wpm: wpm, Accuracy: accuracy, Timestamp: statsNow.Add(-ago).Unix()}
}

func TestFormatSummaryLine(t *testing.T) {
	stored := []persistedResult{
		resultAt(60, 90, 30*time.Minute),
		resultAt(41, 99, 2*time.Hour),
		resultAt(10, 50, 30*24*time.Hour),
	}

	tests := []struct {
		label  string
		window time.Duration
		want   string
	}{
		{"Last hour", time.Hour, "Last hour  avg wpm 60.00, avg accuracy 90.00% (1 tests)"},
		{"Last day", 24 * time.Hour, "Last day   avg wpm 50.50, avg accuracy 94.50% (2 tests)"},
		{"Last week", 7 * 24 * time.Hour, "Last week  avg wpm 50.50, avg accuracy 94.50% (2 tests)"},
		{"Last min", time.Minute, "Last min   no results"},
	}

	for _, tc := range tests {
		got := formatSummaryLine(tc.label, summarize(since(stored, statsNow.Add(-tc.window))))
		if got != tc.want {
			t.Errorf("%s\n got %q\nwant %q", tc.label, got, tc.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	if got := summarize(nil); got != (summary{}) {
		t.Errorf("summarize(nil) = %+v, want the zero summary", got)
	}

	best := resultAt(80, 95, 2*time.Hour)
	got := summarize([]persistedResult{resultAt(60, 90, time.Hour), best, resultAt(80, 100, 3*time.Hour)})
	want := summary{count: 3, wpm: 220.0 / 3, accuracy: 95, best: best}
	if got != want {
		t.Errorf("summarize() = %+v, want %+v (the first of equal bests)", got, want)
	}
}

func TestSinceAndBefore(t *testing.T) {
	a, b, c := resultAt(1, 0, 3*time.Hour), resultAt(2, 0, 2*time.Hour), resultAt(3, 0, time.Hour)
	stored := []persistedResult{a, b, c}

	if got := since(stored, statsNow.Add(-2*time.Hour)); !reflect.DeepEqual(got, []persistedResult{b, c}) {
		t.Errorf("since() = %+v, want the last two results", got)
	}
	if got := before(stored, statsNow.Add(-2*time.Hour)); !reflect.DeepEqual(got, []persistedResult{a}) {
		t.Errorf("before() = %+v, want the first result", got)
	}
}

func TestMostMistyped(t *testing.T) {
	mistakes := []mistake{
		{"the", "teh"}, {"zebra", "zerba"}, {"because", "becuase"}, {"the", "th_"},
		{"a", "s"}, {"because", "bec"}, {"the", "hte"}, {"a", "q"},
	}

	want := []wordCount{{"the", 3}, {"a", 2}, {"because", 2}, {"zebra", 1}}
	if got := mostMistyped(mistakes, 10); !reflect.DeepEqual(got, want) {
		t.Errorf("mostMistyped() = %v, want %v", got, want)
	}

	if got := mostMistyped(mistakes, 2); !reflect.DeepEqual(got, want[:2]) {
		t.Errorf("mostMistyped(2) = %v, want %v", got, want[:2])
	}

	if got := mostMistyped(nil, 10); len(got) != 0 {
		t.Errorf("mostMistyped(nil) = %v, want nothing", got)
	}
}

func TestFormatStats(t *testing.T) {
	day := 24 * time.Hour
	stored := []persistedResult{
		resultAt(20, 70, 40*day),
		resultAt(30, 80, 10*day),
		resultAt(80, 95, 3*day),
		resultAt(41, 99, 2*time.Hour),
		resultAt(60, 90, 30*time.Minute),
	}
	mistakes := []mistake{{"the", "teh"}, {"because", "becuase"}, {"the", "th_"}, {"the", "hte"}}

	want := "" +
		"Typing stats\n" +
		"Tests      5\n" +
		"Best       80 wpm (2026-10-07)\n" +
		"Last hour  avg wpm 60.00, avg accuracy 90.00% (1 tests)\n" +
		"Last day   avg wpm 50.50, avg accuracy 94.50% (2 tests)\n" +
		"Last week  avg wpm 60.33, avg accuracy 94.67% (3 tests)\n" +
		"All time   avg wpm 46.20, avg accuracy 86.80% (5 tests)\n" +
		"Trend      +30.33 wpm compared to the previous week\n" +
		"\n" +
		"Most mistyped words\n" +
		"the        3\n" +
		"because    1\n"

	if got := formatStats(stored, mistakes, statsNow); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFormatStatsWithoutHistory(t *testing.T) {
	stored := []persistedResult{resultAt(50, 100, time.Minute)}

	want := "" +
		"Typing stats\n" +
		"Tests      1\n" +
		"Best       50 wpm (2026-10-10)\n" +
		"Last hour  avg wpm 50.00, avg accuracy 100.00% (1 tests)\n" +
		"Last day   avg wpm 50.00, avg accuracy 100.00% (1 tests)\n" +
		"Last week  avg wpm 50.00, avg accuracy 100.00% (1 tests)\n" +
		"All time   avg wpm 50.00, avg accuracy 100.00% (1 tests)\n"

	if got := formatStats(stored, nil, statsNow); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
