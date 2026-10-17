package main

import "time"

const (
	keyWeight  = 0.05            // Smallest weight of the latest keystroke in recent averages.
	maxLatency = 2 * time.Second // Longer intervals between keystrokes are pauses.

	// recentKeystrokes is how many keystrokes a recent average is as reliable
	// as, however many were typed: that of a plain average with the same
	// variance.
	recentKeystrokes = int(2/keyWeight - 1)
)

// keyStat describes how a character, or a pair of consecutive characters, is
// typed. Its averages favour the latest keystrokes so that they follow the
// progress of the typist.
type keyStat struct {
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

func (s *keyStat) addKeystroke(wrong bool) {
	x := 0.0
	if wrong {
		x = 1
	}

	s.ErrorRate = recentAverage(s.ErrorRate, s.Count, x)
	s.Count++
}

func (s *keyStat) addLatency(d time.Duration) {
	s.Latency = recentAverage(s.Latency, s.Timed, float64(d)/float64(time.Millisecond))
	s.Timed++
}

// keyStats describes how each character and pair of characters is typed.
type keyStats struct {
	Chars   map[string]keyStat `json:"chars"`
	Bigrams map[string]keyStat `json:"bigrams"`
}

func newKeyStats() keyStats {
	return keyStats{map[string]keyStat{}, map[string]keyStat{}}
}

// add updates the statistics with the keystrokes of a test. A pair is only
// counted after a correct keystroke, and the latency of a keystroke only if
// both it and the previous one are correct.
func (ks keyStats) add(strokes []keystroke) {
	var prev keystroke

	for _, k := range strokes {
		if k.expected == 0 {
			prev = keystroke{}
			continue
		}

		wrong := k.typed != k.expected
		char := ks.Chars[string(k.expected)]
		char.addKeystroke(wrong)

		if prev.expected != 0 && prev.typed == prev.expected {
			pair := string([]rune{prev.expected, k.expected})
			bigram := ks.Bigrams[pair]
			bigram.addKeystroke(wrong)

			if d := k.at - prev.at; !wrong && d > 0 && d <= maxLatency {
				char.addLatency(d)
				bigram.addLatency(d)
			}

			ks.Bigrams[pair] = bigram
		}

		ks.Chars[string(k.expected)] = char
		prev = k
	}
}
