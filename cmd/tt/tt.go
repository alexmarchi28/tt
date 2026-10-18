package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync"
	"time"

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
	now              func() time.Time
	homeDir          func() (string, error)

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
		now:              time.Now,
		homeDir:          os.UserHomeDir, //$HOME on unix, %USERPROFILE% on windows.
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

	home, err := env.homeDir()
	if err != nil {
		return fail(env.stderr, errors.New("could not resolve the home directory"))
	}
	deps := dependencies{
		store:     newJSONStore(env.getenv, home),
		resources: newResources(home, env.stdin),
		rng:       rand.New(rand.NewSource(env.now().UnixNano())),
	}
	if err := opts.validate(); err != nil {
		return fail(env.stderr, err)
	}

	for _, c := range commands {
		if c.requested(opts) {
			if err := c.run(opts, env, deps); err != nil {
				return fail(env.stderr, err)
			}

			return 0
		}
	}

	rc, err := runTests(opts, env, deps)
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

// dependencies are what tt works with besides the environment.
type dependencies struct {
	store     store
	resources resourceLister
	rng       *rand.Rand
}

// commands are the actions of tt other than typing tests, the first one
// requested being run.
var commands = []struct {
	requested func(options) bool
	run       func(options, environment, dependencies) error
}{
	{func(o options) bool { return o.listType != "" }, listCommand},
	{func(o options) bool { return o.version }, versionCommand},
	{func(o options) bool { return o.stats }, statsCommand},
	{func(o options) bool { return o.keys }, keysCommand},
	{func(o options) bool { return o.export != "" }, exportCommand},
}

func listCommand(opts options, env environment, deps dependencies) error {
	for _, name := range deps.resources.list(opts.listType) {
		fmt.Fprintln(env.stdout, name)
	}

	return nil
}

func versionCommand(_ options, env environment, _ dependencies) error {
	_, err := fmt.Fprintln(env.stdout, "tt version "+versionString())
	return err
}

func statsCommand(_ options, env environment, deps dependencies) error {
	return printStats(env.stdout, deps.store, env.now())
}

func keysCommand(opts options, env environment, deps dependencies) error {
	return printKeys(env.stdout, deps.store, opts.layout, env.stdoutIsTerminal && env.getenv("NO_COLOR") == "")
}

func exportCommand(opts options, env environment, deps dependencies) error {
	return exportData(env.stdout, deps.store, opts.export, opts.json)
}

// runTests types the tests requested on the command line and returns the
// exit code.
func runTests(opts options, env environment, deps dependencies) (int, error) {
	st, res := deps.store, deps.resources

	if !opts.noTheme && !opts.themeExplicit {
		if rememberedTheme := loadRememberedTheme(st, res, opts.opaqueBg); rememberedTheme != "" {
			opts.themeName = rememberedTheme
		}
	}

	//Checked before typing a test whose result could not be saved.
	stored, err := st.Results()
	if err != nil {
		return 0, err
	}
	if _, err := st.Mistakes(); err != nil {
		return 0, err
	}

	tests, kind, err := newTestSource(opts, textSources{st, res, deps.rng, env.stdin, env.stdinIsTerminal})
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

	a, err := newApp(scr, opts, kind, appDeps{st, res, env.now})
	if err != nil {
		return 0, err
	}

	rc, runErr := a.run(tests)
	restore()

	if err := a.saveTheme(); err != nil {
		fmt.Fprintf(env.stderr, "WARNING: could not save theme selection: %v\n", err)
	}
	if err := writeResults(env.stdout, a.results, opts.json, opts.csv); err != nil {
		return 0, err
	}

	return rc, runErr
}
