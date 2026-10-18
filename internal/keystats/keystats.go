// Package keystats describes how each key and pair of keys is typed, and
// rates the weaknesses of the typist.
package keystats

import (
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
)

const (
	// MinTimed is the number of timed keystrokes needed to judge the speed of
	// a key or pair of keys.
	MinTimed = 5

	keyWeight  = 0.05            // Smallest weight of the latest keystroke in recent averages.
	maxLatency = 2 * time.Second // Longer intervals between keystrokes are pauses.

	// recentKeystrokes is how many keystrokes a recent average is as reliable
	// as, however many were typed: that of a plain average with the same
	// variance.
	recentKeystrokes = int(2/keyWeight - 1)
)

// Stat describes how a character, or a pair of consecutive characters, is
// typed. Its averages favour the latest keystrokes so that they follow the
// progress of the typist.
type Stat struct {
	Count     int     `json:"count"`   // Keystrokes.
	ErrorRate float64 `json:"errors"`  // Recent share of wrong keystrokes.
	Timed     int     `json:"timed"`   // Keystrokes whose latency is known.
	Latency   float64 `json:"latency"` // Recent average time since the previous keystroke, in milliseconds.
}

// recentAverage adds x to avg, the average of n values. The result is a plain
// average at first, then the weight of older values decays.
func recentAverage(avg float64, n int, x float64) float64 {
	w := 1 / float64(n+1)
	if w < keyWeight {
		w = keyWeight
	}

	return avg + (x-avg)*w
}

func (s *Stat) addKeystroke(wrong bool) {
	x := 0.0
	if wrong {
		x = 1
	}

	s.ErrorRate = recentAverage(s.ErrorRate, s.Count, x)
	s.Count++
}

func (s *Stat) addLatency(d time.Duration) {
	s.Latency = recentAverage(s.Latency, s.Timed, float64(d)/float64(time.Millisecond))
	s.Timed++
}

// Stats describes how each character and pair of characters is typed.
type Stats struct {
	Chars   map[string]Stat `json:"chars"`
	Bigrams map[string]Stat `json:"bigrams"`
}

func New() Stats {
	return Stats{map[string]Stat{}, map[string]Stat{}}
}

// add updates the statistics with the keystrokes of a test. A pair is only
// counted after a correct keystroke, and the latency of a keystroke only if
// both it and the previous one are correct.
func (ks Stats) Add(strokes []typing.Keystroke) {
	var prev typing.Keystroke

	for _, k := range strokes {
		if k.Expected == 0 {
			prev = typing.Keystroke{}
			continue
		}

		wrong := k.Typed != k.Expected
		char := ks.Chars[string(k.Expected)]
		char.addKeystroke(wrong)

		if prev.Expected != 0 && prev.Typed == prev.Expected {
			pair := string([]rune{prev.Expected, k.Expected})
			bigram := ks.Bigrams[pair]
			bigram.addKeystroke(wrong)

			if d := k.At - prev.At; !wrong && d > 0 && d <= maxLatency {
				char.addLatency(d)
				bigram.addLatency(d)
			}

			ks.Bigrams[pair] = bigram
		}

		ks.Chars[string(k.Expected)] = char
		prev = k
	}
}

// Merge combines the statistics of two sets of keystrokes.
func Merge(a, b Stat) Stat {
	if a.Count == 0 && a.Timed == 0 {
		return b
	}
	if b.Count == 0 && b.Timed == 0 {
		return a
	}

	m := Stat{Count: a.Count + b.Count, Timed: a.Timed + b.Timed}
	if m.Count > 0 {
		m.ErrorRate = (a.ErrorRate*float64(a.Count) + b.ErrorRate*float64(b.Count)) / float64(m.Count)
	}
	if m.Timed > 0 {
		m.Latency = (a.Latency*float64(a.Timed) + b.Latency*float64(b.Timed)) / float64(m.Timed)
	}

	return m
}
