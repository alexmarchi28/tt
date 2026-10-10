package main

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

func TestLayouts(t *testing.T) {
	for name, rows := range layouts {
		seen := map[rune]bool{}
		letters := 0

		for _, row := range rows {
			for _, r := range row {
				if seen[r] {
					t.Errorf("%s: %q appears twice", name, r)
				}
				seen[r] = true

				if r >= 'a' && r <= 'z' {
					letters++
				}
				if unicode.IsUpper(r) {
					t.Errorf("%s: %q is not the character of an unshifted key", name, r)
				}
			}
		}

		if letters != 26 {
			t.Errorf("%s has %d of the 26 letters", name, letters)
		}
	}
}

func TestMergeKeyStats(t *testing.T) {
	a := keyStat{Count: 30, ErrorRate: 0.25, Timed: 10, Latency: 100}
	b := keyStat{Count: 10, ErrorRate: 0.75, Timed: 30, Latency: 200}

	want := keyStat{Count: 40, ErrorRate: 0.375, Timed: 40, Latency: 175}
	if got := mergeKeyStats(a, b); got != want {
		t.Errorf("mergeKeyStats() = %+v, want %+v", got, want)
	}

	// Merging with nothing keeps the values exactly, 0.1*3/3 would not.
	c := keyStat{Count: 3, ErrorRate: 0.1, Timed: 3, Latency: 0.1}
	if got := mergeKeyStats(c, keyStat{}); got != c {
		t.Errorf("merging with nothing = %+v, want %+v", got, c)
	}
	if got := mergeKeyStats(keyStat{}, c); got != c {
		t.Errorf("merging nothing = %+v, want %+v", got, c)
	}
}

func TestKeyStatsByKey(t *testing.T) {
	ks := keyStats{Chars: map[string]keyStat{
		"a": {Count: 10, ErrorRate: 0.25},
		"A": {Count: 10, ErrorRate: 0.75},
		"?": {Count: 5, ErrorRate: 1},
	}}

	// Upper case letters are typed with the key of the lower case ones.
	got := keyStatsByKey(ks)
	want := map[rune]keyStat{
		'a': {Count: 20, ErrorRate: 0.5},
		'?': {Count: 5, ErrorRate: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keyStatsByKey() = %+v, want %+v", got, want)
	}
}

func TestKeyLevels(t *testing.T) {
	values := map[rune]float64{'a': 10, 'b': 15, 'c': 20, 'd': 50}

	want := map[rune]int{'a': 0, 'b': 1, 'c': 1, 'd': 4}
	if got := keyLevels(values); !reflect.DeepEqual(got, want) {
		t.Errorf("keyLevels() = %v, want %v", got, want)
	}

	if got := keyLevels(map[rune]float64{'a': 3, 'b': 3}); !reflect.DeepEqual(got, map[rune]int{'a': 0, 'b': 0}) {
		t.Errorf("equal values get levels %v, want the best one", got)
	}
}

func TestDrawKeyboard(t *testing.T) {
	rows := []string{"ab", "cd"}
	levels := map[rune]int{'a': 0, 'b': 4, 'd': 2, ' ': 3}

	want := []string{
		"a· b█",
		"  c  d▒",
		"         space▓",
	}
	if got := drawKeyboard(rows, levels, false); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	colored := drawKeyboard(rows, levels, true)
	if !strings.Contains(colored[0], "\x1b[38;5;196mb█\x1b[0m") || strings.Contains(colored[1], "\x1b[38;5;46mc") {
		t.Errorf("unexpected colours:\n%q", colored)
	}
}

func testKeyStats() keyStats {
	ks := newKeyStats()
	for _, c := range "abcdefghijklmnopqrstuvwxyz " {
		ks.Chars[string(c)] = keyStat{Count: 50, ErrorRate: 0.02, Timed: 40, Latency: 150}
	}

	ks.Chars["q"] = keyStat{Count: 50, ErrorRate: 0.2, Timed: 40, Latency: 400}
	ks.Chars["z"] = keyStat{Count: 50, ErrorRate: 0.1, Timed: 40, Latency: 150}
	ks.Chars["x"] = keyStat{Count: 50, ErrorRate: 0.02, Timed: 40, Latency: 300}
	ks.Chars["j"] = keyStat{Count: 5, ErrorRate: 1, Timed: 4, Latency: 900} // Not enough data.
	return ks
}

func TestFormatKeys(t *testing.T) {
	got := formatKeys(testKeyStats(), layouts["us"], false)

	want := "" +
		"Accuracy by key (· best, █ worst, compared to your other keys)\n" +
		"1  2  3  4  5  6  7  8  9  0  -  =\n" +
		"  q█ w· e· r· t· y· u· i· o· p· [  ]\n" +
		"   a· s· d· f· g· h· j  k· l· ;  '\n" +
		"    z▒ x· c· v· b· n· m· ,  .  /\n" +
		"         space·\n" +
		"\n" +
		"Speed by key (· fastest, █ slowest)\n" +
		"1  2  3  4  5  6  7  8  9  0  -  =\n" +
		"  q█ w· e· r· t· y· u· i· o· p· [  ]\n" +
		"   a· s· d· f· g· h· j  k· l· ;  '\n" +
		"    z· x▒ c· v· b· n· m· ,  .  /\n" +
		"         space·\n" +
		"\n" +
		"Most missed keys  q 20.0%, z 10.0%, space 2.0%, a 2.0%, b 2.0%\n" +
		"Slowest keys      q 400ms, x 300ms, space 150ms, a 150ms, b 150ms\n"

	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFormatKeysListsOnlyMissedKeys(t *testing.T) {
	ks := newKeyStats()
	ks.Chars["a"] = keyStat{Count: 20, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Chars["b"] = keyStat{Count: 20, ErrorRate: 0.1, Timed: 10, Latency: 100}

	got := formatKeys(ks, layouts["us"], false)
	if !strings.Contains(got, "Most missed keys  b 10.0%\n") {
		t.Errorf("keys never missed should not be listed:\n%s", got)
	}
}
