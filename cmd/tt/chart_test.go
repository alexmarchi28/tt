package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexmarchi28/tt/internal/history"
)

func TestDailyAverages(t *testing.T) {
	t.Parallel()

	stored := []history.Result{
		resultAt(10, 0, 30*24*time.Hour), // Too old.
		resultAt(40, 0, 29*24*time.Hour),
		resultAt(70, 0, 30*time.Minute),
		resultAt(74, 0, time.Minute),
	}

	got := dailyAverages(stored, 30, statsNow)
	if len(got) != 30 {
		t.Fatalf("got %d days, want 30", len(got))
	}

	if got[0].date.Format("2006-01-02") != "2026-09-11" || got[29].date.Format("2006-01-02") != "2026-10-10" {
		t.Errorf("days go from %s to %s, want 2026-09-11 to 2026-10-10", got[0].date, got[29].date)
	}

	if got[0].count != 1 || got[0].wpm != 40 || got[29].count != 2 || got[29].wpm != 72 {
		t.Errorf("first day %+v, last day %+v, want 40 wpm (1 test) and 72 wpm (2 tests)", got[0], got[29])
	}

	for _, d := range got[1:29] {
		if d.count != 0 {
			t.Errorf("unexpected tests on %s: %+v", d.date, d)
		}
	}
}

// chartDays returns days with the given averages, NaN standing for days
// without tests.
func chartDays(wpm ...float64) []dayAverage {
	days := make([]dayAverage, len(wpm))
	start := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	for i, w := range wpm {
		days[i].date = start.AddDate(0, 0, i)
		if !math.IsNaN(w) {
			days[i].wpm, days[i].count = w, 1
		}
	}

	return days
}

func TestProgressChart(t *testing.T) {
	t.Parallel()

	nan := math.NaN()
	wpm := make([]float64, 30)
	for i := range wpm {
		wpm[i] = nan
	}
	wpm[0], wpm[14], wpm[28], wpm[29] = 40, 55, 60, 72

	// The scale goes from 40 to 80 wpm over 8 rows of 8 levels: 0.625 wpm per level.
	want := []string{
		"80 │",
		"   │                             ▃",
		"   │                             █",
		"   │                             █",
		"   │                            ██",
		"   │              █             ██",
		"   │              █             ██",
		"40 │▁             █             ██",
		"   └──────────────────────────────",
		"    09-11                    10-10",
	}

	if got := progressChart(chartDays(wpm...)); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestProgressChartScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		wpm    []float64
		lo, hi string
	}{
		{"equal values", []float64{50, 50}, "50", "60"},
		{"round maximum", []float64{63, 70}, "60", "70"},
		{"three digits", []float64{95, 104}, " 90", "110"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := progressChart(chartDays(tc.wpm...))
			if !strings.HasPrefix(got[0], tc.hi+" │") || !strings.HasPrefix(got[chartRows-1], tc.lo+" │") {
				t.Errorf("chart scale is not from %s to %s:\n%s", tc.lo, tc.hi, strings.Join(got, "\n"))
			}
		})
	}
}

func TestProgressChartShowsEveryDayWithTests(t *testing.T) {
	t.Parallel()

	// The lowest average still gets a visible column.
	got := progressChart(chartDays(60, 60.1, 69.9))
	if bottom := got[chartRows-1]; !strings.HasSuffix(bottom, "▁▁█") {
		t.Errorf("bottom row = %q, want every day visible", bottom)
	}
}

func TestProgressChartRoundsHeights(t *testing.T) {
	t.Parallel()

	// From 40 to 80 wpm a level is 0.625 wpm: 41 wpm is 1.6 levels, shown as 2.
	got := progressChart(chartDays(40, 41, 80))
	if bottom := got[chartRows-1]; !strings.HasSuffix(bottom, "▁▂█") {
		t.Errorf("bottom row = %q, want heights rounded to the nearest level", bottom)
	}
}
