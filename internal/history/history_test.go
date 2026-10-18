package history

import (
	"fmt"
	"testing"
)

func TestKindString(t *testing.T) {
	t.Parallel()

	tests := map[Kind]string{
		{Mode: "words", Source: "1000en"}:                                          "words 1000en",
		{Mode: "words", Source: "1000en", Words: 50}:                               "words 1000en, 50 words",
		{Mode: "words", Source: "1000en", Seconds: 60, Punct: true, Numbers: true}: "words 1000en, 60s, punctuation, numbers",
		{Mode: "adaptive", Source: "1000en", Words: 50, Focus: 40}:                 "adaptive 1000en, 50 words, focus 40%",
		{Mode: "mistakes", Words: 25}:                                              "mistakes, 25 words",
		{Mode: "quotes", Source: "en", Seconds: 30}:                                "quotes en, 30s",
		{Mode: "text"}: "text",
		{}:             "unknown",
	}

	for kind, want := range tests {
		if got := kind.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", kind, got, want)
		}
	}
}

// Results are printed whole, not only their kind as the embedded Kind
// would.
func TestResultString(t *testing.T) {
	t.Parallel()

	r := Result{Wpm: 60, Accuracy: 95.5, Timestamp: 1000, Duration: 30, Kind: Kind{Mode: "quotes", Source: "en"}}

	if got, want := fmt.Sprint(r), "60 wpm, 95.50% accuracy, 30.0s at 1000 (quotes en)"; got != want {
		t.Errorf("printed as %q, want %q", got, want)
	}
}
