package main

import "github.com/gdamore/tcell"

// action is what a key does during a test, other than typing.
type action int

const (
	actionNone action = iota // The key is typed, if it can be.
	actionToggleHelp
	actionRestart
	actionQuit
	actionRedraw
	actionNext
	actionPrevious
	actionNextTheme
	actionPreviousTheme
	actionDeleteChar
	actionDeleteWord
	actionCount // Number of actions.
)

// keyPress is a key pressed with any modifiers, or only with mods if exact.
type keyPress struct {
	key   tcell.Key
	r     rune // The character of tcell.KeyRune.
	mods  tcell.ModMask
	exact bool
}

func press(key tcell.Key) keyPress { return keyPress{key: key} }

func pressWith(key tcell.Key, mods tcell.ModMask) keyPress {
	return keyPress{key: key, mods: mods, exact: true}
}

func pressRune(r rune) keyPress { return keyPress{key: tcell.KeyRune, r: r} }

func (p keyPress) matches(ev *tcell.EventKey) bool {
	return ev.Key() == p.key &&
		(p.key != tcell.KeyRune || ev.Rune() == p.r) &&
		(!p.exact || ev.Modifiers() == p.mods)
}

// binding gives an action to keys.
type binding struct {
	keys    string // The keys as shown in the help.
	help    string
	action  action
	presses []keyPress // None for keys handled while typing, only shown in the help.
}

// keymap holds the keys of a test, from which their help is generated.
type keymap []binding

// newKeymap returns the keys of tests on the given operating system.
func newKeymap(goos string) keymap {
	deleteChar := []keyPress{press(tcell.KeyBackspace2)}
	deleteWord := []keyPress{
		press(tcell.KeyCtrlW),
		pressWith(tcell.KeyBackspace2, tcell.ModCtrl),
		pressWith(tcell.KeyBackspace2, tcell.ModAlt),
	}
	if goos == "windows" {
		deleteChar = append(deleteChar, press(tcell.KeyBackspace))
		deleteWord = append(deleteWord, pressWith(tcell.KeyBackspace, tcell.ModCtrl), pressWith(tcell.KeyBackspace, tcell.ModAlt))
	} else {
		//Terminals on unix send C-h (tcell.KeyBackspace) for control+backspace.
		deleteWord = append(deleteWord, press(tcell.KeyBackspace))
	}

	return keymap{
		{"?", "show/hide this help", actionToggleHelp, []keyPress{pressRune('?')}},
		{"Esc", "restart the current test", actionRestart, []keyPress{press(tcell.KeyEscape)}},
		{"C-c", "quit", actionQuit, []keyPress{press(tcell.KeyCtrlC)}},
		{"C-l", "redraw the terminal", actionRedraw, []keyPress{press(tcell.KeyCtrlL)}},
		{"Right", "next test", actionNext, []keyPress{press(tcell.KeyRight)}},
		{"Left", "previous test", actionPrevious, []keyPress{press(tcell.KeyLeft)}},
		{"C-n", "next theme", actionNextTheme, []keyPress{press(tcell.KeyCtrlN)}},
		{"C-p", "previous theme", actionPreviousTheme, []keyPress{press(tcell.KeyCtrlP)}},
		{"Backspace", "delete previous character if enabled", actionDeleteChar, deleteChar},
		{"C-w / C-backspace / Alt-backspace", "delete previous word if enabled", actionDeleteWord, deleteWord},
		{"Space", "skip current word if enabled", actionNone, nil},
	}
}

// action returns the action of the key pressed, keys pressed with exactly
// the modifiers of a binding taking precedence over keys pressed with any
// modifiers.
func (km keymap) action(ev *tcell.EventKey) action {
	for _, exact := range []bool{true, false} {
		for _, b := range km {
			for _, p := range b.presses {
				if p.exact == exact && p.matches(ev) {
					return b.action
				}
			}
		}
	}

	return actionNone
}

// help returns the lines of the help of the keys.
func (km keymap) help() []string {
	lines := []string{"Help"}
	for _, b := range km {
		lines = append(lines, b.keys+" - "+b.help)
	}

	return lines
}
