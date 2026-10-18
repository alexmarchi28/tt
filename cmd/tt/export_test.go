package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// export returns what exportData prints of st, failing on errors.
func export(t *testing.T, st historyStore, data string, asJSON bool) string {
	t.Helper()

	var b bytes.Buffer
	if err := exportData(&b, st, data, asJSON); err != nil {
		t.Fatalf("exportData(%s): %v", data, err)
	}

	return b.String()
}

func exportedResults() []persistedResult {
	return []persistedResult{
		{Wpm: 40, Accuracy: 80, Timestamp: 1000},
		persistedResult{Wpm: 72, Accuracy: 96.5, Timestamp: 2000, Duration: 12.5}.
			withKind(testKind{mode: "words", source: "1000en", seconds: 15, punct: true, numbers: true}),
		persistedResult{Wpm: 65, Accuracy: 100, Timestamp: 3000, Duration: 30}.
			withKind(testKind{mode: "quotes", source: "my, quotes"}),
		persistedResult{Wpm: 55, Accuracy: 98, Timestamp: 4000, Duration: 20}.
			withKind(testKind{mode: "adaptive", source: "1000en", words: 50, focus: 40}),
	}
}

func TestExportResultsAsCSV(t *testing.T) {
	st := &memStore{results: exportedResults()}

	want := "" +
		"timestamp,wpm,accuracy,duration,mode,source,seconds,words,punct,numbers,focus\n" +
		"1000,40,80.00,0.00,,,0,0,false,false,0\n" +
		"2000,72,96.50,12.50,words,1000en,15,0,true,true,0\n" +
		"3000,65,100.00,30.00,quotes,\"my, quotes\",0,0,false,false,0\n" +
		"4000,55,98.00,20.00,adaptive,1000en,0,50,false,false,40\n"
	if got := export(t, st, "results", false); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestExportResultsAsJSON(t *testing.T) {
	saved := exportedResults()

	var got []persistedResult
	if err := json.Unmarshal([]byte(export(t, &memStore{results: saved}, "results", true)), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Errorf("exported %+v, want the saved results %+v", got, saved)
	}
}

func TestExportMistakes(t *testing.T) {
	st := &memStore{mistakes: []mistake{{"the", "teh"}, {"say,", "sya,"}}}

	want := "word,typed\nthe,teh\n\"say,\",\"sya,\"\n"
	if got := export(t, st, "mistakes", false); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	want = `[{"word":"the","typed":"teh"},{"word":"say,","typed":"sya,"}]` + "\n"
	if got := export(t, st, "mistakes", true); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExportWithoutData(t *testing.T) {
	tests := []struct {
		data   string
		asJSON bool
		want   string
	}{
		{"results", false, "timestamp,wpm,accuracy,duration,mode,source,seconds,words,punct,numbers,focus\n"},
		{"results", true, "[]\n"},
		{"mistakes", false, "word,typed\n"},
		{"mistakes", true, "[]\n"},
	}

	for _, tc := range tests {
		if got := export(t, &memStore{}, tc.data, tc.asJSON); got != tc.want {
			t.Errorf("export %s (JSON %t) = %q, want %q", tc.data, tc.asJSON, got, tc.want)
		}
	}
}

func TestExportErrors(t *testing.T) {
	err := exportData(&bytes.Buffer{}, &memStore{}, "nope", false)
	if err == nil || err.Error() != "unknown data nope, use one of: mistakes, results" {
		t.Errorf("error = %v, want the data which can be exported", err)
	}

	unreadable := &memStore{err: errors.New("unreadable")}
	for _, data := range []string{"results", "mistakes"} {
		if err := exportData(&bytes.Buffer{}, unreadable, data, false); err == nil || err.Error() != "unreadable" {
			t.Errorf("exporting unreadable %s: %v, want the error", data, err)
		}
	}
}
