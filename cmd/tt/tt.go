package main

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-isatty"
)

// scr is the screen in use, if any, so that die can restore the terminal.
var scr tcell.Screen

func die(format string, args ...interface{}) {
	if scr != nil {
		scr.Fini()
	}
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func main() {
	opts := parseFlags(os.Args[1:])
	if err := opts.validate(); err != nil {
		die("%v", err)
	}

	if opts.listType != "" {
		for _, name := range listResourceNames(opts.listType) {
			fmt.Println(name)
		}

		os.Exit(0)
	}

	if opts.version {
		fmt.Println("tt version " + versionString())
		os.Exit(0)
	}

	if opts.stats {
		printStats()
		os.Exit(0)
	}

	if opts.keys {
		printKeys(opts.layout, isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("NO_COLOR") == "")
		os.Exit(0)
	}

	if opts.export != "" {
		if err := exportData(os.Stdout, opts.export, opts.json); err != nil {
			die("%v", err)
		}

		os.Exit(0)
	}

	if opts.noTheme {
		os.Setenv("TCELL_TRUECOLOR", "disable")
	} else if !opts.themeExplicit {
		if rememberedTheme := loadRememberedTheme(opts.opaqueBg); rememberedTheme != "" {
			opts.themeName = rememberedTheme
		}
	}

	//Checked before typing a test whose result could not be saved.
	if _, err := readPersistedResults(); err != nil && !os.IsNotExist(err) {
		die("%v", unreadableResults(err))
	}

	testFn, kind, err := newTestGenerator(opts, os.Stdin, isatty.IsTerminal(os.Stdin.Fd()))
	if err != nil {
		die("%v", err)
	}

	scr, err = tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	if err := scr.Init(); err != nil {
		panic(err)
	}

	defer func() {
		if r := recover(); r != nil {
			scr.Fini()
			panic(r)
		}
	}()

	a := newApp(scr, opts, kind)
	a.exit(a.run(testFn))
}
