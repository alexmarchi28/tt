package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/stats"
	"github.com/alexmarchi28/tt/internal/textgen"
)

// settings is the configuration of tt resolved once from the command line
// options: what to run and, for typing tests, a group of settings for each
// component they concern.
type settings struct {
	command *command // nil for typing tests.

	list   string // Type of resources listed by -list.
	layout string // Keyboard layout of -keys.
	export string // Data printed by -export.

	source  sourceSettings
	display displaySettings
	rules   ruleSettings
	output  outputSettings
}

// sourceSettings say what the text of the tests is made of.
type sourceSettings struct {
	kind           history.Kind // Mode, source and variant of the tests.
	path           string       // The word list, quote file or file typed, as given.
	words          textgen.WordOptions
	focus          int // Percentage of the words of adaptive tests with the pairs practiced.
	startParagraph int // Paragraph of the file to start from, -1 for the one reached last.
	raw            bool
	multi          bool
}

// displaySettings say how the tests are shown.
type displaySettings struct {
	theme            string
	themeExplicit    bool // Whether -theme was given, rather than remembered.
	noTheme          bool
	opaqueBg         bool
	bold             bool
	highlightCurrent bool
	highlightNext    bool
	blockCursor      bool
	showWpm          bool
	showAccuracy     bool
	maxLineLen       int
	reflow           bool // Whether the text is wrapped to the width of the screen.
}

// ruleSettings say how the tests are typed.
type ruleSettings struct {
	timeout     time.Duration // -1 for no time limit.
	noSkip      bool
	noBackspace bool
	blind       bool
	pace        int    // 0 for no pacer.
	paceTarget  string // -pace best or avg, see paceFromResults.
}

// outputSettings say what is shown and printed once the tests are done.
type outputSettings struct {
	noReport bool
	oneShot  bool
	format   string // Format of the results printed on exit, empty for none.
}

// resourceTypes are the types of resources, see -list.
var resourceTypes = []string{"quotes", "themes", "words"}

// resolve checks the command line options and resolves them into settings.
// Text piped to stdin is typed unless another mode is requested.
func resolve(o options, stdinIsTerminal bool) (settings, error) {
	if err := o.validate(); err != nil {
		return settings{}, err
	}

	cmd, err := requestedCommand(o)
	if err != nil {
		return settings{}, err
	}

	if o.listType != "" && !contains(resourceTypes, o.listType) {
		return settings{}, fmt.Errorf("unknown resource type %s, use one of: %s", o.listType, strings.Join(resourceTypes, ", "))
	}
	if _, err := stats.KeyboardLayout(o.layout); err != nil {
		return settings{}, err
	}
	if o.csv && o.json {
		return settings{}, notTogether([]string{"-csv", "-json"})
	}

	source, err := resolveSource(o, stdinIsTerminal)
	if err != nil {
		return settings{}, err
	}

	return settings{
		command: cmd,
		list:    o.listType,
		layout:  o.layout,
		export:  o.export,
		source:  source,
		display: displaySettings{
			theme:            o.themeName,
			themeExplicit:    o.given["theme"],
			noTheme:          o.noTheme,
			opaqueBg:         o.opaqueBg,
			bold:             o.bold,
			highlightCurrent: !(o.noHighlightCurrent || o.noHighlight),
			highlightNext:    !(o.noHighlightNext || o.noHighlight),
			blockCursor:      o.blockCursor,
			showWpm:          o.showWpm,
			showAccuracy:     o.showAccuracy,
			maxLineLen:       o.maxLineLen,
			reflow:           !o.raw,
		},
		rules: ruleSettings{
			timeout:     o.timeout,
			noSkip:      o.noSkip,
			noBackspace: o.noBackspace,
			blind:       o.blind,
			pace:        o.pace,
			paceTarget:  o.paceTarget,
		},
		output: outputSettings{noReport: o.noReport, oneShot: o.oneShot, format: outputFormat(o)},
	}, nil
}

