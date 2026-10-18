package main

import (
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// keyStatsOf is a store of the given key statistics, which ignores new
// keystrokes.
type keyStatsOf keyStats

func (k keyStatsOf) KeyStats() keyStats              { return keyStats(k) }
func (keyStatsOf) AddKeystrokes(_ []keystroke) error { return nil }

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

	// The average error rate is 0.1 and the average latency 166.67ms. The
	// keystrokes of each pair (20 for th and TH) are completed by 20 at the
	// averages.
	errTH, latencyTH := (0.3*20+0.1*20)/40, (300*20+500.0/3*20)/40
	errHE, latencyHE := (0*10+0.1*20)/30, (100*10+500.0/3*20)/30
	want := map[string]float64{
		"th": (errTH+errorFloor)/(0.1+errorFloor) + latencyTH/(500.0/3),
		"he": (errHE+errorFloor)/(0.1+errorFloor) + latencyHE/(500.0/3),
		"er": (errHE+errorFloor)/(0.1+errorFloor) + latencyHE/(500.0/3),
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

func TestNewWeaknessesTrustsPairsWithMoreKeystrokes(t *testing.T) {
	ks := newKeyStats()
	ks.Bigrams["ab"] = keyStat{Count: 6, ErrorRate: 1.0 / 6, Timed: 6, Latency: 200}
	ks.Bigrams["cd"] = keyStat{Count: 100, ErrorRate: 1.0 / 6, Timed: 100, Latency: 200}
	ks.Bigrams["ef"] = keyStat{Count: 100, ErrorRate: 0, Timed: 100, Latency: 100}

	// A single mistake in a few keystrokes may be bad luck.
	w := newWeaknesses(ks)
	if w["ab"] >= w["cd"] {
		t.Errorf("weaknesses = %v, want ab, typed as badly as cd but less often, to be judged less weak", w)
	}
}

func TestNewWeaknessesOfRecentKeystrokes(t *testing.T) {
	ks := newKeyStats()
	ks.Bigrams["ab"] = keyStat{Count: recentKeystrokes, ErrorRate: 0.2, Timed: recentKeystrokes, Latency: 200}
	ks.Bigrams["cd"] = keyStat{Count: 1000, ErrorRate: 0.2, Timed: 1000, Latency: 200}
	ks.Bigrams["ef"] = keyStat{Count: 100, ErrorRate: 0, Timed: 100, Latency: 100}

	// The rates are recent averages: older keystrokes don't make them surer.
	w := newWeaknesses(ks)
	if !near(w["ab"], w["cd"]) {
		t.Errorf("weaknesses = %v, want ab and cd to be as weak", w)
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

func TestPairFrequencies(t *testing.T) {
	got := pairFrequencies([]string{"the", "There", "a", "can't"})

	// th, he, th, he, er, re, ca, an: the apostrophe isn't a letter.
	want := map[string]float64{"th": 0.25, "he": 0.25, "er": 0.125, "re": 0.125, "ca": 0.125, "an": 0.125}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pairFrequencies() = %v, want %v", got, want)
	}
}

func TestWeaknessFocus(t *testing.T) {
	w := newWeaknesses(weakTH())
	w["er"] = 3
	w["xy"] = 5
	frequency := map[string]float64{"th": 0.01, "he": 0.1, "er": 0.05}

	// er is less weak than th but much more frequent, he is typed well and xy
	// is never typed.
	if got := w.focus(3, frequency); !reflect.DeepEqual(got, []string{"er", "th"}) {
		t.Errorf("focus(3) = %q, want the weak pairs costing the most time first", got)
	}
	if got := w.focus(1, frequency); !reflect.DeepEqual(got, []string{"er"}) {
		t.Errorf("focus(1) = %q, want [er]", got)
	}
}

func TestAdaptiveWordsFocusOnFrequentWeakPairs(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "in into begin rain think during point inside soccer here")

	// cc, rare in the list, was typed a few times with a mistake and slowly,
	// in, frequent, was typed often with mistakes and a bit slowly.
	ks := newKeyStats()
	ks.Bigrams["cc"] = keyStat{Count: 7, ErrorRate: 1.0 / 7, Timed: 6, Latency: 165}
	ks.Bigrams["in"] = keyStat{Count: 190, ErrorRate: 0.1, Timed: 185, Latency: 120}
	ks.Bigrams["er"] = keyStat{Count: 230, ErrorRate: 0, Timed: 230, Latency: 100}
	ks.Bigrams["he"] = keyStat{Count: 96, ErrorRate: 0, Timed: 95, Latency: 71}

	a, err := loadAdaptiveWords("list")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.focus(ks); len(got) == 0 || got[0] != "in" {
		t.Errorf("focus = %q, want in first", got)
	}
}

func TestAdaptiveWordsDescribe(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	a, err := loadAdaptiveWords("list")
	if err != nil {
		t.Fatal(err)
	}

	if got := a.describe(weakTH()); got != "Practicing th" {
		t.Errorf("describe() = %q", got)
	}
	if got := a.describe(newKeyStats()); got != "Practicing all keys until enough keystrokes are recorded" {
		t.Errorf("describe() without statistics = %q", got)
	}
}

func TestGenerateAdaptiveTestFavoursWeakWords(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	got := drawShares(t, mustGen(generateAdaptiveTest("list", wordOptions{n: 100, g: 4}, 0, keyStatsOf(weakTH()))))

	// Uniformly drawn every word would make a quarter of the test.
	for _, w := range []string{"her", "err", "rare"} {
		if got["the"] < 0.3 || got["the"] <= got[w] {
			t.Errorf("shares %v, want the word with the weak pair to come up most", got)
		}
	}
}

func TestGenerateAdaptiveTestWithoutStatistics(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")

	got := drawShares(t, mustGen(generateAdaptiveTest("list", wordOptions{n: 100, g: 4}, 40, keyStatsOf(newKeyStats()))))

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

	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the this that them then ab cd fg ij kl mn op qu vw xz ac bd ce df eg")

	share := 0.0
	for w, s := range drawShares(t, mustGen(generateAdaptiveTest("list", wordOptions{n: 100, g: 4}, focus, keyStatsOf(weakTH())))) {
		if strings.Contains(w, "th") {
			share += s
		}
	}

	return share
}

func TestGenerateAdaptiveTestFocus(t *testing.T) {
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

	if got := thShare(t, 100); !near(got, 1) {
		t.Errorf("share of the words with th with focus 100 = %.2f, want all of them", got)
	}
}

func TestGenerateAdaptiveTestFocusOnASingleWord(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the ab cd")

	done := make(chan []segment, 1)
	go func() {
		done <- mustGen(generateAdaptiveTest("list", wordOptions{n: 4, g: 1}, 100, keyStatsOf(weakTH())))()
	}()

	select {
	case got := <-done:
		if want := []segment{{"the the the the", ""}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("the test was not generated")
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
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")
	st := newTestStore(t)
	if err := writeValue(st.path(keyStatsFile), weakTH()); err != nil {
		t.Fatal(err)
	}

	sim := newSimScreen(t)
	a, err := newApp(sim, parseFlags([]string{"-adaptive", "-words", "list", "-noreport"}), testKind{mode: "adaptive", source: "list"}, st)
	if err != nil {
		t.Fatal(err)
	}
	a.typer.tty = io.Discard

	var calls int
	done := startApp(a, testSequence(&calls, "ab"))
	waitForScreenText(t, sim, "Practicing th", true)

	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)
}

// The word list of adaptive tests is read by both the generator and the app.
func TestAdaptiveWordListFromStdin(t *testing.T) {
	useTempStorage(t)
	useStdin(t, "alpha beta gamma delta")
	opts := parseFlags([]string{"-adaptive", "-words", "-"})

	if _, _, err := newTestGenerator(opts, strings.NewReader(""), true, newTestStore(t)); err != nil {
		t.Fatalf("generator: %v", err)
	}
	if _, err := loadAdaptiveWords(adaptiveWordList(opts)); err != nil {
		t.Errorf("loading the word list a second time: %v", err)
	}
}
