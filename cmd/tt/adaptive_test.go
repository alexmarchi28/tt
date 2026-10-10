package main

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

// weakTH returns statistics where "th" is typed badly and "he" and "er" well.
func weakTH() keyStats {
	ks := newKeyStats()
	ks.Bigrams["th"] = keyStat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["TH"] = keyStat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["he"] = keyStat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["er"] = keyStat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["e "] = keyStat{Count: 10, ErrorRate: 1, Timed: 10, Latency: 900} // Not a pair of letters.
	ks.Bigrams["zz"] = keyStat{Count: 2, ErrorRate: 1, Timed: 2, Latency: 900}   // Not enough keystrokes.
	return ks
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestNewWeaknesses(t *testing.T) {
	w := newWeaknesses(weakTH())

	// The average error rate is 0.1 and the average latency 166.67ms.
	want := map[string]float64{
		"th": (0.3+errorFloor)/(0.1+errorFloor) + 300/(500.0/3),
		"he": errorFloor/(0.1+errorFloor) + 100/(500.0/3),
		"er": errorFloor/(0.1+errorFloor) + 100/(500.0/3),
	}

	if len(w) != len(want) {
		t.Errorf("rated pairs = %v, want %v", w, want)
	}
	for pair, v := range want {
		if !near(w[pair], v) {
			t.Errorf("weakness of %q = %v, want %v", pair, w[pair], v)
		}
	}

	if got := w.of("zz"); got != averageWeakness {
		t.Errorf("weakness of an unrated pair = %v, want the average %v", got, averageWeakness)
	}
}

func TestNewWeaknessesWithoutTimings(t *testing.T) {
	ks := newKeyStats()
	ks.Bigrams["ab"] = keyStat{Count: 10, ErrorRate: 0}
	ks.Bigrams["cd"] = keyStat{Count: 10, ErrorRate: 0}

	w := newWeaknesses(ks)
	if !near(w["ab"], averageWeakness) || !near(w["cd"], averageWeakness) {
		t.Errorf("weaknesses = %v, want average ones", w)
	}
}

func TestWordWeakness(t *testing.T) {
	w := newWeaknesses(weakTH())

	if got, want := w.word("the"), w["th"]; !near(got, want) {
		t.Errorf("weakness of the = %v, want that of its weakest pair %v", got, want)
	}
	if got := w.word("hex"); got != averageWeakness {
		t.Errorf("weakness of hex = %v, want %v as its pair ex is not rated", got, averageWeakness)
	}
	if got, want := w.word("The"), w.word("the"); got != want {
		t.Errorf("weakness of The = %v, want that of the %v", got, want)
	}
	for _, word := range []string{"a", "", "xyz"} {
		if got := w.word(word); got != averageWeakness {
			t.Errorf("weakness of %q = %v, want the average", word, got)
		}
	}
}

func TestWeaknessFocus(t *testing.T) {
	w := newWeaknesses(weakTH())
	w["er"] = 3

	if got := w.focus(3); !reflect.DeepEqual(got, []string{"th", "er"}) {
		t.Errorf("focus(3) = %q, want the pairs weaker than average, the weakest first", got)
	}
	if got := w.focus(1); !reflect.DeepEqual(got, []string{"th"}) {
		t.Errorf("focus(1) = %q, want [th]", got)
	}
}

func TestAdaptiveFocus(t *testing.T) {
	if got := adaptiveFocus(weakTH()); got != "Practicing th" {
		t.Errorf("adaptiveFocus() = %q", got)
	}
	if got := adaptiveFocus(newKeyStats()); got != "Practicing all keys until enough keystrokes are recorded" {
		t.Errorf("adaptiveFocus() without statistics = %q", got)
	}
}

func TestGenerateAdaptiveTestFavoursWeakWords(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")
	if err := writeValue(keyStatsDB, weakTH()); err != nil {
		t.Fatal(err)
	}

	got := drawShares(t, mustGen(generateAdaptiveTest("list", wordOptions{n: 100, g: 4})))

	// Uniformly drawn every word would make a quarter of the test.
	if got["the"] < 0.4 {
		t.Errorf("shares %v, want the word with the weak pair to come up most", got)
	}
}

func TestGenerateAdaptiveTestWithoutStatistics(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	got := drawShares(t, mustGen(generateAdaptiveTest("list", wordOptions{n: 100, g: 4})))

	for _, w := range []string{"the", "her", "err", "rare"} {
		if got[w] < 0.18 || got[w] > 0.32 {
			t.Errorf("shares %v, want words drawn uniformly without statistics", got)
		}
	}
}

// drawShares returns the share of each word in 10 tests made by gen.
func drawShares(t *testing.T, gen func() []segment) map[string]float64 {
	t.Helper()

	counts, total := map[string]float64{}, 0.0
	for i := 0; i < 10; i++ {
		for _, s := range gen() {
			for _, w := range strings.Fields(s.Text) {
				counts[w]++
				total++
			}
		}
	}

	for w := range counts {
		counts[w] /= total
	}

	return counts
}

func TestAppShowsAdaptiveFocus(t *testing.T) {
	opts := parseFlags(nil)
	opts.noReport = true
	a, sim := newTestApp(t, opts)
	a.kind = testKind{"adaptive", "list"}
	if err := writeValue(keyStatsDB, weakTH()); err != nil {
		t.Fatal(err)
	}

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	waitForScreenText(t, sim, "Practicing th", true)

	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)
}
