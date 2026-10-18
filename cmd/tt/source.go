package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
)

// testSource produces the successive tests of a session, each made of one or
// more segments.
type testSource interface {
	// next returns the next test, or io.EOF once there are no more.
	next() ([]segment, error)
}

// describer is implemented by the test sources telling what their tests
// practice, which is shown before each of them.
type describer interface {
	describe() string
}

// funcSource produces the tests returned by a function, which returns nil
// once there are no more.
type funcSource func() []segment

func (f funcSource) next() ([]segment, error) {
	if test := f(); test != nil {
		return test, nil
	}

	return nil, io.EOF
}

// adaptiveSource draws words from a word list according to the latest key
// statistics, see adaptiveWords.draw.
type adaptiveSource struct {
	words adaptiveWords
	rng   *rand.Rand
	o     wordOptions
	share float64 // Share of the words with the pairs of letters practiced.
	stats keyStatsStore
}

// newAdaptiveSource draws words from the named word list, focus percent of
// them with the pairs of letters to practice.
func newAdaptiveSource(rng *rand.Rand, res resourceReader, name string, o wordOptions, focus int, stats keyStatsStore) (*adaptiveSource, error) {
	words, err := loadAdaptiveWords(res, name)
	if err != nil {
		return nil, err
	}

	return &adaptiveSource{words, rng, o, float64(focus) / 100, stats}, nil
}

func (s *adaptiveSource) next() ([]segment, error) {
	ks := s.stats.KeyStats()
	return wordTest(s.rng, func(n int) []string { return s.words.draw(s.rng, n, ks, s.share) }, s.o)(), nil
}

func (s *adaptiveSource) describe() string {
	return s.words.describe(s.stats.KeyStats())
}

// fileSource makes a test of each paragraph of a file, remembering the
// paragraph reached.
type fileSource struct {
	path       string
	paragraphs []string
	idx        int // The paragraph of the last test.
	store      paragraphStore
}

// newFileSource makes tests of the paragraphs of the file at path, starting
// from the given paragraph, or else from the one reached last.
func newFileSource(path string, startParagraph int, st paragraphStore) (*fileSource, error) {
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

	return &fileSource{path, paragraphs, st.Paragraph(path) - 1, st}, nil
}

func (s *fileSource) next() ([]segment, error) {
	s.idx++
	if err := s.store.SetParagraph(s.path, s.idx); err != nil {
		return nil, fmt.Errorf("could not save the paragraph reached in %s: %v", s.path, err)
	}

	if s.idx >= len(s.paragraphs) {
		return nil, io.EOF
	}

	return []segment{{s.paragraphs[s.idx], ""}}, nil
}
