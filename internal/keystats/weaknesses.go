package keystats

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	minPairCount    = 5    // Keystrokes needed to rate a pair of letters.
	priorKeystrokes = 20   // Keystrokes at the average rates assumed for every pair, see NewWeaknesses.
	errorFloor      = 0.02 // Added to error rates so that tiny ones aren't overrated.

	// AverageWeakness is the weakness of an average pair of letters, see
	// NewWeaknesses.
	AverageWeakness = 2
)

// Weaknesses rates how badly each pair of letters is typed, as the sum of its
// error rate and latency relative to the average ones: 2 is the average and
// higher is worse. The rates of a pair are drawn towards the averages as if
// it had priorKeystrokes more keystrokes at the average rates, so that pairs
// typed a few times aren't judged on luck.
type Weaknesses map[string]float64

// NewWeaknesses rates the pairs of letters with enough keystrokes, ignoring
// case.
func NewWeaknesses(ks Stats) Weaknesses {
	pairs := map[string]Stat{}
	for pair, s := range ks.Bigrams {
		r := []rune(pair)
		if len(r) == 2 && unicode.IsLetter(r[0]) && unicode.IsLetter(r[1]) {
			pair = strings.ToLower(pair)
			pairs[pair] = Merge(pairs[pair], s)
		}
	}

	var errorSum, latencySum float64
	var rated, timed int
	for pair, s := range pairs {
		if s.Count < minPairCount {
			delete(pairs, pair)
			continue
		}

		errorSum += s.ErrorRate
		rated++
		if s.Timed >= MinTimed {
			latencySum += s.Latency
			timed++
		}
	}

	errorAvg, latencyAvg := errorSum/float64(rated), latencySum/float64(timed)

	w := Weaknesses{}
	for pair, s := range pairs {
		errorRate := towardsAverage(s.ErrorRate, s.Count, errorAvg)
		w[pair] = (errorRate + errorFloor) / (errorAvg + errorFloor)

		if timed > 0 {
			w[pair] += towardsAverage(s.Latency, s.Timed, latencyAvg) / latencyAvg
		} else {
			w[pair]++
		}
	}

	return w
}

// towardsAverage returns a recent average of n values drawn towards the
// average of all pairs, see Weaknesses.
func towardsAverage(recent float64, n int, average float64) float64 {
	if n > recentKeystrokes {
		n = recentKeystrokes
	}

	return (recent*float64(n) + average*priorKeystrokes) / float64(n+priorKeystrokes)
}

// of returns the weakness of a pair of lower case letters, the average one if
// it isn't rated.
func (w Weaknesses) Of(pair string) float64 {
	if v, ok := w[pair]; ok {
		return v
	}

	return AverageWeakness
}

// word returns the weakness of the weakest pair of letters of word, so that
// weak pairs aren't diluted in long words.
func (w Weaknesses) Word(word string) float64 {
	pairs := LetterPairs(word)
	if len(pairs) == 0 {
		return AverageWeakness
	}

	weakest := 0.0
	for _, pair := range pairs {
		weakest = math.Max(weakest, w.Of(pair))
	}

	return weakest
}

// focus returns up to n of the pairs weaker than average, those costing the
// most time first: the weakest and most frequent ones, given the share of
// each pair among the pairs typed.
func (w Weaknesses) Focus(n int, frequency map[string]float64) []string {
	cost := map[string]float64{}
	for pair, v := range w {
		if v > AverageWeakness && frequency[pair] > 0 {
			cost[pair] = (v - AverageWeakness) * frequency[pair]
		}
	}

	pairs := sortedKeys(cost)
	sort.SliceStable(pairs, func(i, j int) bool { return cost[pairs[i]] > cost[pairs[j]] })

	if len(pairs) > n {
		pairs = pairs[:n]
	}

	return pairs
}

// LetterPairs returns the pairs of consecutive letters of word, in lower
// case.
func LetterPairs(word string) []string {
	r := []rune(strings.ToLower(word))

	var pairs []string
	for i := 1; i < len(r); i++ {
		if unicode.IsLetter(r[i-1]) && unicode.IsLetter(r[i]) {
			pairs = append(pairs, string(r[i-1:i+1]))
		}
	}

	return pairs
}

// PairFrequencies returns the share of each pair of letters among the pairs
// of letters of words.
func PairFrequencies(words []string) map[string]float64 {
	counts, total := map[string]float64{}, 0.0
	for _, word := range words {
		for _, pair := range LetterPairs(word) {
			counts[pair]++
			total++
		}
	}

	for pair := range counts {
		counts[pair] /= total
	}

	return counts
}

// sortedKeys returns the keys of m in alphabetical order.
func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
