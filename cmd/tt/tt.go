package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-isatty"
)

// environment is what tt takes from the process it runs in, replaced in
// tests.
type environment struct {
	stdin            io.Reader
	stdout, stderr   io.Writer
	stdinIsTerminal  bool
	stdoutIsTerminal bool
	getenv           func(key string) string

	// newScreen returns the screen tests are typed on, with true colour
	// disabled unless trueColor is set.
	newScreen func(trueColor bool) (tcell.Screen, error)
}

func processEnvironment() environment {
	return environment{
		stdin:            os.Stdin,
		stdout:           os.Stdout,
		stderr:           os.Stderr,
		stdinIsTerminal:  isatty.IsTerminal(os.Stdin.Fd()),
		stdoutIsTerminal: isatty.IsTerminal(os.Stdout.Fd()),
		getenv:           os.Getenv,
		newScreen: func(trueColor bool) (tcell.Screen, error) {
			if !trueColor {
				if err := os.Setenv("TCELL_TRUECOLOR", "disable"); err != nil {
					return nil, err
				}
			}

			return tcell.NewScreen()
		},
	}
}

func main() {
	os.Exit(run(os.Args[1:], processEnvironment()))
}

// run runs tt with the given command line arguments and returns its exit
// code. It is the only place deciding how tt exits.
func run(args []string, env environment) int {
	opts, err := parseArgs(args, env.stdout, env.stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2 //The flag package reports the error.
	}

	if storageErr != nil {
		return fail(env.stderr, storageErr)
	}
	if err := opts.validate(); err != nil {
		return fail(env.stderr, err)
	}

	for _, c := range commands {
		if c.requested(opts) {
			if err := c.run(opts, env); err != nil {
				return fail(env.stderr, err)
			}

			return 0
		}
	}

	rc, err := runTests(opts, env)
	if err != nil {
		return fail(env.stderr, err)
	}

	return rc
}

// fail reports err and returns the exit code of a failure.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "ERROR: %v\n", err)
	return 1
}

// commands are the actions of tt other than typing tests, the first one
// requested being run.
var commands = []struct {
	requested func(options) bool
	run       func(options, environment) error
}{
	{func(o options) bool { return o.listType != "" }, listCommand},
	{func(o options) bool { return o.version }, versionCommand},
	{func(o options) bool { return o.stats }, statsCommand},
	{func(o options) bool { return o.keys }, keysCommand},
	{func(o options) bool { return o.export != "" }, exportCommand},
}

func listCommand(opts options, env environment) error {
	for _, name := range listResourceNames(opts.listType) {
		fmt.Fprintln(env.stdout, name)
	}

	return nil
}

func versionCommand(_ options, env environment) error {
	_, err := fmt.Fprintln(env.stdout, "tt version "+versionString())
	return err
}

func statsCommand(_ options, env environment) error {
	return printStats(env.stdout)
}

func keysCommand(opts options, env environment) error {
	return printKeys(env.stdout, opts.layout, env.stdoutIsTerminal && env.getenv("NO_COLOR") == "")
}

func exportCommand(opts options, env environment) error {
	return exportData(env.stdout, opts.export, opts.json)
}

// runTests types the tests requested on the command line and returns the
// exit code.
func runTests(opts options, env environment) (int, error) {
	if !opts.noTheme && !opts.themeExplicit {
		if rememberedTheme := loadRememberedTheme(opts.opaqueBg); rememberedTheme != "" {
			opts.themeName = rememberedTheme
		}
	}

	//Checked before typing a test whose result could not be saved.
	stored, err := readPersistedResults()
	if err != nil {
		return 0, err
	}
	if _, err := readMistakes(); err != nil {
		return 0, err
	}

	testFn, kind, err := newTestGenerator(opts, env.stdin, env.stdinIsTerminal)
	if err != nil {
		return 0, err
	}

	if opts.paceTarget != "" {
		if opts.pace, err = paceFromResults(opts.paceTarget, stored, kind); err != nil {
			return 0, err
		}
	}

	scr, err := env.newScreen(!opts.noTheme)
	if err != nil {
		return 0, err
	}
	if err := scr.Init(); err != nil {
		return 0, err
	}

	//The terminal is restored before anything is printed, and even on panics.
	var fini sync.Once
	restore := func() { fini.Do(scr.Fini) }
	defer restore()

	a, err := newApp(scr, opts, kind)
	if err != nil {
		return 0, err
	}

	rc, runErr := a.run(testFn)
	restore()

	if err := a.saveTheme(); err != nil {
		fmt.Fprintf(env.stderr, "WARNING: could not save theme selection: %v\n", err)
	}
	if err := writeResults(env.stdout, a.results, opts.json, opts.csv); err != nil {
		return 0, err
	}

	return rc, runErr
}
