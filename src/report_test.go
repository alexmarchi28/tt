package main

import (
	"bytes"
	"testing"
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
