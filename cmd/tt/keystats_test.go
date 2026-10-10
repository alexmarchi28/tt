package main

import (
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestRecentAverage(t *testing.T) {
	avg := 0.0
	for n, x := range []float64{1, 0, 0, 1} {
		avg = recentAverage(avg, n, x)
	}
	if avg != 0.5 {
		t.Errorf("average of the first values = %v, want their plain average 0.5", avg)
	}

	avg = 0
	for n := 0; n < 100; n++ {
		avg = recentAverage(avg, n, 0)
	}
	if avg = recentAverage(avg, 100, 1); math.Abs(avg-keyWeight) > 1e-9 {
		t.Errorf("after many values the latest one weighs %v, want %v", avg, keyWeight)
	}
}

func TestKeyStatsAdd(t *testing.T) {
	ms := time.Millisecond
	ks := newKeyStats()
	ks.add([]keystroke{
		{'a', 'a', 0},
		{'b', 'b', 100 * ms},
		{'c', 'x', 300 * ms},
		{'d', 'd', 400 * ms}, // After a mistake: no pair, no latency.
		{},
		{'e', 'e', 2 * time.Second}, // After a break: no pair, no latency.
		{'f', 'f', 4500 * ms},       // After a pause: no latency.
	})

	wantChars := map[string]keyStat{
		"a": {Count: 1},
		"b": {Count: 1, Timed: 1, Latency: 100},
		"c": {Count: 1, ErrorRate: 1},
		"d": {Count: 1},
		"e": {Count: 1},
		"f": {Count: 1},
	}
	wantBigrams := map[string]keyStat{
		"ab": {Count: 1, Timed: 1, Latency: 100},
		"bc": {Count: 1, ErrorRate: 1},
		"ef": {Count: 1},
	}

	if !reflect.DeepEqual(ks.Chars, wantChars) {
		t.Errorf("chars = %+v\nwant %+v", ks.Chars, wantChars)
	}
	if !reflect.DeepEqual(ks.Bigrams, wantBigrams) {
		t.Errorf("bigrams = %+v\nwant %+v", ks.Bigrams, wantBigrams)
	}

	ks.add([]keystroke{{'a', 'a', 0}, {'b', 'b', 300 * ms}, {'c', 'c', 400 * ms}})

	if got := ks.Chars["b"]; got != (keyStat{Count: 2, Timed: 2, Latency: 200}) {
		t.Errorf("b after two tests = %+v, want an average latency of 200ms", got)
	}
	if got := ks.Chars["c"]; got != (keyStat{Count: 2, ErrorRate: 0.5, Timed: 1, Latency: 100}) {
		t.Errorf("c after two tests = %+v, want a 50%% error rate", got)
	}
}

func TestSaveKeyStats(t *testing.T) {
	useTempStorage(t)

	if got := loadKeyStats(); len(got.Chars) != 0 || len(got.Bigrams) != 0 {
		t.Errorf("loadKeyStats() without a file = %+v, want empty statistics", got)
	}

	saveKeyStats([]keystroke{{'a', 'a', 0}, {'b', 'x', 100 * time.Millisecond}})
	saveKeyStats([]keystroke{{'a', 'a', 0}})

	got := loadKeyStats()
	if got.Chars["a"].Count != 2 || got.Chars["b"].ErrorRate != 1 || got.Bigrams["ab"].Count != 1 {
		t.Errorf("saved statistics = %+v", got)
	}
}

func TestSaveKeyStatsOverCorruptFile(t *testing.T) {
	useTempStorage(t)

	if err := os.WriteFile(keyStatsDB, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	// The statistics are only an aid, they start over rather than stopping tt.
	saveKeyStats([]keystroke{{'a', 'a', 0}})
	if got := loadKeyStats(); got.Chars["a"].Count != 1 {
		t.Errorf("statistics = %+v, want them to start over", got)
	}
}
