package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// export returns what exportData prints, failing on errors.
func export(t *testing.T, data string, asJSON bool) string {
	t.Helper()

	var b bytes.Buffer
	if err := exportData(&b, data, asJSON); err != nil {
		t.Fatalf("exportData(%s): %v", data, err)
	}

	return b.String()
}

func saveExportedResults(t *testing.T) []persistedResult {
	t.Helper()

	results := []persistedResult{
		{Wpm: 40, Accuracy: 80, Timestamp: 1000},
		persistedResult{Wpm: 72, Accuracy: 96.5, Timestamp: 2000, Duration: 12.5}.
			withKind(testKind{mode: "words", source: "1000en", seconds: 15, punct: true, numbers: true}),
		persistedResult{Wpm: 65, Accuracy: 100, Timestamp: 3000, Duration: 30}.
			withKind(testKind{mode: "quotes", source: "my, quotes"}),
		persistedResult{Wpm: 55, Accuracy: 98, Timestamp: 4000, Duration: 20}.
			withKind(testKind{mode: "adaptive", source: "1000en", words: 50, focus: 40}),
	}
	if err := writePersistedResults(results); err != nil {
		t.Fatal(err)
	}

	return results
}

func TestExportResultsAsCSV(t *testing.T) {
	useTempStorage(t)
	saveExportedResults(t)

	want := "" +
		"timestamp,wpm,accuracy,duration,mode,source,seconds,words,punct,numbers,focus\n" +
		"1000,40,80.00,0.00,,,0,0,false,false,0\n" +
		"2000,72,96.50,12.50,words,1000en,15,0,true,true,0\n" +
		"3000,65,100.00,30.00,quotes,\"my, quotes\",0,0,false,false,0\n" +
		"4000,55,98.00,20.00,adaptive,1000en,0,50,false,false,40\n"
	if got := export(t, "results", false); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestExportResultsAsJSON(t *testing.T) {
	useTempStorage(t)
	saved := saveExportedResults(t)

	var got []persistedResult
	if err := json.Unmarshal([]byte(export(t, "results", true)), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Errorf("exported %+v, want the saved results %+v", got, saved)
	}
}

func TestExportMistakes(t *testing.T) {
	useTempStorage(t)
	if err := saveMistakes([]mistake{{"the", "teh"}, {"say,", "sya,"}}); err != nil {
		t.Fatal(err)
	}

	want := "word,typed\nthe,teh\n\"say,\",\"sya,\"\n"
	if got := export(t, "mistakes", false); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	want = `[{"word":"the","typed":"teh"},{"word":"say,","typed":"sya,"}]` + "\n"
	if got := export(t, "mistakes", true); got != want {
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
		useTempStorage(t)
		if got := export(t, tc.data, tc.asJSON); got != tc.want {
			t.Errorf("export %s (JSON %t) = %q, want %q", tc.data, tc.asJSON, got, tc.want)
		}
	}
}

func TestExportErrors(t *testing.T) {
	useTempStorage(t)

	err := exportData(&bytes.Buffer{}, "nope", false)
	if err == nil || err.Error() != "unknown data nope, use one of: mistakes, results" {
		t.Errorf("error = %v, want the data which can be exported", err)
	}

	writeFile(t, resultsDB, "not json")
	if err := exportData(&bytes.Buffer{}, "results", false); err == nil || !strings.HasPrefix(err.Error(), "could not read the saved results") {
		t.Errorf("error = %v, want the results to be unreadable", err)
	}

	writeFile(t, mistakeDB, "not json")
	if err := exportData(&bytes.Buffer{}, "mistakes", false); err == nil || !strings.HasPrefix(err.Error(), "could not read the recorded mistakes") {
		t.Errorf("error = %v, want the mistakes to be unreadable", err)
	}
}
