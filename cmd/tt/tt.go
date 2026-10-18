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

	"github.com/alexmarchi28/tt/internal/resource"
	"github.com/alexmarchi28/tt/internal/stats"
	"github.com/alexmarchi28/tt/internal/store"
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
	// openTTY opens the terminal for writing, nil if it can't be.
	openTTY func() (io.WriteCloser, error)
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
		openTTY:          func() (io.WriteCloser, error) { return os.OpenFile("/dev/tty", os.O_WRONLY, 0) },
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
	s, err := resolve(opts, env.stdinIsTerminal)
	if err != nil {
		return fail(env.stderr, err)
	}

	home, err := env.homeDir()
	if err != nil {
		return fail(env.stderr, errors.New("could not resolve the home directory"))
	}
	deps := dependencies{
		store:     store.New(env.getenv, home),
		resources: resource.New(home, env.stdin),
		rng:       rand.New(rand.NewSource(env.now().UnixNano())),
	}
	if s.command != nil {
		if err := s.command.run(s, env, deps); err != nil {
			return fail(env.stderr, err)
		}

		return 0
	}

	rc, err := runTests(s, env, deps)
	if err != nil {
		return fail(env.stderr, err)
	}

	return rc
}

// fail reports err and returns the exit code of a failure.
func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "ERROR: %v\n", err)
	return 1
}

// dependencies are what tt works with besides the environment.
type dependencies struct {
	store     dataStore
	resources resourceLister
	rng       *rand.Rand
}

// command is an action of tt other than typing tests.
type command struct {
	flag  string // The flag requesting it.
	given func(o options) bool
	run   func(s settings, env environment, deps dependencies) error
}

// commands are the actions of tt other than typing tests, at most one of
// which can be requested, see requestedCommand.
var commands = []command{
	{"list", func(o options) bool { return o.listType != "" }, listCommand},
	{"v", func(o options) bool { return o.version }, versionCommand},
	{"stats", func(o options) bool { return o.stats }, statsCommand},
	{"keys", func(o options) bool { return o.keys }, keysCommand},
	{"export", func(o options) bool { return o.export != "" }, exportCommand},
}

func listCommand(s settings, env environment, deps dependencies) error {
	for _, name := range deps.resources.List(s.list) {
		if _, err := fmt.Fprintln(env.stdout, name); err != nil {
			return err
		}
	}

	return nil
}

func versionCommand(_ settings, env environment, _ dependencies) error {
	_, err := fmt.Fprintln(env.stdout, "tt version "+versionString())
	return err
}

func statsCommand(_ settings, env environment, deps dependencies) error {
	return printStats(env.stdout, deps.store, env.now())
}

func keysCommand(s settings, env environment, deps dependencies) error {
	return printKeys(env.stdout, deps.store, s.layout, env.stdoutIsTerminal && env.getenv("NO_COLOR") == "")
}

func exportCommand(s settings, env environment, deps dependencies) error {
	return exportData(env.stdout, deps.store, s.export, s.output.format == "json")
}

// runTests types the tests requested on the command line and returns the
// exit code.
func runTests(s settings, env environment, deps dependencies) (int, error) {
	st, res := deps.store, deps.resources

	if !s.display.noTheme && !s.display.themeExplicit {
		if rememberedTheme := loadRememberedTheme(st, res, s.display.opaqueBg); rememberedTheme != "" {
			s.display.theme = rememberedTheme
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

	tests, err := newTestSource(s.source, textSources{st, res, deps.rng, env.stdin})
	if err != nil {
		return 0, err
	}

	if s.rules.paceTarget != "" {
		if s.rules.pace, err = stats.Pace(s.rules.paceTarget, stored, s.source.kind); err != nil {
			return 0, err
		}
	}

	scr, err := env.newScreen(!s.display.noTheme)
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

	cursor := cursorShaper(keptCursor{})
	if !s.display.blockCursor && env.openTTY != nil {
		//Fails on windows, where tt is still mostly usable via tcell.
		if tty, err := env.openTTY(); err == nil {
			defer func() { _ = tty.Close() }()
			cursor = ttyCursor{tty}
		}
	}

	a, err := newApp(scr, s, appDeps{st, res, env.now, cursor})
	if err != nil {
		return 0, err
	}

	rc, runErr := a.run(tests)
	restore()

	if err := a.saveTheme(); err != nil {
		_, _ = fmt.Fprintf(env.stderr, "WARNING: could not save theme selection: %v\n", err)
	}
	if err := writeResults(env.stdout, a.results, s.output.format); err != nil {
		return 0, err
	}

	return rc, runErr
}
