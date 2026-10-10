package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell"
)

// app runs the sequence of tests requested on the command line.
type app struct {
	opts    options
	kind    testKind
	scr     tcell.Screen
	typer   *typer
	results []result
	theme   string // Theme to remember on exit, empty if none.
}

func newApp(scr tcell.Screen, opts options, kind testKind) *app {
	a := &app{opts: opts, kind: kind, scr: scr}

	if opts.noTheme {
		a.typer = NewTyper(scr, true, terminalTheme)
	} else {
		th, err := loadTheme(opts.themeName, opts.opaqueBg)
		if err != nil {
			die("%s theme is invalid: %v", opts.themeName, err)
		}

		a.typer = NewTyper(scr, opts.bold, th)
		a.theme = opts.themeName

		themes := newThemeCycler(listResourceNames("themes"), opts.themeName, opts.opaqueBg)
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

	a.typer.SkipWord = !opts.noSkip
	a.typer.DisableBackspace = opts.noBackspace
	a.typer.BlockCursor = opts.blockCursor
	a.typer.ShowWpm = opts.showWpm
	a.typer.ShowAccuracy = opts.showAccuracy

	return a
}

// run shows the tests produced by testFn until there are no more or the user
// quits, and returns the exit code.
func (a *app) run(testFn func() []segment) int {
	var tests [][]segment
	var idx = 0

	for {
		if idx >= len(tests) {
			tests = append(tests, testFn())
		}

		if tests[idx] == nil {
			return 0
		}

		if !a.opts.raw {
			sw, _ := a.scr.Size()
			for i := range tests[idx] {
				tests[idx][i].Text = reflow(tests[idx][i].Text, lineWidth(a.opts.maxLineLen, sw))
			}
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
			a.record(res)

			if !a.opts.noReport {
				attribution := ""
				if len(tests[idx]) == 1 {
					attribution = tests[idx][0].Attribution
				}
				if quit := showReport(a.scr, formatReport(res, attribution)); quit {
					return 1
				}
			}
			if a.opts.oneShot {
				return 0
			}

			idx++
		case TyperSigInt:
			return 1

		case TyperResize:
			//Resize events restart the test, this shouldn't be a problem in the vast majority of cases
			//and allows us to avoid baking rewrapping logic into the typer.

			//TODO: implement state-preserving resize (maybe)
		}
	}
}

// record stores the outcome of a completed test.
func (a *app) record(res testResult) {
	timestamp := time.Now().Unix()

	a.results = append(a.results, result{res.wpm(), res.cpm(), res.accuracy(), timestamp, res.mistakes})
	savePersistedResult(persistedResult{
		Wpm:       res.wpm(),
		Accuracy:  res.accuracy(),
		Timestamp: timestamp,
		Mode:      a.kind.mode,
		Source:    a.kind.source,
		Duration:  res.duration.Seconds(),
	})
	saveMistakes(res.mistakes)
}

// exit restores the terminal, remembers the selected theme, prints the
// results if requested and exits.
func (a *app) exit(rc int) {
	a.scr.Fini()
	scr = nil

	if a.theme != "" {
		if err := writeAppConfig(appConfig{Theme: a.theme}); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not save theme selection: %v\n", err)
		}
	}

	if err := writeResults(os.Stdout, a.results, a.opts.json, a.opts.csv); err != nil {
		panic(err)
	}

	os.Exit(rc)
}
