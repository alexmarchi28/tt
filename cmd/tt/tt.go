package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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

const usage = `usage: tt [options] [file]

Modes
    -words  WORDFILE    Specifies the file from which words are randomly
                        drawn (default: 1000en).
    -quotes QUOTEFILE   Starts quote mode in which quotes are randomly drawn
                        from the given file. The file should be JSON encoded and
                        have the following form:

                        [{"text": "foo", attribution: "bar"}]

    -mistakes           Practice the recently mistyped words, the most
                        frequently mistyped ones coming up more often.

Word Mode (also applies to -mistakes)
    -n GROUPSZ          Sets the number of words which constitute a group.
    -g NGROUPS          Sets the number of groups which constitute a test.

File Mode
    -start PARAGRAPH    The offset of the starting paragraph, set this to 0 to
                        reset progress on a given file.

Aesthetics
    -showwpm            Display WPM whilst typing.
    -showaccuracy       Display accuracy whilst typing.
    -theme THEMEFILE    The theme to use.
    -w                  The maximum line length in characters. This option is
                        ignored if -raw is present.
    -notheme            Attempt to use the default terminal theme.
                        This may produce odd results depending
                        on the theme colours.
    -notransparent      Disable transparent background.
    -blockcursor        Use the default cursor style.
    -bold               Embolden typed text.

Test Parameters
    -t SECONDS          Terminate the test after the given number of seconds.
    -noskip             Disable word skipping when space is pressed.
    -nobackspace        Disable the backspace key.
    -nohighlight        Disable current and next word highlighting.
    -highlight1         Only highlight the current word.
    -highlight2         Only highlight the next word.

Scripting
    -oneshot            Automatically exit after a single run.
    -noreport           Don't show a report at the end of a test.
    -csv                Print the test results to stdout in the form:
                        [type],[wpm],[cpm],[accuracy],[timestamp].
    -json               Print the test output in JSON.
    -raw                Don't reflow STDIN text or show one paragraph at a time.
                        Note that line breaks are determined exclusively by the
                        input.
    -multi              Treat each input paragraph as a self contained test.

Misc
    -list TYPE          Lists internal resources of the given type.
                        TYPE=[themes|quotes|words]
    -stats              Print statistics about the saved results: number of
                        tests, time spent typing, personal best, averages over
                        time, weekly trend, results by kind of test and most
                        mistyped words.

Version
    -v                  Print the current version.
`

// options holds the command line options, see usage.
type options struct {
	n, g           int
	startParagraph int
	maxLineLen     int
	timeout        time.Duration // -1 for no time limit.

	listType  string
	wordFile  string
	quoteFile string
	themeName string
	files     []string

	themeExplicit      bool // Whether -theme was given.
	mistakes           bool
	showWpm            bool
	showAccuracy       bool
	noSkip             bool
	blockCursor        bool
	noBackspace        bool
	noTheme            bool
	opaqueBg           bool
	oneShot            bool
	noHighlight        bool
	noHighlightCurrent bool
	noHighlightNext    bool
	noReport           bool
	bold               bool
	csv                bool
	json               bool
	raw                bool
	multi              bool
	stats              bool
	version            bool
}

