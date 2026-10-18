package typing

import "sort"

// WordCount is a word and how many times it was mistyped.
type WordCount struct {
	Word  string
	Count int
}

// MostMistyped returns up to n of the most frequently mistyped words, the most
// frequent first.
func MostMistyped(mistakes []Mistake, n int) []WordCount {
	counts := map[string]int{}
	for _, m := range mistakes {
		counts[m.Word]++
	}

	words := make([]WordCount, 0, len(counts))
	for w, c := range counts {
		words = append(words, WordCount{w, c})
	}

	sort.Slice(words, func(i, j int) bool {
		if words[i].Count != words[j].Count {
			return words[i].Count > words[j].Count
		}
		return words[i].Word < words[j].Word
	})

	if len(words) > n {
		words = words[:n]
	}

	return words
}
