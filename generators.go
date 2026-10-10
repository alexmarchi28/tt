package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// The generators below return functions producing the successive tests of a
// session, each made of one or more segments. They return nil once there are
// no more tests.

func generateWordTest(name string, n int, g int) func() []segment {
	var b []byte

	if b = readResource("words", name); b == nil {
		die("%s does not appear to be a valid word list. See '-list words' for a list of builtin word lists.", name)
	}

	words := strings.Fields(string(b))
	if len(words) == 0 {
		die("%s does not contain any words.", name)
	}

	return func() []segment {
		segments := make([]segment, g)
		for i := 0; i < g; i++ {
			segments[i] = segment{randomText(n, words), ""}
		}

		return segments
	}
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
