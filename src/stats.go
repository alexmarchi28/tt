package main

import (
	"fmt"
	"os"
	"time"
)

func formatStatsLine(label string, stored []persistedResult, window time.Duration, now time.Time) string {
	var wpmSum int
	var accuracySum float64
	var count int
	cutoff := now.Add(-window).Unix()

	for _, r := range stored {
		if r.Timestamp < cutoff {
			continue
		}

		wpmSum += r.Wpm
		accuracySum += r.Accuracy
		count++
	}

	if count == 0 {
		return fmt.Sprintf("%-10s no results", label)
	}

	return fmt.Sprintf(
		"%-10s avg wpm %.2f, avg accuracy %.2f%% (%d tests)",
		label,
		float64(wpmSum)/float64(count),
		accuracySum/float64(count),
		count,
	)
}

func printStats() {
	stored, err := readPersistedResults()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No saved results yet.")
			return
		}

		die("Could not read saved results: %v", err)
	}

	if len(stored) == 0 {
		fmt.Println("No saved results yet.")
		return
	}

	now := time.Now()
	fmt.Println("Typing stats")
	fmt.Println(formatStatsLine("Last hour", stored, time.Hour, now))
	fmt.Println(formatStatsLine("Last day", stored, 24*time.Hour, now))
	fmt.Println(formatStatsLine("Last week", stored, 7*24*time.Hour, now))
}