// requestedCommand returns the command requested, nil for typing tests.
func requestedCommand(o options) (*command, error) {
	var requested []string
	var cmd *command
	for i := range commands {
		if commands[i].given(o) {
			requested = append(requested, "-"+commands[i].flag)
			cmd = &commands[i]
		}
	}

	if len(requested) > 1 {
		return nil, notTogether(requested)
	}

	return cmd, nil
}

// resolveSource resolves the mode of the tests and what their text is made
// of.
func resolveSource(o options, stdinIsTerminal bool) (sourceSettings, error) {
	if len(o.files) > 1 {
		return sourceSettings{}, errors.New("only one FILE can be typed at a time")
	}
	if o.given["focus"] && !o.adaptive {
		return sourceSettings{}, errors.New("-focus only applies to -adaptive")
	}

	//The modes requested, -words being the word list of -adaptive.
	var modes []string
	if o.adaptive {
		modes = append(modes, "-adaptive")
	} else if o.wordFile != "" {
		modes = append(modes, "-words")
	}
	if o.quoteFile != "" {
		modes = append(modes, "-quotes")
	}
	if o.mistakes {
		modes = append(modes, "-mistakes")
	}
	if len(o.files) > 0 {
		modes = append(modes, "FILE")
	}
	if len(modes) > 1 {
		return sourceSettings{}, notTogether(modes)
	}

	s := sourceSettings{
		words:          newWordOptions(o),
		focus:          o.focus,
		startParagraph: o.startParagraph,
		raw:            o.raw,
		multi:          o.multi,
	}

	switch {
	case o.adaptive:
		s.kind.Mode, s.path = history.ModeAdaptive, adaptiveWordList(o)
	case o.wordFile != "":
		s.kind.Mode, s.path = history.ModeWords, o.wordFile
	case o.quoteFile != "":
		s.kind.Mode, s.path = history.ModeQuotes, o.quoteFile
	case o.mistakes:
		s.kind.Mode = history.ModeMistakes
	case len(o.files) > 0:
		s.kind.Mode, s.path = history.ModeFile, o.files[0]
	case !stdinIsTerminal:
		s.kind.Mode = history.ModeText
	default:
		s.kind.Mode, s.path = history.ModeWords, defaultWordList
	}

	if s.path != "" {
		s.kind.Source = filepath.Base(s.path)
	}
	s.kind = withVariant(s.kind, o)

	return s, nil
}

// notTogether returns the error of options which can't be used together.
func notTogether(names []string) error {
	return fmt.Errorf("%s can't be used together", strings.Join(names, " and "))
}

// outputFormat returns the format requested for the results printed on exit,
// empty for none.
func outputFormat(o options) string {
	switch {
	case o.csv:
		return "csv"
	case o.json:
		return "json"
	}

	return ""
}

// adaptiveWordList returns the word list of adaptive tests.
func adaptiveWordList(opts options) string {
	if opts.wordFile != "" {
		return opts.wordFile
	}

	return defaultWordList
}

// newWordOptions returns the options of word tests given on the command line.
// Timed tests get enough words to last until the time is up.
func newWordOptions(opts options) textgen.WordOptions {
	o := textgen.WordOptions{N: opts.n, G: opts.g, Punctuation: opts.punct, Numbers: opts.numbers}
	if opts.timeout != -1 {
		o.MinChars = int(opts.timeout.Minutes() * fastestCpm)
	}

	return o
}

// withVariant returns k completed with how the tests are made according to
// the command line options.
func withVariant(k history.Kind, opts options) history.Kind {
	if opts.timeout != -1 {
		k.Seconds = int(opts.timeout / time.Second)
	}

	if k.MadeOfWords() {
		k.Punct, k.Numbers = opts.punct, opts.numbers
		if opts.timeout == -1 {
			k.Words = opts.n * opts.g
		}
	}

	if k.Mode == history.ModeAdaptive {
		k.Focus = opts.focus
	}

	return k
}

// defaultWordList is the word list of word tests unless -words is given.
const defaultWordList = "1000en"

// fastestCpm is a speed beyond the reach of any typist (500 wpm), at which
// timed word tests still don't run out of words.
const fastestCpm = 2500
