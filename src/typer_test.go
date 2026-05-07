package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

func assertWordPosition(t *testing.T, positions []int, text, word string, want int) {
	t.Helper()

	start := strings.Index(text, word)
	if start == -1 {
		t.Fatalf("word %q not found in %q", word, text)
	}

	for i := start; i < start+len(word); i++ {
		if positions[i] != want {
			t.Fatalf("word %q at char %d: got %d, want %d", word, i-start, positions[i], want)
		}
	}
}

func TestHighlightWordPositionsAcrossWrappedLineBreak(t *testing.T) {
	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, "beta")

	positions := highlightWordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "beta", 0)
	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func TestHighlightWordPositionsWhenCursorIsOnWrapWhitespace(t *testing.T) {
	text := "alpha beta \ngamma delta"
	idx := strings.Index(text, " \n")

	positions := highlightWordPositions([]rune(text), idx)

	assertWordPosition(t, positions, text, "gamma", 1)
	assertWordPosition(t, positions, text, "delta", 2)
}

func simulationScreenText(sim tcell.SimulationScreen) string {
	cells, width, height := sim.GetContents()
	var b strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := cells[y*width+x]
			if len(cell.Runes) == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(cell.Runes[0])
			}
		}
		b.WriteRune('\n')
	}

	return b.String()
}

func waitForScreenText(t *testing.T, sim tcell.SimulationScreen, needle string, want bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(simulationScreenText(sim), needle) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("screen contains %q = %t, want %t\n%s", needle, !want, want, simulationScreenText(sim))
}

func TestQuestionMarkTogglesKeyboardHelp(t *testing.T) {
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer sim.Fini()

	oldScr := scr
	scr = sim
	defer func() {
		scr = oldScr
	}()

	typer := NewTyper(sim, true, tcell.ColorWhite, tcell.ColorBlack, tcell.ColorGreen, tcell.ColorBlue, tcell.ColorYellow, tcell.ColorRed)
	done := make(chan int, 1)
	go func() {
		_, rc, _, _, _, _ := typer.start("alpha beta", -1, false, "")
		done <- rc
	}()

	sim.InjectKey(tcell.KeyRune, '?', tcell.ModNone)
	waitForScreenText(t, sim, "Help", true)
	waitForScreenText(t, sim, "C-n - next theme", true)

	sim.InjectKey(tcell.KeyRune, '?', tcell.ModNone)
	waitForScreenText(t, sim, "Help", false)

	sim.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	select {
	case rc := <-done:
		if rc != TyperEscape {
			t.Fatalf("rc = %d, want %d", rc, TyperEscape)
		}
	case <-time.After(time.Second):
		t.Fatal("typer did not exit after escape")
	}
}
