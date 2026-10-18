package main

import "github.com/alexmarchi28/tt/internal/textgen"

// testHistory keeps the tests made by a source, so that the user can go back
// to the previous ones. A test is made when first needed.
type testHistory struct {
	src   textgen.Source
	tests [][]textgen.Segment
	idx   int
}

func newTestHistory(src textgen.Source) *testHistory {
	return &testHistory{src: src}
}

// current returns the test reached, making it if needed. Its error is that of
// the source, io.EOF once there are no more tests.
func (h *testHistory) current() ([]textgen.Segment, error) {
	if h.idx == len(h.tests) {
		test, err := h.src.Next()
		if err != nil {
			return nil, err
		}

		h.tests = append(h.tests, test)
	}

	return h.tests[h.idx], nil
}

// next moves on to the following test.
func (h *testHistory) next() {
	h.idx++
}

// previous goes back to the previous test, if any.
func (h *testHistory) previous() {
	if h.idx > 0 {
		h.idx--
	}
}
