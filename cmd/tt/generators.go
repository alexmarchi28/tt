package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The generators below return functions producing the successive tests of a
// session, each made of one or more segments. They return nil once there are
// no more tests.

// testKind describes where the text of the tests comes from.
type testKind struct {
	mode   string // words, quotes, mistakes, file or text.
	source string // The word list, quote file or file used, if any.
}

func (k testKind) String() string {
	switch {
	case k.mode == "":
		return "unknown"
	case k.source == "":
		return k.mode
	default:
		return k.mode + " " + k.source
	}
}

// wordOptions describes the tests made of random words.
type wordOptions struct {
	n           int  // Words per segment.
	g           int  // Segments per test.
	punctuation bool // Turn the words into sentences.
	numbers     bool // Replace some of the words with numbers.
}

func generateWordTest(name string, o wordOptions) func() []segment {
	var b []byte

	if b = readResource("words", name); b == nil {
		die("%s does not appear to be a valid word list. See '-list words' for a list of builtin word lists.", name)
	}

	words := strings.Fields(string(b))
	if len(words) == 0 {
		die("%s does not contain any words.", name)
	}

	return wordTest(func(n int) []string { return randomWords(n, words) }, o)
}

// recentMistakes is the number of most recent mistakes practiced with
// -mistakes, so that words which have since been mastered eventually stop
// coming up.
const recentMistakes = 500

// weightedMistakes returns the words of the last n mistakes, each repeated as
// many times as it was mistyped.
func weightedMistakes(mistakes []mistake, n int) []string {
	if len(mistakes) > n {
		mistakes = mistakes[len(mistakes)-n:]
	}

	var words []string
	for _, w := range mostMistyped(mistakes, len(mistakes)) {
		for i := 0; i < w.count; i++ {
			words = append(words, w.word)
		}
	}

	return words
}

// generateMistakesTest practices the recently mistyped words, the most
// frequently mistyped ones coming up more often.
func generateMistakesTest(o wordOptions) func() []segment {
	var mistakes []mistake
	if err := readValue(mistakeDB, &mistakes); err != nil && !os.IsNotExist(err) {
		die("Could not read the recorded mistakes: %v", err)
	}

	words := weightedMistakes(mistakes, recentMistakes)
	if len(words) == 0 {
		die("There are no recorded mistakes to practice yet.")
	}

	return wordTest(func(n int) []string { return randomWords(n, words) }, o)
}

// wordTest makes tests of words drawn by draw.
func wordTest(draw func(n int) []string, o wordOptions) func() []segment {
	return func() []segment {
		segments := make([]segment, o.g)
		for i := range segments {
			text := draw(o.n)
			if o.numbers {
				text = addNumbers(text)
			}
			if o.punctuation {
				text = punctuate(text)
			}

			segments[i] = segment{strings.Join(text, " "), ""}
		}

		return segments
	}
}

// randomWords draws n words, avoiding consecutive repetitions when possible.
func randomWords(n int, words []string) []string {
	return drawWords(n, words, func() string { return words[rand.Intn(len(words))] })
}

// drawWords draws n words with pick, which randomly chooses one of words,
// avoiding consecutive repetitions when possible.
func drawWords(n int, words []string, pick func() string) []string {
	//Consecutive repetitions can only be avoided if there are at least two distinct words.
	canAvoidRepeats := false
	for _, w := range words {
		if w != words[0] {
			canAvoidRepeats = true
			break
		}
	}

	drawn := make([]string, n)
	for i := range drawn {
		w := pick()
		for canAvoidRepeats && i > 0 && drawn[i-1] == w {
			w = pick()
		}

		drawn[i] = w
	}

	return drawn
}

// sentenceEnds are the marks ending sentences, periods being the most common.
var sentenceEnds = []string{".", ".", ".", ".", "?", "!"}

// punctuate turns words into sentences by capitalizing their first word and
// adding punctuation marks. The last word always ends a sentence.
func punctuate(words []string) []string {
	out := make([]string, len(words))
	capitalize := true

	for i, w := range words {
		if capitalize {
			r, size := utf8.DecodeRuneInString(w)
			w = string(unicode.ToUpper(r)) + w[size:]
		}

		//Roughly 8% of the words end a sentence, 10% are followed by a comma, 2% by a
		//semicolon, 2% by a colon, 3% are quoted and 3% are in parentheses.
		capitalize = false
		switch p := rand.Float64(); {
		case i == len(words)-1 || p < 0.08:
			w += sentenceEnds[rand.Intn(len(sentenceEnds))]
			capitalize = true
		case p < 0.18:
			w += ","
		case p < 0.20:
			w += ";"
		case p < 0.22:
			w += ":"
		case p < 0.25:
			w = `"` + w + `"`
		case p < 0.28:
			w = "(" + w + ")"
		}

		out[i] = w
	}

	return out
}

// addNumbers replaces some of the words with numbers of up to 4 digits.
func addNumbers(words []string) []string {
	out := make([]string, len(words))

	for i, w := range words {
		if rand.Float64() < 0.15 {
			digits := 1 + rand.Intn(4)
			w = strconv.Itoa(rand.Intn(int(math.Pow10(digits))))
		}

		out[i] = w
	}

	return out
}

func generateQuoteTest(name string) func() []segment {
	var quotes []segment

	if b := readResource("quotes", name); b == nil {
		die("%s does not appear to be a valid quote file. See '-list quotes' for a list of builtin quotes.", name)
	} else {
		err := json.Unmarshal(b, &quotes)
		if err != nil {
			die("Improperly formatted quote file: %v", err)
		}
	}

	if len(quotes) == 0 {
		die("%s does not contain any quotes.", name)
	}

	return func() []segment {
		idx := rand.Int() % len(quotes)
		return quotes[idx : idx+1]
	}
}

func generateTestFromData(data []byte, raw bool, split bool) func() []segment {
	if raw {
		return func() []segment { return []segment{{string(data), ""}} }
	} else if split {
		paragraphs := getParagraphs(string(data))
		i := 0

		return func() []segment {
			if i < len(paragraphs) {
				p := paragraphs[i]
				i++
				return []segment{{p, ""}}
			} else {
				return nil
			}
		}
	} else {
		return func() []segment {
			var segments []segment

			for _, p := range getParagraphs(string(data)) {
				segments = append(segments, segment{p, ""})
			}

			return segments
		}
	}
}

func generateTestFromFile(path string, startParagraph int) func() []segment {
	var paragraphs []string
	var db map[string]int
	var err error

	if path, err = filepath.Abs(path); err != nil {
		panic(err)
	}

	if err := readValue(fileStateDB, &db); err != nil {
		db = map[string]int{}
	}

	if startParagraph != -1 {
		db[path] = startParagraph
		if err := writeValue(fileStateDB, db); err != nil {
			panic(err)
		}
	}

	idx := db[path] - 1

	if b, err := os.ReadFile(path); err != nil {
		die("Failed to read %s.", path)
	} else {
		paragraphs = getParagraphs(string(b))
	}

	return func() []segment {
		idx++
		db[path] = idx
		if err := writeValue(fileStateDB, db); err != nil {
			panic(err)
		}

		if idx >= len(paragraphs) {
			return nil
		}

		return []segment{{paragraphs[idx], ""}}
	}
}
