package main

import (
	"bytes"
	"testing"
	"time"
)

var testResults = []result{
	{Wpm: 60, Cpm: 300, Accuracy: 97.5, Timestamp: 1000},
	{Wpm: 50, Cpm: 250, Accuracy: 100, Timestamp: 2000, Mistakes: []mistake{{"alpha", "alpah"}, {"beta", "bet_"}}},
}

func TestWriteResults(t *testing.T) {
	tests := []struct {
		name          string
		results       []result
		asJSON, asCSV bool
		want          string
	}{
		{"nothing requested", testResults, false, false, ""},
		{
			"json", testResults, true, false,
			`[{"wpm":60,"cpm":300,"accuracy":97.5,"timestamp":1000,"mistakes":[]},` +
				`{"wpm":50,"cpm":250,"accuracy":100,"timestamp":2000,"mistakes":[{"word":"alpha","typed":"alpah"},{"word":"beta","typed":"bet_"}]}]`,
		},
		{
			"csv", testResults, false, true,
			"test,60,300,97.50,1000\n" +
				"test,50,250,100.00,2000\n" +
				"mistake,alpha,alpah\n" +
				"mistake,beta,bet_\n",
		},
		{"csv without results", nil, false, true, ""},
		{"json without results", nil, true, false, "[]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := writeResults(&b, tc.results, tc.asJSON, tc.asCSV); err != nil {
				t.Fatal(err)
			}

			if b.String() != tc.want {
				t.Errorf("got\n%s\nwant\n%s", b.String(), tc.want)
			}
		})
	}
}

func TestWriteResultsEscapesCSV(t *testing.T) {
	results := []result{{Wpm: 40, Cpm: 200, Accuracy: 90, Timestamp: 3000, Mistakes: []mistake{
		{"world,", "worl,"},
		{`"quoted"`, `"quoted_`},
	}}}

	var b bytes.Buffer
	if err := writeResults(&b, results, false, true); err != nil {
		t.Fatal(err)
	}

	want := "test,40,200,90.00,3000\n" +
		"mistake,\"world,\",\"worl,\"\n" +
		"mistake,\"\"\"quoted\"\"\",\"\"\"quoted_\"\n"
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}

func TestFormatReport(t *testing.T) {
	res := testResult{
		duration:     time.Minute,
		correctChars: 300,
		correctKeys:  39,
		wrongKeys:    1,
		mistakes:     []mistake{{"alpha", "alpah"}, {"beta", "bet_"}},
	}

	tests := []struct {
		name        string
		res         testResult
		attribution string
		want        string
	}{
		{"plain", testResult{duration: time.Minute, correctChars: 250, correctKeys: 10}, "", "" +
			"WPM:         50\n" +
			"CPM:         250\n" +
			"Accuracy:    100.00%"},
		{"mistakes and attribution", res, "Someone", "" +
			"WPM:         60\n" +
			"CPM:         300\n" +
			"Accuracy:    97.50%\n" +
			"Mistakes:    alpha, beta\n" +
			"\n" +
			"Attribution: Someone"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatReport(tc.res, tc.attribution); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
