package main

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"unicode"
)

const (
	minPairCount    = 5    // Keystrokes needed to rate a pair of letters.
	errorFloor      = 0.02 // Added to error rates so that tiny ones aren't overrated.
	adaptivePower   = 2    // How strongly words with weak pairs of letters are favoured.
	averageWeakness = 2    // Weakness of an average pair of letters, see newWeaknesses.
)

// weaknesses rates how badly each pair of letters is typed, as the sum of its
// error rate and latency relative to the average ones: 2 is the average and
// higher is worse.
type weaknesses map[string]float64

// newWeaknesses rates the pairs of letters with enough keystrokes, ignoring
// case.
func newWeaknesses(ks keyStats) weaknesses {
	pairs := map[string]keyStat{}
	for pair, s := range ks.Bigrams {
		r := []rune(pair)
		if len(r) == 2 && unicode.IsLetter(r[0]) && unicode.IsLetter(r[1]) {
			pair = strings.ToLower(pair)
			pairs[pair] = mergeKeyStats(pairs[pair], s)
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
		if s.Timed >= minKeyTimed {
			latencySum += s.Latency
			timed++
		}
	}

	w := weaknesses{}
	for pair, s := range pairs {
		w[pair] = (s.ErrorRate + errorFloor) / (errorSum/float64(rated) + errorFloor)

		if s.Timed >= minKeyTimed {
			w[pair] += s.Latency / (latencySum / float64(timed))
		} else {
			w[pair]++
		}
	}

	return w
}

// of returns the weakness of a pair of lower case letters, the average one if
// it isn't rated.
func (w weaknesses) of(pair string) float64 {
	if v, ok := w[pair]; ok {
		return v
	}

	return averageWeakness
}

// word returns the weakness of the weakest pair of letters of word, so that
// weak pairs aren't diluted in long words.
func (w weaknesses) word(word string) float64 {
	r := []rune(strings.ToLower(word))
	if len(r) < 2 {
		return averageWeakness
	}

	weakest := 0.0
	for i := 1; i < len(r); i++ {
		weakest = math.Max(weakest, w.of(string(r[i-1:i+1])))
	}

	return weakest
}

// focus returns up to n of the pairs weaker than average, the weakest first.
func (w weaknesses) focus(n int) []string {
	var pairs []string
	for pair, v := range w {
		if v > averageWeakness {
			pairs = append(pairs, pair)
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if w[pairs[i]] != w[pairs[j]] {
			return w[pairs[i]] > w[pairs[j]]
		}
		return pairs[i] < pairs[j]
	})

	if len(pairs) > n {
		pairs = pairs[:n]
	}

	return pairs
}

// adaptiveFocus describes what adaptive tests practice.
func adaptiveFocus(ks keyStats) string {
	pairs := newWeaknesses(ks).focus(3)
	if len(pairs) == 0 {
		return "Practicing all keys until enough keystrokes are recorded"
	}

	return "Practicing " + strings.Join(pairs, ", ")
}

// generateAdaptiveTest draws words from the given word list, favouring those
// with the pairs of letters typed the worst according to the latest key
// statistics.
func generateAdaptiveTest(name string, o wordOptions) (func() []segment, error) {
	words, err := loadWords(name)
	if err != nil {
		return nil, err
	}

	return wordTest(func(n int) []string {
		w := newWeaknesses(loadKeyStats())

		cumulative := make([]float64, len(words))
		total := 0.0
		for i, word := range words {
			total += math.Pow(w.word(word), adaptivePower)
			cumulative[i] = total
		}

		return drawWords(n, words, func() string {
			return words[sort.SearchFloat64s(cumulative, rand.Float64()*total)]
		})
	}, o), nil
}
