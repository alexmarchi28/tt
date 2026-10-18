package main

import "testing"

func TestTestKindString(t *testing.T) {
	t.Parallel()

	tests := map[testKind]string{
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
