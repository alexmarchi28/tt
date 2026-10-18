package main

import (
	"errors"
	"flag"
	"io"
	"strconv"
	"time"
)

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
    -focus PERCENT      The percentage of the words of -adaptive drawn among
                        those with the pairs of letters being practiced
                        (default: 40).

    Only one mode can be used at a time. Text piped to stdin is typed unless
    a mode or a file is given.

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
                        Word tests have enough words to last until then.
    -noskip             Disable word skipping when space is pressed.
    -nobackspace        Disable the backspace key.
    -blind              Don't show mistakes while typing, they are still
                        reported at the end of the test.
    -pace WPM|best|avg  Show a pacer going through the text at the given speed,
                        or at your best or recent average speed on the same
                        kind of test.
    -nohighlight        Disable current and next word highlighting.
    -highlight1         Only highlight the current word.
    -highlight2         Only highlight the next word.

Scripting
    -oneshot            Automatically exit after a single run.
    -noreport           Don't show a report at the end of a test.
    -csv                Print the test results to stdout in the form:
                        [type],[wpm],[cpm],[accuracy],[timestamp].
    -json               Print the test output in JSON, see also -export.
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
    -layout LAYOUT      The keyboard layout used by -keys: us, uk, it, es, de,
                        fr, dvorak, colemak, colemak-dh or workman
                        (default: us).
    -export DATA        Print the saved results or mistakes (DATA=results or
                        mistakes) as CSV, or as JSON with -json.

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
	focus          int           // Percentage of the words of -adaptive with the pairs practiced.
	paceTarget     string        // -pace best or avg, see paceFromResults.

	listType  string
	layout    string
	export    string
	wordFile  string
	quoteFile string
	themeName string
	files     []string

	given map[string]bool // The flags given, rather than left to their default.

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

// parseArgs parses the command line arguments. The usage is printed on
// stdout when asked for, in which case the error is flag.ErrHelp, and invalid
// arguments are reported on stderr.
func parseArgs(args []string, stdout, stderr io.Writer) (options, error) {
	var opts options
	var timeout int
	var pace string

	fs := flag.NewFlagSet("tt", flag.ContinueOnError)
	fs.SetOutput(stderr)

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
	fs.IntVar(&opts.focus, "focus", 40, "")
	fs.BoolVar(&opts.punct, "punct", false, "")
	fs.BoolVar(&opts.numbers, "numbers", false, "")

	fs.BoolVar(&opts.showWpm, "showwpm", false, "")
	fs.BoolVar(&opts.showAccuracy, "showaccuracy", false, "")
	fs.BoolVar(&opts.noSkip, "noskip", false, "")
	fs.BoolVar(&opts.blockCursor, "blockcursor", false, "")
	fs.BoolVar(&opts.noBackspace, "nobackspace", false, "")
	fs.BoolVar(&opts.blind, "blind", false, "")
	fs.StringVar(&pace, "pace", "", "")
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
	fs.StringVar(&opts.export, "export", "", "")

	fs.Usage = func() { io.WriteString(stdout, usage) }
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	opts.given = map[string]bool{}
	fs.Visit(func(f *flag.Flag) { opts.given[f.Name] = true })

	opts.files = fs.Args()

	opts.timeout = -1
	if timeout != -1 {
		opts.timeout = time.Duration(timeout) * time.Second
	}

	if wpm, err := strconv.Atoi(pace); err == nil {
		opts.pace = wpm
	} else {
		opts.paceTarget = pace
	}

	return opts, nil
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
	case o.paceTarget != "" && o.paceTarget != paceBest && o.paceTarget != paceAverage:
		return errors.New("-pace must be a speed in wpm, best or avg")
	case o.focus < 0 || o.focus > 100:
		return errors.New("-focus must be between 0 and 100")
	}

	return nil
}
