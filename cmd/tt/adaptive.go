package main

import (
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/alexmarchi28/tt/internal/keystats"
)

const (
	adaptivePower = 2 // How strongly words with weak pairs of letters are favoured.
	focusPairs    = 3 // Weak pairs of letters practiced at once.
)

// adaptiveWords is the word list of adaptive tests.
type adaptiveWords struct {
	words     []string
	frequency map[string]float64 // See keystats.PairFrequencies.
}

func loadAdaptiveWords(res resourceReader, name string) (adaptiveWords, error) {
	words, err := loadWords(res, name)
	if err != nil {
		return adaptiveWords{}, err
	}

	return adaptiveWords{words, keystats.PairFrequencies(words)}, nil
}

// focus returns the weak pairs of letters to practice according to the
// given key statistics.
func (a adaptiveWords) focus(ks keystats.Stats) []string {
	return keystats.NewWeaknesses(ks).Focus(focusPairs, a.frequency)
}

// describe tells what adaptive tests practice.
func (a adaptiveWords) describe(ks keystats.Stats) string {
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
func (a adaptiveWords) draw(rng *rand.Rand, n int, ks keystats.Stats, share float64) []string {
	w := keystats.NewWeaknesses(ks)
	favoured := weightedPick(rng, a.words, func(word string) float64 { return math.Pow(w.Word(word), adaptivePower) })

	focused := a.containing(w.Focus(focusPairs, a.frequency))
	if len(focused) == 0 {
		return drawWords(n, a.words, favoured)
	}

	candidates := a.words
	if share >= 1 {
		candidates = focused
	}

	return drawWords(n, candidates, func() string {
		if rng.Float64() < share {
			return focused[rng.Intn(len(focused))]
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
		for _, pair := range keystats.LetterPairs(word) {
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
func weightedPick(rng *rand.Rand, words []string, weight func(word string) float64) func() string {
	cumulative := make([]float64, len(words))
	total := 0.0
	for i, word := range words {
		total += weight(word)
		cumulative[i] = total
	}

	return func() string {
		return words[sort.SearchFloat64s(cumulative, rng.Float64()*total)]
	}
}
