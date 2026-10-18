package main

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell"
)

func TestKeymapActions(t *testing.T) {
	key := func(k tcell.Key, mods tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(k, 0, mods) }
	char := func(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

	common := []struct {
		name string
		ev   *tcell.EventKey
		want action
	}{
		{"?", char('?'), actionToggleHelp},
		{"Esc", key(tcell.KeyEscape, tcell.ModNone), actionRestart},
		{"C-c", key(tcell.KeyCtrlC, tcell.ModCtrl), actionQuit},
		{"C-l", key(tcell.KeyCtrlL, tcell.ModCtrl), actionRedraw},
		{"Right", key(tcell.KeyRight, tcell.ModNone), actionNext},
		{"Left", key(tcell.KeyLeft, tcell.ModNone), actionPrevious},
		{"C-n", key(tcell.KeyCtrlN, tcell.ModCtrl), actionNextTheme},
		{"C-p", key(tcell.KeyCtrlP, tcell.ModCtrl), actionPreviousTheme},
		{"Backspace", key(tcell.KeyBackspace2, tcell.ModNone), actionDeleteChar},
		{"Shift-backspace", key(tcell.KeyBackspace2, tcell.ModShift), actionDeleteChar},
		{"C-w", key(tcell.KeyCtrlW, tcell.ModCtrl), actionDeleteWord},
		{"C-backspace", key(tcell.KeyBackspace2, tcell.ModCtrl), actionDeleteWord},
		{"Alt-backspace", key(tcell.KeyBackspace2, tcell.ModAlt), actionDeleteWord},
		{"a", char('a'), actionNone},
		{"Space", char(' '), actionNone},
		{"Tab", key(tcell.KeyTab, tcell.ModNone), actionNone},
	}

	//Terminals on unix send C-h for control+backspace, windows has a key of its own.
	byOS := map[string][]struct {
		name string
		ev   *tcell.EventKey
		want action
	}{
		"linux": {
			{"C-h", key(tcell.KeyBackspace, tcell.ModNone), actionDeleteWord},
			{"C-h with Ctrl", key(tcell.KeyBackspace, tcell.ModCtrl), actionDeleteWord},
		},
		"windows": {
			{"Backspace", key(tcell.KeyBackspace, tcell.ModNone), actionDeleteChar},
			{"C-backspace", key(tcell.KeyBackspace, tcell.ModCtrl), actionDeleteWord},
			{"Alt-backspace", key(tcell.KeyBackspace, tcell.ModAlt), actionDeleteWord},
		},
	}

	for goos, specific := range byOS {
		km := newKeymap(goos)
		for _, tc := range append(common, specific...) {
			if got := km.action(tc.ev); got != tc.want {
				t.Errorf("%s: %s = action %d, want %d", goos, tc.name, got, tc.want)
			}
		}
	}
}

// The help is generated from the keymap, so it can't miss a key.
func TestKeymapHelp(t *testing.T) {
	want := []string{
		"Help",
		"? - show/hide this help",
		"Esc - restart the current test",
		"C-c - quit",
		"C-l - redraw the terminal",
		"Right - next test",
		"Left - previous test",
		"C-n - next theme",
		"C-p - previous theme",
		"Backspace - delete previous character if enabled",
		"C-w / C-backspace / Alt-backspace - delete previous word if enabled",
		"Space - skip current word if enabled",
	}

	for _, goos := range []string{"linux", "windows"} {
		if got := newKeymap(goos).help(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: help\n%q\nwant\n%q", goos, got, want)
		}
	}
}

// Every action can be given with a key.
func TestKeymapBindsEveryAction(t *testing.T) {
	bound := map[action]bool{}
	for _, b := range newKeymap("linux") {
		if len(b.presses) > 0 {
			bound[b.action] = true
		}
	}

	for a := actionNone + 1; a < actionCount; a++ {
		if !bound[a] {
			t.Errorf("action %d has no key", a)
		}
	}
}
