package textgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexmarchi28/tt"
	"github.com/alexmarchi28/tt/internal/resource"
	"github.com/alexmarchi28/tt/internal/typing"
)

// newTestResources returns the resources of the calling test: user resources
// in a directory of its own, whose path is returned (see writeUserResource),
// then the builtin ones.
func newTestResources(t *testing.T) (*resource.Layers, string) {
	t.Helper()

	dir := t.TempDir()
	return resource.NewLayers(strings.NewReader(""), os.DirFS(filepath.Join(dir, ".tt")), tt.Builtin), dir
}

// writeUserResource creates a user resource (word list, quote file) inside
// the directory returned by newTestResources.
func writeUserResource(t *testing.T, dir, typ, name, content string) {
	t.Helper()

	path := filepath.Join(dir, ".tt", typ, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// memStore keeps the mistakes and the paragraphs reached in memory. Saving a
// paragraph fails with err if it is set.
type memStore struct {
	mistakes   []typing.Mistake
	paragraphs map[string]int
	err        error
}

func (s *memStore) Mistakes() ([]typing.Mistake, error) { return s.mistakes, nil }

func (s *memStore) Paragraph(path string) int { return s.paragraphs[path] }

func (s *memStore) SetParagraph(path string, paragraph int) error {
	if s.err == nil {
		if s.paragraphs == nil {
			s.paragraphs = map[string]int{}
		}
		s.paragraphs[path] = paragraph
	}
	return s.err
}

// testSequence returns a test generator giving one segment of each text in
// turn, counting its calls, then nothing.
func testSequence(calls *int, texts ...string) func() []Segment {
	return func() []Segment {
		*calls++
		if *calls > len(texts) {
			return nil
		}

		return []Segment{{Text: texts[*calls-1]}}
	}
}
