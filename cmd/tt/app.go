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
	cursor    cursorShaper // Shapes the cursor while typing.
}

// app runs the sequence of tests requested on the command line.
type app struct {
	rules    ruleSettings
	output   outputSettings
	kind     testKind
	store    sessionStore
	now      func() time.Time
	typer    *typer
	reporter reporter
	results  []result
	theme    string // Theme to remember on exit, empty if none.
}

func newApp(scr tcell.Screen, s settings, deps appDeps) (*app, error) {
	a := &app{rules: s.rules, output: s.output, kind: s.source.kind, store: deps.store, now: deps.now}
	res, display := deps.resources, s.display

	opts := []typerOption{
		withRules(typingRules{noSkip: s.rules.noSkip, noBackspace: s.rules.noBackspace}),
		withPace(s.rules.pace),
		withClock(deps.now),
		withCursor(deps.cursor),
	}
	if display.reflow {
		opts = append(opts, withWrap(func(text string, sw int) string {
			return reflow(text, lineWidth(display.maxLineLen, sw))
		}))
	}

	viewOpts := viewOptions{
		bold:             display.bold || display.noTheme, //Typed text can't be told apart by its colour.
		highlightCurrent: display.highlightCurrent,
		highlightNext:    display.highlightNext,
		blind:            s.rules.blind,
		showWpm:          display.showWpm,
		showAccuracy:     display.showAccuracy,
	}

	if display.noTheme {
		a.useTyper(newTyper(scr, newView(scr, terminalTheme, viewOpts), opts...))
		return a, nil
	}

	th, err := loadTheme(res, display.theme, display.opaqueBg)
	if err != nil {
		return nil, fmt.Errorf("%s theme is invalid: %v", display.theme, err)
	}
	a.theme = display.theme

	v := newView(scr, th, viewOpts)
	themes := newThemeCycler(res, display.theme, display.opaqueBg)
	cycle := func(direction int) (string, error) {
		name, th, err := themes.next(direction)
		if err != nil || name == "" {
			return name, err
		}

		v.setTheme(th)
		v.clear()
		a.theme = name
		return name, nil
	}

	a.useTyper(newTyper(scr, v, append(opts, withThemeCycler(cycle))...))
	return a, nil
}

// useTyper makes the app run its tests with ty, showing the reports on its
// screen unless they were disabled.
func (a *app) useTyper(ty *typer) {
	a.typer = ty
	a.reporter = screenReporter{ty.view}
	if a.output.noReport {
		a.reporter = noReporter{}
	}
}

// run shows the tests of src until there are no more or the user quits, and
// returns the exit code. It stops at the first test which can't be made or
// whose outcome can't be saved.
func (a *app) run(src testSource) (int, error) {
	history := newTestHistory(src)

	for {
		test, err := history.current()
		if err == io.EOF {
			return 0, nil
		} else if err != nil {
			return 1, err
		}

		if d, ok := src.(describer); ok {
			a.typer.showStatusMessage(d.describe(), 0)
		}

		switch o, res := a.typer.Start(test, a.rules.timeout); o {
		case outcomeNext:
			history.next()
		case outcomePrevious:
			history.previous()
		case outcomeComplete:
			if stop, rc, err := a.complete(test, res); stop {
				return rc, err
			}

			history.next()
		case outcomeRestart: //The same test is shown again.
		case outcomeQuit:
			return 1, nil
		}
	}
}

// complete records and reports a completed test. It tells whether the app
// must stop, with its exit code: the outcome can't be saved, the user quit
// from the report, or a single test was requested.
func (a *app) complete(test []segment, res testResult) (stop bool, rc int, err error) {
	best, err := a.record(res)
	if err != nil {
		return true, 1, err
	}

	if quit := a.reporter.show(formatReport(res, attributionOf(test), best)); quit {
		return true, 1, nil
	}
	if a.output.oneShot {
		return true, 0, nil
	}

	return false, 0, nil
}

// attributionOf returns the attribution of a test made of a single text, such
// as a quote, and nothing for tests mixing several texts.
func attributionOf(test []segment) string {
	if len(test) == 1 {
		return test[0].Attribution
	}

	return ""
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
