package main

import (
	"fmt"
	"io"
	"time"

	"github.com/gdamore/tcell"
)

// sessionStore saves the outcome of the tests typed.
type sessionStore interface {
	resultStore
	mistakeStore
	keyStatsStore
	configStore
}

// appDeps are what the app works with.
type appDeps struct {
	store     sessionStore
	resources resourceLister
	now       func() time.Time
}

// app runs the sequence of tests requested on the command line.
type app struct {
	rules   ruleSettings
	output  outputSettings
	kind    testKind
	scr     tcell.Screen
	store   sessionStore
	now     func() time.Time
	typer   *typer
	results []result
	theme   string // Theme to remember on exit, empty if none.
}

func newApp(scr tcell.Screen, s settings, deps appDeps) (*app, error) {
	a := &app{rules: s.rules, output: s.output, kind: s.source.kind, scr: scr, store: deps.store, now: deps.now}
	res, display := deps.resources, s.display

	if display.noTheme {
		a.typer = NewTyper(scr, true, terminalTheme)
	} else {
		th, err := loadTheme(res, display.theme, display.opaqueBg)
		if err != nil {
			return nil, fmt.Errorf("%s theme is invalid: %v", display.theme, err)
		}

		a.typer = NewTyper(scr, display.bold, th)
		a.theme = display.theme

		themes := newThemeCycler(res, display.theme, display.opaqueBg)
		a.typer.CycleTheme = func(direction int) (string, error) {
			name, th, err := themes.next(direction)
			if err != nil || name == "" {
				return name, err
			}

			a.typer.SetTheme(th)
			a.typer.RefreshScreen()
			a.theme = name
			return name, nil
		}
	}

	a.typer.SetHighlightMode(display.highlightCurrent, display.highlightNext)

	a.typer.now = deps.now
	a.typer.SkipWord = !s.rules.noSkip
	a.typer.DisableBackspace = s.rules.noBackspace
	a.typer.BlockCursor = display.blockCursor
	a.typer.ShowWpm = display.showWpm
	a.typer.ShowAccuracy = display.showAccuracy
	a.typer.Blind = s.rules.blind
	a.typer.Pace = s.rules.pace
	if display.reflow {
		a.typer.Wrap = func(text string, sw int) string {
			return reflow(text, lineWidth(display.maxLineLen, sw))
		}
	}

	return a, nil
}

// run shows the tests of src until there are no more or the user quits, and
// returns the exit code. It stops at the first test which can't be made or
// whose outcome can't be saved.
func (a *app) run(src testSource) (int, error) {
	var tests [][]segment
	var idx = 0

	for {
		if idx >= len(tests) {
			test, err := src.next()
			if err == io.EOF {
				return 0, nil
			} else if err != nil {
				return 1, err
			}

			tests = append(tests, test)
		}

		if d, ok := src.(describer); ok {
			a.typer.ShowStatusMessage(d.describe(), 0)
		}

		res := a.typer.Start(tests[idx], a.rules.timeout)

		switch res.rc {
		case TyperNext:
			idx++
		case TyperPrevious:
			if idx > 0 {
				idx--
			}
		case TyperComplete:
			best, err := a.record(res)
			if err != nil {
				return 1, err
			}

			if !a.output.noReport {
				attribution := ""
				if len(tests[idx]) == 1 {
					attribution = tests[idx][0].Attribution
				}
				if quit := showReport(a.scr, formatReport(res, attribution, best)); quit {
					return 1, nil
				}
			}
			if a.output.oneShot {
				return 0, nil
			}

			idx++
		case TyperSigInt:
			return 1, nil
		}
	}
}

// record stores the outcome of a completed test and tells whether it is a
// new personal best.
func (a *app) record(res testResult) (personalBest, error) {
	timestamp := a.now().Unix()

	a.results = append(a.results, result{res.wpm(), res.cpm(), res.accuracy(), timestamp, res.mistakes})
	previous, err := a.store.AddResult(persistedResult{
		Wpm:       res.wpm(),
		Accuracy:  res.accuracy(),
		Timestamp: timestamp,
		Duration:  res.duration.Seconds(),
		testKind:  a.kind,
	})
	if err != nil {
		return personalBest{}, err
	}
	if err := a.store.AddMistakes(res.mistakes); err != nil {
		return personalBest{}, err
	}
	if err := a.store.AddKeystrokes(res.keystrokes); err != nil {
		return personalBest{}, err
	}

	if best, ok := bestWpm(previous, a.kind); ok && res.wpm() > best {
		return personalBest{beaten: true, previous: best}, nil
	}

	return personalBest{}, nil
}

// saveTheme remembers the theme selected last, if any.
func (a *app) saveTheme() error {
	if a.theme == "" {
		return nil
	}

	return a.store.SaveConfig(appConfig{Theme: a.theme})
}
