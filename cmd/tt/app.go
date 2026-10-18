package main

import (
	"fmt"
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
	opts    options
	kind    testKind
	scr     tcell.Screen
	store   sessionStore
	now     func() time.Time
	typer   *typer
	results []result
	theme   string         // Theme to remember on exit, empty if none.
	words   *adaptiveWords // The word list of adaptive tests, nil for other tests.
}

func newApp(scr tcell.Screen, opts options, kind testKind, deps appDeps) (*app, error) {
	a := &app{opts: opts, kind: kind, scr: scr, store: deps.store, now: deps.now}
	res := deps.resources

	if kind.mode == modeAdaptive {
		words, err := loadAdaptiveWords(res, adaptiveWordList(opts))
		if err != nil {
			return nil, err
		}
		a.words = &words
	}

	if opts.noTheme {
		a.typer = NewTyper(scr, true, terminalTheme)
	} else {
		th, err := loadTheme(res, opts.themeName, opts.opaqueBg)
		if err != nil {
			return nil, fmt.Errorf("%s theme is invalid: %v", opts.themeName, err)
		}

		a.typer = NewTyper(scr, opts.bold, th)
		a.theme = opts.themeName

		themes := newThemeCycler(res, opts.themeName, opts.opaqueBg)
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

	a.typer.SetHighlightMode(
		!(opts.noHighlightCurrent || opts.noHighlight),
		!(opts.noHighlightNext || opts.noHighlight),
	)

	a.typer.now = deps.now
	a.typer.SkipWord = !opts.noSkip
	a.typer.DisableBackspace = opts.noBackspace
	a.typer.BlockCursor = opts.blockCursor
	a.typer.ShowWpm = opts.showWpm
	a.typer.ShowAccuracy = opts.showAccuracy
	a.typer.Blind = opts.blind
	a.typer.Pace = opts.pace
	if !opts.raw {
		a.typer.Wrap = func(text string, sw int) string {
			return reflow(text, lineWidth(opts.maxLineLen, sw))
		}
	}

	return a, nil
}

// run shows the tests produced by testFn until there are no more or the user
// quits, and returns the exit code. It stops at the first test whose outcome
// can't be saved.
func (a *app) run(testFn func() []segment) (int, error) {
	var tests [][]segment
	var idx = 0

	for {
		if idx >= len(tests) {
			tests = append(tests, testFn())
		}

		if tests[idx] == nil {
			return 0, nil
		}

		if a.words != nil {
			a.typer.ShowStatusMessage(a.words.describe(a.store.KeyStats()), 0)
		}

		res := a.typer.Start(tests[idx], a.opts.timeout)

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

			if !a.opts.noReport {
				attribution := ""
				if len(tests[idx]) == 1 {
					attribution = tests[idx][0].Attribution
				}
				if quit := showReport(a.scr, formatReport(res, attribution, best)); quit {
					return 1, nil
				}
			}
			if a.opts.oneShot {
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
	}.withKind(a.kind))
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
