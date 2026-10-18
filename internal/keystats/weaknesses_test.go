package keystats

import (
	"math"
	"reflect"
	"testing"
)

// weakTH returns statistics where "th" is typed badly and "he" and "er" well.
func weakTH() Stats {
	ks := New()
	ks.Bigrams["th"] = Stat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["TH"] = Stat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["he"] = Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["er"] = Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["e "] = Stat{Count: 10, ErrorRate: 1, Timed: 10, Latency: 900} // Not a pair of letters.
	ks.Bigrams["zz"] = Stat{Count: 2, ErrorRate: 1, Timed: 2, Latency: 900}   // Not enough keystrokes.
	return ks
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestNewWeaknesses(t *testing.T) {
	w := NewWeaknesses(weakTH())

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

	if got := w.Of("zz"); got != AverageWeakness {
		t.Errorf("weakness of an unrated pair = %v, want the average %v", got, AverageWeakness)
	}
}

func TestNewWeaknessesTrustsPairsWithMoreKeystrokes(t *testing.T) {
	ks := New()
	ks.Bigrams["ab"] = Stat{Count: 6, ErrorRate: 1.0 / 6, Timed: 6, Latency: 200}
	ks.Bigrams["cd"] = Stat{Count: 100, ErrorRate: 1.0 / 6, Timed: 100, Latency: 200}
	ks.Bigrams["ef"] = Stat{Count: 100, ErrorRate: 0, Timed: 100, Latency: 100}

	// A single mistake in a few keystrokes may be bad luck.
	w := NewWeaknesses(ks)
	if w["ab"] >= w["cd"] {
		t.Errorf("Weaknesses = %v, want ab, typed as badly as cd but less often, to be judged less weak", w)
	}
}

func TestNewWeaknessesOfRecentKeystrokes(t *testing.T) {
	ks := New()
	ks.Bigrams["ab"] = Stat{Count: recentKeystrokes, ErrorRate: 0.2, Timed: recentKeystrokes, Latency: 200}
	ks.Bigrams["cd"] = Stat{Count: 1000, ErrorRate: 0.2, Timed: 1000, Latency: 200}
	ks.Bigrams["ef"] = Stat{Count: 100, ErrorRate: 0, Timed: 100, Latency: 100}

	// The rates are recent averages: older keystrokes don't make them surer.
	w := NewWeaknesses(ks)
	if !near(w["ab"], w["cd"]) {
		t.Errorf("Weaknesses = %v, want ab and cd to be as weak", w)
	}
}

func TestNewWeaknessesWithoutTimings(t *testing.T) {
	ks := New()
	ks.Bigrams["ab"] = Stat{Count: 10, ErrorRate: 0}
	ks.Bigrams["cd"] = Stat{Count: 10, ErrorRate: 0}

	w := NewWeaknesses(ks)
	if !near(w["ab"], AverageWeakness) || !near(w["cd"], AverageWeakness) {
		t.Errorf("Weaknesses = %v, want average ones", w)
	}
}

func TestWordWeakness(t *testing.T) {
	w := NewWeaknesses(weakTH())

	if got, want := w.Word("the"), w["th"]; !near(got, want) {
		t.Errorf("weakness of the = %v, want that of its weakest pair %v", got, want)
	}
	if got := w.Word("hex"); got != AverageWeakness {
		t.Errorf("weakness of hex = %v, want %v as its pair ex is not rated", got, AverageWeakness)
	}
	if got, want := w.Word("The"), w.Word("the"); got != want {
		t.Errorf("weakness of The = %v, want that of the %v", got, want)
	}
	for _, word := range []string{"a", "", "xyz"} {
		if got := w.Word(word); got != AverageWeakness {
			t.Errorf("weakness of %q = %v, want the average", word, got)
		}
	}
}

func TestPairFrequencies(t *testing.T) {
	got := PairFrequencies([]string{"the", "There", "a", "can't"})

	// th, he, th, he, er, re, ca, an: the apostrophe isn't a letter.
	want := map[string]float64{"th": 0.25, "he": 0.25, "er": 0.125, "re": 0.125, "ca": 0.125, "an": 0.125}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairFrequencies() = %v, want %v", got, want)
	}
}

func TestWeaknessFocus(t *testing.T) {
	w := NewWeaknesses(weakTH())
	w["er"] = 3
	w["xy"] = 5
	frequency := map[string]float64{"th": 0.01, "he": 0.1, "er": 0.05}

	// er is less weak than th but much more frequent, he is typed well and xy
	// is never typed.
	if got := w.Focus(3, frequency); !reflect.DeepEqual(got, []string{"er", "th"}) {
		t.Errorf("focus(3) = %q, want the weak pairs costing the most time first", got)
	}
	if got := w.Focus(1, frequency); !reflect.DeepEqual(got, []string{"er"}) {
		t.Errorf("focus(1) = %q, want [er]", got)
	}
}
