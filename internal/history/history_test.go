package history

import (
	"encoding/json"
	"fmt"
	"reflect"
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

// The saved results must remain readable by other versions of tt.
func TestResultFormat(t *testing.T) {
	t.Parallel()

	r := Result{Wpm: 60, Accuracy: 95.5, Timestamp: 1000, Duration: 30, Kind: Kind{Mode: "adaptive", Source: "1000en", Seconds: 15, Words: 50, Punct: true, Numbers: true, Focus: 40}}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	// The order of the fields doesn't matter.
	want := `{"wpm":60,"accuracy":95.5,"timestamp":1000,"mode":"adaptive","source":"1000en","duration":30,"seconds":15,"words":50,"punct":true,"numbers":true,"focus":40}`
	var gotFields, wantFields map[string]interface{}
	if err := json.Unmarshal(b, &gotFields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantFields); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Errorf("saved as\n%s\nwant\n%s", b, want)
	}

	// Results saved by older versions have no kind or variant.
	var old Result
	if err := json.Unmarshal([]byte(`{"wpm":60,"accuracy":95.5,"timestamp":1000}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Kind != (Kind{}) {
		t.Errorf("kind of an old result = %+v, want none", old.Kind)
	}
}
