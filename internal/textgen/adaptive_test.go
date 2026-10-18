package textgen

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

// keyStatsOf is a store of the given key statistics, which ignores new
// keystrokes.
type keyStatsOf keystats.Stats

func (k keyStatsOf) KeyStats() keystats.Stats { return keystats.Stats(k) }

func (keyStatsOf) AddKeystrokes(_ []typing.Keystroke) error { return nil }

// weakTH returns statistics where "th" is typed badly and "he" and "er" well.
func weakTH() keystats.Stats {
	ks := keystats.New()
	ks.Bigrams["th"] = keystats.Stat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["TH"] = keystats.Stat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["he"] = keystats.Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["er"] = keystats.Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["e "] = keystats.Stat{Count: 10, ErrorRate: 1, Timed: 10, Latency: 900} // Not a pair of letters.
	ks.Bigrams["zz"] = keystats.Stat{Count: 2, ErrorRate: 1, Timed: 2, Latency: 900}   // Not enough keystrokes.
	return ks
}

func TestAdaptiveWordsFocusOnFrequentWeakPairs(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "in into begin rain think during point inside soccer here")

	// cc, rare in the list, was typed a few times with a mistake and slowly,
	// in, frequent, was typed often with mistakes and a bit slowly.
	ks := keystats.New()
	ks.Bigrams["cc"] = keystats.Stat{Count: 7, ErrorRate: 1.0 / 7, Timed: 6, Latency: 165}
	ks.Bigrams["in"] = keystats.Stat{Count: 190, ErrorRate: 0.1, Timed: 185, Latency: 120}
	ks.Bigrams["er"] = keystats.Stat{Count: 230, ErrorRate: 0, Timed: 230, Latency: 100}
	ks.Bigrams["he"] = keystats.Stat{Count: 96, ErrorRate: 0, Timed: 95, Latency: 71}

	a, err := loadAdaptiveWords(res, "list")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.focus(ks); len(got) == 0 || got[0] != "in" {
		t.Errorf("focus = %q, want in first", got)
	}
}

func TestAdaptiveWordsDescribe(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	a, err := loadAdaptiveWords(res, "list")
	if err != nil {
		t.Fatal(err)
	}

	if got := a.describe(weakTH()); got != "Practicing th" {
		t.Errorf("describe() = %q", got)
	}
	if got := a.describe(keystats.New()); got != "Practicing all keys until enough keystrokes are recorded" {
		t.Errorf("describe() without statistics = %q", got)
	}
}

func TestGenerateAdaptiveTestFavoursWeakWords(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	got := drawShares(t, nextFunc(t, mustSource(NewAdaptive(newRand(), res, "list", WordOptions{N: 100, G: 4}, 0, keyStatsOf(weakTH())))))

	// Uniformly drawn every word would make a quarter of the test.
	for _, w := range []string{"her", "err", "rare"} {
		if got["the"] < 0.3 || got["the"] <= got[w] {
			t.Errorf("shares %v, want the word with the weak pair to come up most", got)
		}
	}
}

func TestGenerateAdaptiveTestWithoutStatistics(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	got := drawShares(t, nextFunc(t, mustSource(NewAdaptive(newRand(), res, "list", WordOptions{N: 100, G: 4}, 40, keyStatsOf(keystats.New())))))

	for _, w := range []string{"the", "her", "err", "rare"} {
		if got[w] < 0.18 || got[w] > 0.32 {
			t.Errorf("shares %v, want words drawn uniformly without statistics", got)
		}
	}
}

// thShare returns the share of the words containing th in 10 tests of
// adaptive words, practicing th with the given focus.
func thShare(t *testing.T, focus int) float64 {
	t.Helper()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the this that them then ab cd fg ij kl mn op qu vw xz ac bd ce df eg")

	share := 0.0
	for w, s := range drawShares(t, nextFunc(t, mustSource(NewAdaptive(newRand(), res, "list", WordOptions{N: 100, G: 4}, focus, keyStatsOf(weakTH()))))) {
		if strings.Contains(w, "th") {
			share += s
		}
	}

	return share
}

func TestGenerateAdaptiveTestFocus(t *testing.T) {
	t.Parallel()

	// Without focus the words with th are only favoured.
	share := thShare(t, 0)
	if share < 0.3 || share > 0.5 {
		t.Errorf("share of the words with th without focus = %.2f, want about 0.4", share)
	}

	// With focus 40, 40% of the words have th and the others are drawn as
	// without focus.
	want := 0.4 + 0.6*share
	if got := thShare(t, 40); math.Abs(got-want) > 0.05 {
		t.Errorf("share of the words with th with focus 40 = %.2f, want %.2f", got, want)
	}

	if got := thShare(t, 100); math.Abs(got-1) > 0.001 {
		t.Errorf("share of the words with th with focus 100 = %.2f, want all of them", got)
	}
}

func TestGenerateAdaptiveTestFocusOnASingleWord(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the ab cd")

	src := mustSource(NewAdaptive(newRand(), res, "list", WordOptions{N: 4, G: 1}, 100, keyStatsOf(weakTH())))
	done := make(chan []Segment, 1)
	go func() {
		test, _ := src.Next()
		done <- test
	}()

	select {
	case got := <-done:
		if want := []Segment{{"the the the the", ""}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("the test was not generated")
	}
}

// drawShares returns the share of each word in 10 tests made by gen.
func drawShares(t *testing.T, gen func() []Segment) map[string]float64 {
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
