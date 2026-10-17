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
	priorKeystrokes = 20   // Keystrokes at the average rates assumed for every pair, see newWeaknesses.
	errorFloor      = 0.02 // Added to error rates so that tiny ones aren't overrated.
	adaptivePower   = 2    // How strongly words with weak pairs of letters are favoured.
	averageWeakness = 2    // Weakness of an average pair of letters, see newWeaknesses.
	focusPairs      = 3    // Weak pairs of letters practiced at once.
)

// weaknesses rates how badly each pair of letters is typed, as the sum of its
// error rate and latency relative to the average ones: 2 is the average and
// higher is worse. The rates of a pair are drawn towards the averages as if
// it had priorKeystrokes more keystrokes at the average rates, so that pairs
// typed a few times aren't judged on luck.
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

	errorAvg, latencyAvg := errorSum/float64(rated), latencySum/float64(timed)

	w := weaknesses{}
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
// average of all pairs, see weaknesses.
func towardsAverage(recent float64, n int, average float64) float64 {
	if n > recentKeystrokes {
		n = recentKeystrokes
	}

	return (recent*float64(n) + average*priorKeystrokes) / float64(n+priorKeystrokes)
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
	pairs := letterPairs(word)
	if len(pairs) == 0 {
		return averageWeakness
	}

	weakest := 0.0
	for _, pair := range pairs {
		weakest = math.Max(weakest, w.of(pair))
	}

	return weakest
}

// focus returns up to n of the pairs weaker than average, those costing the
// most time first: the weakest and most frequent ones, given the share of
// each pair among the pairs typed.
func (w weaknesses) focus(n int, frequency map[string]float64) []string {
	cost := map[string]float64{}
	for pair, v := range w {
		if v > averageWeakness && frequency[pair] > 0 {
			cost[pair] = (v - averageWeakness) * frequency[pair]
		}
	}

	pairs := sortedKeys(cost)
	sort.SliceStable(pairs, func(i, j int) bool { return cost[pairs[i]] > cost[pairs[j]] })

	if len(pairs) > n {
		pairs = pairs[:n]
	}

	return pairs
}

// letterPairs returns the pairs of consecutive letters of word, in lower
// case.
func letterPairs(word string) []string {
	r := []rune(strings.ToLower(word))

	var pairs []string
	for i := 1; i < len(r); i++ {
		if unicode.IsLetter(r[i-1]) && unicode.IsLetter(r[i]) {
			pairs = append(pairs, string(r[i-1:i+1]))
		}
	}

	return pairs
}

// pairFrequencies returns the share of each pair of letters among the pairs
// of letters of words.
func pairFrequencies(words []string) map[string]float64 {
	counts, total := map[string]float64{}, 0.0
	for _, word := range words {
		for _, pair := range letterPairs(word) {
			counts[pair]++
			total++
		}
	}

	for pair := range counts {
		counts[pair] /= total
	}

	return counts
}

// adaptiveWords is the word list of adaptive tests.
type adaptiveWords struct {
	words     []string
	frequency map[string]float64 // See pairFrequencies.
}

func loadAdaptiveWords(name string) (adaptiveWords, error) {
	words, err := loadWords(name)
	if err != nil {
		return adaptiveWords{}, err
	}

	return adaptiveWords{words, pairFrequencies(words)}, nil
}

// focus returns the weak pairs of letters to practice according to the
// given key statistics.
func (a adaptiveWords) focus(ks keyStats) []string {
	return newWeaknesses(ks).focus(focusPairs, a.frequency)
}

// describe tells what adaptive tests practice.
func (a adaptiveWords) describe(ks keyStats) string {
	pairs := a.focus(ks)
	if len(pairs) == 0 {
		return "Practicing all keys until enough keystrokes are recorded"
	}

	return "Practicing " + strings.Join(pairs, ", ")
}

// draw draws n words according to the given key statistics: the given
// share of them (from 0 to 1) among the words with the pairs of letters to
// practice, see focus, the others among all words, favouring those with weak
// pairs.
func (a adaptiveWords) draw(n int, ks keyStats, share float64) []string {
	w := newWeaknesses(ks)
	favoured := weightedPick(a.words, func(word string) float64 { return math.Pow(w.word(word), adaptivePower) })

	focused := a.containing(w.focus(focusPairs, a.frequency))
	if len(focused) == 0 {
		return drawWords(n, a.words, favoured)
	}

	candidates := a.words
	if share >= 1 {
		candidates = focused
	}

	return drawWords(n, candidates, func() string {
		if rand.Float64() < share {
			return focused[rand.Intn(len(focused))]
		}
		return favoured()
	})
}

// containing returns the words containing at least one of the given pairs of
// letters.
func (a adaptiveWords) containing(pairs []string) []string {
	wanted := map[string]bool{}
	for _, pair := range pairs {
		wanted[pair] = true
	}

	var words []string
	for _, word := range a.words {
		for _, pair := range letterPairs(word) {
			if wanted[pair] {
				words = append(words, word)
				break
			}
		}
	}

	return words
}

// weightedPick returns a function drawing one of words at random, in
// proportion to their weight.
func weightedPick(words []string, weight func(word string) float64) func() string {
	cumulative := make([]float64, len(words))
	total := 0.0
	for i, word := range words {
		total += weight(word)
		cumulative[i] = total
	}

	return func() string {
		return words[sort.SearchFloat64s(cumulative, rand.Float64()*total)]
	}
}

// generateAdaptiveTest draws words from the given word list according to the
// latest key statistics, focus percent of them with the pairs of letters to
// practice, see adaptiveWords.draw.
func generateAdaptiveTest(name string, o wordOptions, focus int) (func() []segment, error) {
	a, err := loadAdaptiveWords(name)
	if err != nil {
		return nil, err
	}

	return wordTest(func(n int) []string {
		return a.draw(n, loadKeyStats(), float64(focus)/100)
	}, o), nil
}
