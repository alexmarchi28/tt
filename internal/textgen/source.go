// Package textgen makes the texts of the tests: random words, quotes, the
// recently mistyped words, the words practicing weak pairs of letters, the
// paragraphs of a file or of text read from stdin.
package textgen

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

// Segment is a text to type in one go, and who wrote it if known.
type Segment struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution"`
}

// Resources gives the word lists and quote files, by type and name.
type Resources interface {
	Read(typ, name string) ([]byte, error)
}

// MistakeStore gives the recorded mistakes.
type MistakeStore interface {
	Mistakes() ([]typing.Mistake, error)
}

// KeyStatsStore gives the latest key statistics.
type KeyStatsStore interface {
	KeyStats() keystats.Stats
}

// ParagraphStore remembers the paragraph reached in each file.
type ParagraphStore interface {
	// Paragraph returns the paragraph reached in the file at path, 0 for a
	// file never typed.
	Paragraph(path string) int
	SetParagraph(path string, paragraph int) error
}

// Source produces the successive tests of a session, each made of one or
// more segments.
type Source interface {
	// Next returns the Next test, or io.EOF once there are no more.
	Next() ([]Segment, error)
}

// Describer is implemented by the test sources telling what their tests
// practice, which is shown before each of them.
type Describer interface {
	Describe() string
}

// Func produces the tests returned by a function, which returns nil
// once there are no more.
type Func func() []Segment

func (f Func) Next() ([]Segment, error) {
	if test := f(); test != nil {
		return test, nil
	}

	return nil, io.EOF
}

// Adaptive draws words from a word list according to the latest key
// statistics, see adaptiveWords.draw.
type Adaptive struct {
	words adaptiveWords
	rng   *rand.Rand
	o     WordOptions
	share float64 // Share of the words with the pairs of letters practiced.
	stats KeyStatsStore
}

// NewAdaptive draws words from the named word list, focus percent of
// them with the pairs of letters to practice.
func NewAdaptive(rng *rand.Rand, res Resources, name string, o WordOptions, focus int, stats KeyStatsStore) (*Adaptive, error) {
	words, err := loadAdaptiveWords(res, name)
	if err != nil {
		return nil, err
	}

	return &Adaptive{words, rng, o, float64(focus) / 100, stats}, nil
}

func (s *Adaptive) Next() ([]Segment, error) {
	ks := s.stats.KeyStats()
	return wordTest(s.rng, func(n int) []string { return s.words.draw(s.rng, n, ks, s.share) }, s.o)(), nil
}

// Describe tells which letter pairs the tests of s focus on.
func (s *Adaptive) Describe() string {
	return s.words.describe(s.stats.KeyStats())
}

// File makes a test of each paragraph of a file, remembering the
// paragraph reached.
type File struct {
	path       string
	paragraphs []string
	idx        int // The paragraph of the last test.
	store      ParagraphStore
}

// NewFile makes tests of the paragraphs of the file at path, starting
// from the given paragraph, or else from the one reached last.
func NewFile(path string, startParagraph int, st ParagraphStore) (*File, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if startParagraph != -1 {
		if err := st.SetParagraph(path, startParagraph); err != nil {
			return nil, err
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s", path)
	}
	paragraphs := getParagraphs(string(b))
	if len(paragraphs) == 0 {
		return nil, fmt.Errorf("%s does not contain any text", path)
	}

	return &File{path, paragraphs, st.Paragraph(path) - 1, st}, nil
}

func (s *File) Next() ([]Segment, error) {
	s.idx++
	if err := s.store.SetParagraph(s.path, s.idx); err != nil {
		return nil, fmt.Errorf("could not save the paragraph reached in %s: %v", s.path, err)
	}

	if s.idx >= len(s.paragraphs) {
		return nil, io.EOF
	}

	return []Segment{{s.paragraphs[s.idx], ""}}, nil
}

// blankLines matches the blank lines separating paragraphs.
var blankLines = regexp.MustCompile("\n\n+")

// getParagraphs returns the paragraphs of s which aren't blank.
func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = blankLines.ReplaceAllString(s, "\n\n")

	var paragraphs []string
	for _, p := range strings.Split(strings.Trim(s, "\n"), "\n\n") {
		if strings.TrimSpace(p) != "" {
			paragraphs = append(paragraphs, p)
		}
	}

	return paragraphs
}
