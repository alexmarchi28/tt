package main

import "testing"

func TestTestKindString(t *testing.T) {
	tests := map[testKind]string{
		{mode: "words", source: "1000en"}:                                          "words 1000en",
		{mode: "words", source: "1000en", words: 50}:                               "words 1000en, 50 words",
		{mode: "words", source: "1000en", seconds: 60, punct: true, numbers: true}: "words 1000en, 60s, punctuation, numbers",
		{mode: "adaptive", source: "1000en", words: 50, focus: 40}:                 "adaptive 1000en, 50 words, focus 40%",
		{mode: "mistakes", words: 25}:                                              "mistakes, 25 words",
		{mode: "quotes", source: "en", seconds: 30}:                                "quotes en, 30s",
		{mode: "text"}: "text",
		{}:             "unknown",
	}

	for kind, want := range tests {
		if got := kind.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", kind, got, want)
		}
	}
}
