package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/alexmarchi28/tt/internal/keystats"
)

// layouts holds the characters of the unshifted keys of keyboard layouts, from
// the number row to the bottom row.
var layouts = map[string][]string{
	"us":         {"1234567890-=", "qwertyuiop[]", "asdfghjkl;'", "zxcvbnm,./"},
	"uk":         {"1234567890-=", "qwertyuiop[]", "asdfghjkl;'#", "zxcvbnm,./"},
	"it":         {"1234567890'ì", "qwertyuiopè+", "asdfghjklòàù", "zxcvbnm,.-"},
	"es":         {"1234567890'¡", "qwertyuiop`+", "asdfghjklñ´ç", "zxcvbnm,.-"},
	"de":         {"1234567890ß", "qwertzuiopü+", "asdfghjklöä#", "yxcvbnm,.-"},
	"fr":         {"&é\"'(-è_çà)=", "azertyuiop^$", "qsdfghjklmù*", "wxcvbn,;:!"},
	"dvorak":     {"1234567890[]", "',.pyfgcrl/=", "aoeuidhtns-", ";qjkxbmwvz"},
	"colemak":    {"1234567890-=", "qwfpgjluy;[]", "arstdhneio'", "zxcvbkm,./"},
	"colemak-dh": {"1234567890-=", "qwfpbjluy;[]", "arstgmneio'", "zxcdvkh,./"},
	"workman":    {"1234567890-=", "qdrwbjfup;[]", "ashtgyneoi'", "zxmcvkl,./"},
}

const (
	minKeyCount = 10 // Keystrokes needed to judge the accuracy of a key.
	spaceIndent = 9  // Offset of the space bar.
)

var (
	keyboardIndents = []int{0, 2, 3, 4}             // Offsets of the rows of a keyboard.
	levelShades     = []rune("·░▒▓█")               // Levels from the best (0) to the worst (4).
	levelColors     = []int{46, 154, 226, 208, 196} // Colours of the levels in the 256 colour palette.
)

// keyStatsByKey returns the statistics of each key, upper case letters being
// typed with the key of the lower case ones.
func keyStatsByKey(ks keystats.Stats) map[rune]keystats.Stat {
	byKey := map[rune]keystats.Stat{}

	for s, stat := range ks.Chars {
		for _, r := range s {
			key := unicode.ToLower(r)
			byKey[key] = keystats.Merge(byKey[key], stat)
			break
		}
	}

	return byKey
}

// keyLevels ranks values from 0 (the lowest) to 4 (the highest).
func keyLevels(values map[rune]float64) map[rune]int {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}

	levels := map[rune]int{}
	for r, v := range values {
		if hi > lo {
			levels[r] = int(math.Round((v - lo) / (hi - lo) * float64(len(levelShades)-1)))
		} else {
			levels[r] = 0
		}
	}

	return levels
}

// drawKeyboard draws the keys of rows and the space bar, each followed by the
// shade of its level if it has one.
func drawKeyboard(rows []string, levels map[rune]int, color bool) []string {
	cell := func(label string, r rune) string {
		level, ok := levels[r]
		if !ok {
			return label + "  "
		}

		text := label + string(levelShades[level])
		if color {
			text = fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", levelColors[level], text)
		}
		return text + " "
	}

	var lines []string
	for i, row := range rows {
		line := strings.Repeat(" ", keyboardIndents[i])
		for _, r := range row {
			line += cell(string(r), r)
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}

	space := strings.Repeat(" ", spaceIndent) + cell("space", ' ')
	return append(lines, strings.TrimRight(space, " "))
}

// worstKeys returns up to n of the keys with the highest values.
func worstKeys(values map[rune]float64, n int) []rune {
	keys := make([]rune, 0, len(values))
	for r := range values {
		keys = append(keys, r)
	}

	sort.Slice(keys, func(i, j int) bool {
		if values[keys[i]] != values[keys[j]] {
			return values[keys[i]] > values[keys[j]]
		}
		return keys[i] < keys[j]
	})

	if len(keys) > n {
		keys = keys[:n]
	}

	return keys
}

func keyLabel(r rune) string {
	switch r {
	case ' ':
		return "space"
	case '\t':
		return "tab"
	default:
		return string(r)
	}
}

// FormatKeys describes how accurately and quickly each key is typed on a
// keyboard with the given rows.
func FormatKeys(ks keystats.Stats, rows []string, color bool) string {
	accuracy, speed := map[rune]float64{}, map[rune]float64{}
	for r, s := range keyStatsByKey(ks) {
		if s.Count >= minKeyCount {
			accuracy[r] = s.ErrorRate
		}
		if s.Timed >= keystats.MinTimed {
			speed[r] = s.Latency
		}
	}

	var b strings.Builder
	fmt.Fprintln(&b, "Accuracy by key (· best, █ worst, compared to your other keys)")
	for _, line := range drawKeyboard(rows, keyLevels(accuracy), color) {
		fmt.Fprintln(&b, line)
	}

	fmt.Fprintln(&b, "\nSpeed by key (· fastest, █ slowest)")
	for _, line := range drawKeyboard(rows, keyLevels(speed), color) {
		fmt.Fprintln(&b, line)
	}

	var missed, slowest []string
	for _, r := range worstKeys(accuracy, 5) {
		if accuracy[r] > 0 {
			missed = append(missed, fmt.Sprintf("%s %.1f%%", keyLabel(r), accuracy[r]*100))
		}
	}
	for _, r := range worstKeys(speed, 5) {
		slowest = append(slowest, fmt.Sprintf("%s %.0fms", keyLabel(r), speed[r]))
	}

	if len(missed) > 0 || len(slowest) > 0 {
		fmt.Fprintln(&b)
	}
	if len(missed) > 0 {
		fmt.Fprintf(&b, "%-17s %s\n", "Most missed keys", strings.Join(missed, ", "))
	}
	if len(slowest) > 0 {
		fmt.Fprintf(&b, "%-17s %s\n", "Slowest keys", strings.Join(slowest, ", "))
	}

	return b.String()
}

// KeyboardLayout returns the rows of the named keyboard layout, see layouts.
func KeyboardLayout(name string) ([]string, error) {
	rows, ok := layouts[name]
	if !ok {
		return nil, fmt.Errorf("unknown layout %s, use one of: %s", name, strings.Join(sortedKeys(layouts), ", "))
	}

	return rows, nil
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
