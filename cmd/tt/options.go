package main

import (
	"errors"
	"flag"
	"io"
	"strconv"
	"time"
)

// options holds the command line options, see flagSections.
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
	var v flagValues

	fs := flag.NewFlagSet("tt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	defineFlags(fs, &v)

	fs.Usage = func() { _, _ = io.WriteString(stdout, usage()) }
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	opts := v.options
	opts.given = map[string]bool{}
	fs.Visit(func(f *flag.Flag) { opts.given[f.Name] = true })

	opts.files = fs.Args()

	opts.timeout = -1
	if v.timeout != -1 {
		opts.timeout = time.Duration(v.timeout) * time.Second
	}

	if wpm, err := strconv.Atoi(v.pace); err == nil {
		opts.pace = wpm
	} else {
		opts.paceTarget = v.pace
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
