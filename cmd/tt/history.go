package main

// testHistory keeps the tests made by a source, so that the user can go back
// to the previous ones. A test is made when first needed.
type testHistory struct {
	src   testSource
	tests [][]segment
	idx   int
}

func newTestHistory(src testSource) *testHistory {
	return &testHistory{src: src}
}

// current returns the test reached, making it if needed. Its error is that of
// the source, io.EOF once there are no more tests.
func (h *testHistory) current() ([]segment, error) {
	if h.idx == len(h.tests) {
		test, err := h.src.next()
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