func parseFlags(args []string) options {
	var opts options
	var timeout int

	fs := flag.NewFlagSet("tt", flag.ExitOnError)

	fs.IntVar(&opts.n, "n", 50, "")
	fs.IntVar(&opts.g, "g", 1, "")
	fs.IntVar(&opts.startParagraph, "start", -1, "")

	fs.IntVar(&opts.maxLineLen, "w", 80, "")
	fs.IntVar(&timeout, "t", -1, "")

	fs.BoolVar(&opts.version, "v", false, "")

	fs.StringVar(&opts.wordFile, "words", "", "")
	fs.StringVar(&opts.quoteFile, "quotes", "", "")
	fs.BoolVar(&opts.mistakes, "mistakes", false, "")

	fs.BoolVar(&opts.showWpm, "showwpm", false, "")
	fs.BoolVar(&opts.showAccuracy, "showaccuracy", false, "")
	fs.BoolVar(&opts.noSkip, "noskip", false, "")
	fs.BoolVar(&opts.blockCursor, "blockcursor", false, "")
	fs.BoolVar(&opts.noBackspace, "nobackspace", false, "")
	fs.BoolVar(&opts.noTheme, "notheme", false, "")
	fs.BoolVar(&opts.opaqueBg, "notransparent", false, "")
	fs.BoolVar(&opts.oneShot, "oneshot", false, "")
	fs.BoolVar(&opts.noHighlight, "nohighlight", false, "")
	fs.BoolVar(&opts.noHighlightCurrent, "highlight2", false, "")
	fs.BoolVar(&opts.noHighlightNext, "highlight1", false, "")
	fs.BoolVar(&opts.noReport, "noreport", false, "")
	fs.BoolVar(&opts.bold, "bold", false, "")
	fs.BoolVar(&opts.csv, "csv", false, "")
	fs.BoolVar(&opts.json, "json", false, "")
	fs.BoolVar(&opts.raw, "raw", false, "")
	fs.BoolVar(&opts.multi, "multi", false, "")
	fs.BoolVar(&opts.stats, "stats", false, "")
	fs.StringVar(&opts.themeName, "theme", "default", "")
	fs.StringVar(&opts.listType, "list", "", "")

	fs.Usage = func() { os.Stdout.Write([]byte(usage)) }
	fs.Parse(args)

	fs.Visit(func(f *flag.Flag) {
		if f.Name == "theme" {
			opts.themeExplicit = true
		}
	})

	opts.files = fs.Args()

	opts.timeout = -1
	if timeout != -1 {
		opts.timeout = time.Duration(timeout) * time.Second
	}

	return opts
}

// newTestGenerator returns a function producing the successive tests
// requested on the command line, or nil once there are no more, and what they
// are made of. Text piped to stdin is used unless stdin is a terminal.
func newTestGenerator(opts options, stdin io.Reader, stdinIsTerminal bool) (func() []segment, testKind) {
	switch {
	case opts.wordFile != "":
		return generateWordTest(opts.wordFile, opts.n, opts.g), testKind{"words", filepath.Base(opts.wordFile)}
	case opts.quoteFile != "":
		return generateQuoteTest(opts.quoteFile), testKind{"quotes", filepath.Base(opts.quoteFile)}
	case opts.mistakes:
		return generateMistakesTest(opts.n, opts.g), testKind{"mistakes", ""}
	case !stdinIsTerminal:
		b, err := io.ReadAll(stdin)
		if err != nil {
			panic(err)
		}

		return generateTestFromData(b, opts.raw, opts.multi), testKind{"text", ""}
	case len(opts.files) > 0:
		return generateTestFromFile(opts.files[0], opts.startParagraph), testKind{"file", filepath.Base(opts.files[0])}
	default:
		return generateWordTest("1000en", opts.n, opts.g), testKind{"words", "1000en"}
	}
}

func main() {
	opts := parseFlags(os.Args[1:])

	if opts.listType != "" {
		for _, name := range listResourceNames(opts.listType) {
			fmt.Println(name)
		}

		os.Exit(0)
	}

	if opts.version {
		fmt.Println("tt version 0.4.4")
		os.Exit(0)
	}

	if opts.stats {
		printStats()
		os.Exit(0)
	}

	if opts.noTheme {
		os.Setenv("TCELL_TRUECOLOR", "disable")
	} else if !opts.themeExplicit {
		if rememberedTheme := loadRememberedTheme(opts.opaqueBg); rememberedTheme != "" {
			opts.themeName = rememberedTheme
		}
	}

	testFn, kind := newTestGenerator(opts, os.Stdin, isatty.IsTerminal(os.Stdin.Fd()))

	var err error
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
