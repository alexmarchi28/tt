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
		"Progress (average wpm per day)\n" +
		"80 │                          █\n" +
		"   │                          █\n" +
		"   │                          █\n" +
		"   │                          █  ▆\n" +
		"   │                          █  █\n" +
		"   │                          █  █\n" +
		"   │                          █ ▆█\n" +
		"30 │                   ▁      █ ██\n" +
		"   └──────────────────────────────\n" +
		"    09-11                    10-10\n" +
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

func TestFormatTypingTime(t *testing.T) {
	tests := map[time.Duration]string{
		45 * time.Second:                             "45s",
		12*time.Minute + 5*time.Second:               "12m 05s",
		3*time.Hour + 2*time.Minute + 59*time.Second: "3h 02m",
	}

	for d, want := range tests {
		if got := formatTypingTime(d); got != want {
			t.Errorf("formatTypingTime(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestSummarizeByKind(t *testing.T) {
	quote := persistedResult{Wpm: 50, Accuracy: 90, Mode: "quotes", Source: "en"}
	words := persistedResult{Wpm: 70, Accuracy: 100, Mode: "words", Source: "1000en", Words: 50}
	timed := persistedResult{Wpm: 80, Accuracy: 100, Mode: "words", Source: "1000en", Seconds: 30}
	old := persistedResult{Wpm: 40, Accuracy: 80}

	got := summarizeByKind([]persistedResult{old, quote, words, timed, words, quote, words})

	want := []kindSummary{
		{testKind{mode: "words", source: "1000en", words: 50}, summarize([]persistedResult{words, words, words})},
		{testKind{mode: "quotes", source: "en"}, summarize([]persistedResult{quote, quote})},
		{testKind{}, summarize([]persistedResult{old})},
		{testKind{mode: "words", source: "1000en", seconds: 30}, summarize([]persistedResult{timed})},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("summarizeByKind() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestFormatStatsByTest(t *testing.T) {
	stored := []persistedResult{
		{Wpm: 40, Accuracy: 80, Timestamp: statsNow.Add(-time.Hour).Unix()},
		{Wpm: 70, Accuracy: 100, Timestamp: statsNow.Add(-time.Minute).Unix(), Mode: "words", Source: "1000en", Duration: 30, Words: 50},
		{Wpm: 50, Accuracy: 90, Timestamp: statsNow.Add(-time.Minute).Unix(), Mode: "quotes", Source: "en", Duration: 45.5},
		{Wpm: 60, Accuracy: 96, Timestamp: statsNow.Add(-time.Minute).Unix(), Mode: "words", Source: "1000en", Duration: 20, Words: 50},
		{Wpm: 66, Accuracy: 90, Timestamp: statsNow.Add(-time.Minute).Unix(), Mode: "words", Source: "1000en", Duration: 15, Seconds: 15, Punct: true},
	}

	want := "" +
		"Typing stats\n" +
		"Tests      5 (1m 50s of typing)\n" +
		"Best       70 wpm (2026-10-10, words 1000en, 50 words)\n" +
		"Last hour  avg wpm 57.20, avg accuracy 91.20% (5 tests)\n" +
		"Last day   avg wpm 57.20, avg accuracy 91.20% (5 tests)\n" +
		"Last week  avg wpm 57.20, avg accuracy 91.20% (5 tests)\n" +
		"All time   avg wpm 57.20, avg accuracy 91.20% (5 tests)\n" +
		"\n" +
		"By test\n" +
		"words 1000en, 50 words          avg wpm 65.00, avg accuracy 98.00% (2 tests), best 70 wpm\n" +
		"quotes en                       avg wpm 50.00, avg accuracy 90.00% (1 tests), best 50 wpm\n" +
		"unknown                         avg wpm 40.00, avg accuracy 80.00% (1 tests), best 40 wpm\n" +
		"words 1000en, 15s, punctuation  avg wpm 66.00, avg accuracy 90.00% (1 tests), best 66 wpm\n"

	if got := formatStats(stored, nil, statsNow); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestBestWpm(t *testing.T) {
	stored := []persistedResult{
		{Wpm: 90},
		{Wpm: 50, Mode: "words", Source: "1000en", Words: 50},
		{Wpm: 70, Mode: "words", Source: "1000en", Words: 50},
		{Wpm: 80, Mode: "words", Source: "200en", Words: 50},
		{Wpm: 85, Mode: "words", Source: "1000en", Words: 10},
		{Wpm: 75, Mode: "words", Source: "1000en", Words: 50, Numbers: true},
		{Wpm: 60, Mode: "words", Source: "1000en", Words: 50},
	}

	if best, ok := bestWpm(stored, testKind{mode: "words", source: "1000en", words: 50}); best != 70 || !ok {
		t.Errorf("bestWpm(words 1000en, 50 words) = %d, %t, want 70, true", best, ok)
	}

	if best, ok := bestWpm(stored, testKind{mode: "quotes", source: "en"}); ok {
		t.Errorf("bestWpm(quotes en) = %d, %t, want no result", best, ok)
	}
}
