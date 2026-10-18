package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The generators below return functions producing the successive tests of a
// session, each made of one or more segments, see funcSource. They return nil
// once there are no more tests.

// generatorStore gives the generators the saved data they make tests from.
type generatorStore interface {
	mistakeStore
	keyStatsStore
	paragraphStore
}

// textSources are what the generators make the text of tests from.
type textSources struct {
	store           generatorStore
	resources       resourceReader
	rng             *rand.Rand
	stdin           io.Reader // Its text is typed unless it is a terminal.
	stdinIsTerminal bool
}

// newTestSource returns the source of the tests requested on the command
// line and what they are made of.
func newTestSource(opts options, src textSources) (testSource, testKind, error) {
	tests, kind, err := selectTestSource(opts, src)
	if err != nil {
		return nil, testKind{}, err
	}

	return tests, kind.withVariant(opts), nil
}

// selectTestSource returns the source of the tests requested on the command
// line and where their text comes from, see newTestSource.
func selectTestSource(opts options, src textSources) (testSource, testKind, error) {
	words := newWordOptions(opts)
	res, rng := src.resources, src.rng

	switch {
	case opts.adaptive:
		list := adaptiveWordList(opts)
		tests, err := newAdaptiveSource(rng, res, list, words, opts.focus, src.store)
		return tests, testKind{mode: modeAdaptive, source: filepath.Base(list)}, err
	case opts.wordFile != "":
		tests, err := fromFunc(generateWordTest(rng, res, opts.wordFile, words))
		return tests, testKind{mode: modeWords, source: filepath.Base(opts.wordFile)}, err
	case opts.quoteFile != "":
		tests, err := fromFunc(generateQuoteTest(rng, res, opts.quoteFile))
		return tests, testKind{mode: modeQuotes, source: filepath.Base(opts.quoteFile)}, err
	case opts.mistakes:
		tests, err := fromFunc(generateMistakesTest(rng, words, src.store))
		return tests, testKind{mode: modeMistakes}, err
	case !src.stdinIsTerminal:
		b, err := io.ReadAll(src.stdin)
		if err != nil {
			return nil, testKind{}, err
		}

		tests, err := fromFunc(generateTestFromData(b, opts.raw, opts.multi))
		return tests, testKind{mode: modeText}, err
	case len(opts.files) > 0:
		tests, err := newFileSource(opts.files[0], opts.startParagraph, src.store)
		return tests, testKind{mode: modeFile, source: filepath.Base(opts.files[0])}, err
	default:
		tests, err := fromFunc(generateWordTest(rng, res, defaultWordList, words))
		return tests, testKind{mode: modeWords, source: defaultWordList}, err
	}
}

// fromFunc returns the source of the tests made by gen, unless err is set.
func fromFunc(gen func() []segment, err error) (testSource, error) {
	if err != nil {
		return nil, err
	}

	return funcSource(gen), nil
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

func generateWordTest(rng *rand.Rand, res resourceReader, name string, o wordOptions) (func() []segment, error) {
	words, err := loadWords(res, name)
	if err != nil {
		return nil, err
	}

	return wordTest(rng, func(n int) []string { return randomWords(rng, n, words) }, o), nil
}

// loadWords returns the words of the named word list.
func loadWords(res resourceReader, name string) ([]string, error) {
	b, err := res.read("words", name)
	if errors.Is(err, errNoResource) {
		return nil, fmt.Errorf("%s does not appear to be a valid word list, see '-list words' for a list of builtin word lists", name)
	} else if err != nil {
		return nil, err
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
func generateMistakesTest(rng *rand.Rand, o wordOptions, st mistakeStore) (func() []segment, error) {
	mistakes, err := st.Mistakes()
	if err != nil {
		return nil, err
	}

	words := weightedMistakes(mistakes, recentMistakes)
	if len(words) == 0 {
		return nil, errors.New("there are no recorded mistakes to practice yet")
	}

	return wordTest(rng, func(n int) []string { return randomWords(rng, n, words) }, o), nil
}

// wordTest makes tests of words drawn by draw, one segment per group.
func wordTest(rng *rand.Rand, draw func(n int) []string, o wordOptions) func() []segment {
	return func() []segment {
		var segments []segment
		for chars := 0; len(segments) < o.g || chars < o.minChars; {
			text := wordGroup(rng, draw, o)
			segments = append(segments, segment{text, ""})
			chars += utf8.RuneCountInString(text)
		}

		return segments
	}
}

// wordGroup draws a group of words and adds numbers and punctuation to them
// if requested.
func wordGroup(rng *rand.Rand, draw func(n int) []string, o wordOptions) string {
	words := draw(o.n)
	if o.numbers {
		words = addNumbers(rng, words)
	}
	if o.punctuation {
		words = punctuate(rng, words)
	}

	return strings.Join(words, " ")
}

// randomWords draws n words, avoiding consecutive repetitions when possible.
func randomWords(rng *rand.Rand, n int, words []string) []string {
	return drawWords(n, words, func() string { return words[rng.Intn(len(words))] })
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
func punctuate(rng *rand.Rand, words []string) []string {
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
		switch p := rng.Float64(); {
		case i == len(words)-1 || p < 0.08:
			w += sentenceEnds[rng.Intn(len(sentenceEnds))]
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
func addNumbers(rng *rand.Rand, words []string) []string {
	out := make([]string, len(words))

	for i, w := range words {
		if rng.Float64() < 0.15 {
			digits := 1 + rng.Intn(4)
			w = strconv.Itoa(rng.Intn(int(math.Pow10(digits))))
		}

		out[i] = w
	}

	return out
}

func generateQuoteTest(rng *rand.Rand, res resourceReader, name string) (func() []segment, error) {
	var quotes []segment

	if b, err := res.read("quotes", name); errors.Is(err, errNoResource) {
		return nil, fmt.Errorf("%s does not appear to be a valid quote file, see '-list quotes' for a list of builtin quotes", name)
	} else if err != nil {
		return nil, err
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
		return []segment{quotes[rng.Intn(len(quotes))]}
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
