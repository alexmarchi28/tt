package main

import (
	"errors"
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
    -adaptive           Practice the words of the word list (see -words) with
                        the pairs of letters you type the slowest or miss the
                        most.

Word Mode (also applies to -mistakes and -adaptive)
    -n GROUPSZ          Sets the number of words which constitute a group.
    -g NGROUPS          Sets the number of groups which constitute a test.
    -punct              Turn the words into sentences with capital letters
                        and punctuation.
    -numbers            Replace some of the words with numbers.

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
    -blind              Don't show mistakes while typing, they are still
                        reported at the end of the test.
    -pace WPM           Show a pacer going through the text at the given speed.
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
                        time, weekly trend, a chart of the daily progress,
                        results by kind of test and most mistyped words.
    -keys               Print how accurately and quickly each key is typed on
                        a keyboard.
    -layout LAYOUT      The keyboard layout used by -keys: us, it, de, fr,
                        dvorak or colemak (default: us).

Version
    -v                  Print the current version.
`

// options holds the command line options, see usage.
type options struct {
	n, g           int
	startParagraph int
	maxLineLen     int
	timeout        time.Duration // -1 for no time limit.
	pace           int           // 0 for no pacer.

	listType  string
	layout    string
	wordFile  string
	quoteFile string
	themeName string
	files     []string

	themeExplicit      bool // Whether -theme was given.
	mistakes           bool
	adaptive           bool
	punct              bool
	numbers            bool
	showWpm            bool
	showAccuracy       bool
	noSkip             bool
	blockCursor        bool
	noBackspace        bool
	blind              bool
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
	keys               bool
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
	fs.BoolVar(&opts.adaptive, "adaptive", false, "")
	fs.BoolVar(&opts.punct, "punct", false, "")
	fs.BoolVar(&opts.numbers, "numbers", false, "")

	fs.BoolVar(&opts.showWpm, "showwpm", false, "")
	fs.BoolVar(&opts.showAccuracy, "showaccuracy", false, "")
	fs.BoolVar(&opts.noSkip, "noskip", false, "")
	fs.BoolVar(&opts.blockCursor, "blockcursor", false, "")
	fs.BoolVar(&opts.noBackspace, "nobackspace", false, "")
	fs.BoolVar(&opts.blind, "blind", false, "")
	fs.IntVar(&opts.pace, "pace", 0, "")
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
	fs.BoolVar(&opts.keys, "keys", false, "")
	fs.StringVar(&opts.layout, "layout", "us", "")

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

// validate checks the numeric options.
func (o options) validate() error {
	switch {
	case o.n < 1:
		return errors.New("-n must be at least 1")
	case o.g < 1:
		return errors.New("-g must be at least 1")
	case o.maxLineLen < 1:
		return errors.New("-w must be at least 1")
	case o.timeout != -1 && o.timeout < time.Second:
		return errors.New("-t must be at least 1")
	case o.pace < 0:
		return errors.New("-pace can't be negative")
	}

	return nil
}

// newTestGenerator returns a function producing the successive tests
// requested on the command line, or nil once there are no more, and what they
// are made of. Text piped to stdin is used unless stdin is a terminal.
func newTestGenerator(opts options, stdin io.Reader, stdinIsTerminal bool) (func() []segment, testKind, error) {
	words := wordOptions{n: opts.n, g: opts.g, punctuation: opts.punct, numbers: opts.numbers}

	switch {
	case opts.adaptive:
		list := opts.wordFile
		if list == "" {
			list = "1000en"
		}
		gen, err := generateAdaptiveTest(list, words)
		return gen, testKind{"adaptive", filepath.Base(list)}, err
	case opts.wordFile != "":
		gen, err := generateWordTest(opts.wordFile, words)
		return gen, testKind{"words", filepath.Base(opts.wordFile)}, err
	case opts.quoteFile != "":
		gen, err := generateQuoteTest(opts.quoteFile)
		return gen, testKind{"quotes", filepath.Base(opts.quoteFile)}, err
	case opts.mistakes:
		gen, err := generateMistakesTest(words)
		return gen, testKind{"mistakes", ""}, err
	case !stdinIsTerminal:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, testKind{}, err
		}

		gen, err := generateTestFromData(b, opts.raw, opts.multi)
		return gen, testKind{"text", ""}, err
	case len(opts.files) > 0:
		gen, err := generateTestFromFile(opts.files[0], opts.startParagraph)
		return gen, testKind{"file", filepath.Base(opts.files[0])}, err
	default:
		gen, err := generateWordTest("1000en", words)
		return gen, testKind{"words", "1000en"}, err
	}
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
		fmt.Println("tt version 0.4.4")
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
