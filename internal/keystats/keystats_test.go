package keystats

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
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
	ks := New()
	ks.Add([]typing.Keystroke{
		{Expected: 'a', Typed: 'a', At: 0},
		{Expected: 'b', Typed: 'b', At: 100 * ms},
		{Expected: 'c', Typed: 'x', At: 300 * ms},
		{Expected: 'd', Typed: 'd', At: 400 * ms}, // After a mistake: no pair, no latency.
		{},
		{Expected: 'e', Typed: 'e', At: 2 * time.Second}, // After a break: no pair, no latency.
		{Expected: 'f', Typed: 'f', At: 4500 * ms},       // After a pause: no latency.
	})

	wantChars := map[string]Stat{
		"a": {Count: 1},
		"b": {Count: 1, Timed: 1, Latency: 100},
		"c": {Count: 1, ErrorRate: 1},
		"d": {Count: 1},
		"e": {Count: 1},
		"f": {Count: 1},
	}
	wantBigrams := map[string]Stat{
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

	ks.Add([]typing.Keystroke{{Expected: 'a', Typed: 'a', At: 0}, {Expected: 'b', Typed: 'b', At: 300 * ms}, {Expected: 'c', Typed: 'c', At: 400 * ms}})

	if got := ks.Chars["b"]; got != (Stat{Count: 2, Timed: 2, Latency: 200}) {
		t.Errorf("b after two tests = %+v, want an average latency of 200ms", got)
	}
	if got := ks.Chars["c"]; got != (Stat{Count: 2, ErrorRate: 0.5, Timed: 1, Latency: 100}) {
		t.Errorf("c after two tests = %+v, want a 50%% error rate", got)
	}
}

func TestMergeKeyStats(t *testing.T) {
	a := Stat{Count: 30, ErrorRate: 0.25, Timed: 10, Latency: 100}
	b := Stat{Count: 10, ErrorRate: 0.75, Timed: 30, Latency: 200}

	want := Stat{Count: 40, ErrorRate: 0.375, Timed: 40, Latency: 175}
	if got := Merge(a, b); got != want {
		t.Errorf("Merge() = %+v, want %+v", got, want)
	}

	// Merging with nothing keeps the values exactly, 0.1*3/3 would not.
	c := Stat{Count: 3, ErrorRate: 0.1, Timed: 3, Latency: 0.1}
	if got := Merge(c, Stat{}); got != c {
		t.Errorf("merging with nothing = %+v, want %+v", got, c)
	}
	if got := Merge(Stat{}, c); got != c {
		t.Errorf("merging nothing = %+v, want %+v", got, c)
	}
}
