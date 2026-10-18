package textgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alexmarchi28/tt/internal/resource"
	"github.com/alexmarchi28/tt/internal/typing"
)

// The generators below return functions producing the successive tests of a
// session, each made of one or more segments, see Func. They return nil once
// there are no more tests.

// Words makes tests of words drawn from the named word list.
func Words(rng *rand.Rand, res Resources, name string, o WordOptions) (Source, error) {
	return fromFunc(generateWordTest(rng, res, name, o))
}

// Quotes makes tests of quotes drawn from the named quote file.
func Quotes(rng *rand.Rand, res Resources, name string) (Source, error) {
	return fromFunc(generateQuoteTest(rng, res, name))
}

// Mistakes makes tests of the recently mistyped words, the most frequently
// mistyped ones coming up more often.
func Mistakes(rng *rand.Rand, o WordOptions, st MistakeStore) (Source, error) {
	return fromFunc(generateMistakesTest(rng, o, st))
}

// Text makes tests of the paragraphs of text: all of them in a test, one
// per test (split), or text as is (raw).
func Text(text []byte, raw, split bool) (Source, error) {
	return fromFunc(generateTestFromData(text, raw, split))
}

// fromFunc returns the source of the tests made by gen, unless err is set.
func fromFunc(gen func() []Segment, err error) (Source, error) {
	if err != nil {
		return nil, err
	}

	return Func(gen), nil
}

// WordOptions describes the tests made of random words.
type WordOptions struct {
	N           int  // Words per segment.
	G           int  // Least number of segments per test.
	Punctuation bool // Turn the words into sentences.
	Numbers     bool // Replace some of the words with numbers.
	MinChars    int  // Groups are added until the test has this many characters.
}

func generateWordTest(rng *rand.Rand, res Resources, name string, o WordOptions) (func() []Segment, error) {
	words, err := loadWords(res, name)
	if err != nil {
		return nil, err
	}

	return wordTest(rng, func(n int) []string { return randomWords(rng, n, words) }, o), nil
}

// loadWords returns the words of the named word list.
func loadWords(res Resources, name string) ([]string, error) {
	b, err := res.Read("words", name)
	if errors.Is(err, resource.ErrNotFound) {
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
func weightedMistakes(mistakes []typing.Mistake, n int) []string {
	if len(mistakes) > n {
		mistakes = mistakes[len(mistakes)-n:]
	}

	var words []string
	for _, w := range typing.MostMistyped(mistakes, len(mistakes)) {
		for i := 0; i < w.Count; i++ {
			words = append(words, w.Word)
		}
	}

	return words
}

// generateMistakesTest practices the recently mistyped words, the most
// frequently mistyped ones coming up more often.
func generateMistakesTest(rng *rand.Rand, o WordOptions, st MistakeStore) (func() []Segment, error) {
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
func wordTest(rng *rand.Rand, draw func(n int) []string, o WordOptions) func() []Segment {
	return func() []Segment {
		var segments []Segment
		for chars := 0; len(segments) < o.G || chars < o.MinChars; {
			text := wordGroup(rng, draw, o)
			segments = append(segments, Segment{text, ""})
			chars += utf8.RuneCountInString(text)
		}

		return segments
	}
}

// wordGroup draws a group of words and adds numbers and punctuation to them
// if requested.
func wordGroup(rng *rand.Rand, draw func(n int) []string, o WordOptions) string {
	words := draw(o.N)
	if o.Numbers {
		words = addNumbers(rng, words)
	}
	if o.Punctuation {
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

func generateQuoteTest(rng *rand.Rand, res Resources, name string) (func() []Segment, error) {
	var quotes []Segment

	if b, err := res.Read("quotes", name); errors.Is(err, resource.ErrNotFound) {
		return nil, fmt.Errorf("%s does not appear to be a valid quote file, see '-list quotes' for a list of builtin quotes", name)
	} else if err != nil {
		return nil, err
	} else if err := json.Unmarshal(b, &quotes); err != nil {
		return nil, fmt.Errorf("improperly formatted quote file: %v", err)
	}

	var withText []Segment
	for _, q := range quotes {
		if strings.TrimSpace(q.Text) != "" {
			withText = append(withText, q)
		}
	}
	quotes = withText

	if len(quotes) == 0 {
		return nil, fmt.Errorf("%s does not contain any quotes", name)
	}

	return func() []Segment {
		return []Segment{quotes[rng.Intn(len(quotes))]}
	}, nil
}

func generateTestFromData(data []byte, raw bool, split bool) (func() []Segment, error) {
	paragraphs := getParagraphs(string(data))
	if len(paragraphs) == 0 {
		return nil, errors.New("there is no text to type")
	}

	if raw {
		return func() []Segment { return []Segment{{string(data), ""}} }, nil
	} else if split {
		i := 0

		return func() []Segment {
			if i < len(paragraphs) {
				p := paragraphs[i]
				i++
				return []Segment{{p, ""}}
			} else {
				return nil
			}
		}, nil
	} else {
		return func() []Segment {
			var segments []Segment

			for _, p := range paragraphs {
				segments = append(segments, Segment{p, ""})
			}

			return segments
		}, nil
	}
}
