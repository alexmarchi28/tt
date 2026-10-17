package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// newTestGenerator returns a function producing the successive tests
// requested on the command line, or nil once there are no more, and what they
// are made of. Text piped to stdin is used unless stdin is a terminal.
func newTestGenerator(opts options, stdin io.Reader, stdinIsTerminal bool) (func() []segment, testKind, error) {
	gen, kind, err := selectTestGenerator(opts, stdin, stdinIsTerminal)
	if err != nil {
		return nil, testKind{}, err
	}

	return gen, kind.withVariant(opts), nil
}

// selectTestGenerator returns the generator of the tests requested on the
// command line and where their text comes from, see newTestGenerator.
func selectTestGenerator(opts options, stdin io.Reader, stdinIsTerminal bool) (func() []segment, testKind, error) {
	words := newWordOptions(opts)

	switch {
	case opts.adaptive:
		list := adaptiveWordList(opts)
		gen, err := generateAdaptiveTest(list, words, opts.focus)
		return gen, testKind{mode: modeAdaptive, source: filepath.Base(list)}, err
	case opts.wordFile != "":
		gen, err := generateWordTest(opts.wordFile, words)
		return gen, testKind{mode: modeWords, source: filepath.Base(opts.wordFile)}, err
	case opts.quoteFile != "":
		gen, err := generateQuoteTest(opts.quoteFile)
		return gen, testKind{mode: modeQuotes, source: filepath.Base(opts.quoteFile)}, err
	case opts.mistakes:
		gen, err := generateMistakesTest(words)
		return gen, testKind{mode: modeMistakes}, err
	case !stdinIsTerminal:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, testKind{}, err
		}

		gen, err := generateTestFromData(b, opts.raw, opts.multi)
		return gen, testKind{mode: modeText}, err
	case len(opts.files) > 0:
		gen, err := generateTestFromFile(opts.files[0], opts.startParagraph)
		return gen, testKind{mode: modeFile, source: filepath.Base(opts.files[0])}, err
	default:
		gen, err := generateWordTest(defaultWordList, words)
		return gen, testKind{mode: modeWords, source: defaultWordList}, err
	}
}

// defaultWordList is the word list of word tests unless -words is given.
const defaultWordList = "1000en"

// adaptiveWordList returns the word list of adaptive tests.
func adaptiveWordList(opts options) string {
	if opts.wordFile != "" {
		return opts.wordFile
	}

	return defaultWordList
}

// fastestCpm is a speed beyond the reach of any typist (500 wpm), at which
// timed word tests still don't run out of words.
const fastestCpm = 2500

// wordOptions describes the tests made of random words.
type wordOptions struct {
	n           int  // Words per segment.
	g           int  // Least number of segments per test.
	punctuation bool // Turn the words into sentences.
	numbers     bool // Replace some of the words with numbers.
	minChars    int  // Groups are added until the test has this many characters.
}

// newWordOptions returns the options of word tests given on the command line.
// Timed tests get enough words to last until the time is up.
func newWordOptions(opts options) wordOptions {
	o := wordOptions{n: opts.n, g: opts.g, punctuation: opts.punct, numbers: opts.numbers}
	if opts.timeout != -1 {
		o.minChars = int(opts.timeout.Minutes() * fastestCpm)
	}

	return o
}

func generateWordTest(name string, o wordOptions) (func() []segment, error) {
	words, err := loadWords(name)
	if err != nil {
		return nil, err
	}

	return wordTest(func(n int) []string { return randomWords(n, words) }, o), nil
}

// loadWords returns the words of the named word list.
func loadWords(name string) ([]string, error) {
	b := readResource("words", name)
	if b == nil {
		return nil, fmt.Errorf("%s does not appear to be a valid word list, see '-list words' for a list of builtin word lists", name)
	}

	words := strings.Fields(string(b))
	if len(words) == 0 {
		return nil, fmt.Errorf("%s does not contain any words", name)
	}

	return words, nil
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
func generateMistakesTest(o wordOptions) (func() []segment, error) {
	mistakes, err := readMistakes()
	if err != nil {
		return nil, err
	}

	words := weightedMistakes(mistakes, recentMistakes)
	if len(words) == 0 {
		return nil, errors.New("there are no recorded mistakes to practice yet")
	}

	return wordTest(func(n int) []string { return randomWords(n, words) }, o), nil
}

// wordTest makes tests of words drawn by draw, one segment per group.
func wordTest(draw func(n int) []string, o wordOptions) func() []segment {
	return func() []segment {
		var segments []segment
		for chars := 0; len(segments) < o.g || chars < o.minChars; {
			text := wordGroup(draw, o)
			segments = append(segments, segment{text, ""})
			chars += utf8.RuneCountInString(text)
		}

		return segments
	}
}

// wordGroup draws a group of words and adds numbers and punctuation to them
// if requested.
func wordGroup(draw func(n int) []string, o wordOptions) string {
	words := draw(o.n)
	if o.numbers {
		words = addNumbers(words)
	}
	if o.punctuation {
		words = punctuate(words)
	}

	return strings.Join(words, " ")
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

func generateQuoteTest(name string) (func() []segment, error) {
	var quotes []segment

	if b := readResource("quotes", name); b == nil {
		return nil, fmt.Errorf("%s does not appear to be a valid quote file, see '-list quotes' for a list of builtin quotes", name)
	} else if err := json.Unmarshal(b, &quotes); err != nil {
		return nil, fmt.Errorf("improperly formatted quote file: %v", err)
	}

	var withText []segment
	for _, q := range quotes {
		if strings.TrimSpace(q.Text) != "" {
			withText = append(withText, q)
		}
	}
	quotes = withText

	if len(quotes) == 0 {
		return nil, fmt.Errorf("%s does not contain any quotes", name)
	}

	return func() []segment {
		return []segment{quotes[rand.Intn(len(quotes))]}
	}, nil
}

func generateTestFromData(data []byte, raw bool, split bool) (func() []segment, error) {
	paragraphs := getParagraphs(string(data))
	if len(paragraphs) == 0 {
		return nil, errors.New("there is no text to type")
	}

	if raw {
		return func() []segment { return []segment{{string(data), ""}} }, nil
	} else if split {
		i := 0

		return func() []segment {
			if i < len(paragraphs) {
				p := paragraphs[i]
				i++
				return []segment{{p, ""}}
			} else {
				return nil
			}
		}, nil
	} else {
		return func() []segment {
			var segments []segment

			for _, p := range paragraphs {
				segments = append(segments, segment{p, ""})
			}

			return segments
		}, nil
	}
}

func generateTestFromFile(path string, startParagraph int) (func() []segment, error) {
	var db map[string]int
	var err error

	if path, err = filepath.Abs(path); err != nil {
		return nil, err
	}

	if err := readValue(fileStateDB, &db); err != nil {
		db = map[string]int{}
	}

	if startParagraph != -1 {
		db[path] = startParagraph
		if err := writeValue(fileStateDB, db); err != nil {
			return nil, err
		}
	}

	idx := db[path] - 1

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s", path)
	}
	paragraphs := getParagraphs(string(b))
	if len(paragraphs) == 0 {
		return nil, fmt.Errorf("%s does not contain any text", path)
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
	}, nil
}
