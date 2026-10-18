package main

import (
	"flag"
	"fmt"
	"strings"
)

// flagValues are the values of the flags, before parseArgs converts the
// ones which aren't options as is.
type flagValues struct {
	options
	timeout int    // -t, in seconds.
	pace    string // -pace, a speed or a target.
}

// flagSpec is a flag of the command line: how the usage shows it and where
// its value goes.
type flagSpec struct {
	syntax string // The flag and its argument, e.g. "-words  WORDFILE".
	help   string // Its lines are aligned in the usage, empty ones kept.
	define func(fs *flag.FlagSet, v *flagValues)
}

// flagSection is a group of flags of the usage, followed by a note.
type flagSection struct {
	title string
	flags []flagSpec
	note  string
}

// flagName returns the name of the flag of syntax, without its dash.
func flagName(syntax string) string {
	return strings.TrimPrefix(strings.Fields(syntax)[0], "-")
}

func boolFlag(syntax, help string, value func(v *flagValues) *bool) flagSpec {
	return flagSpec{syntax, help, func(fs *flag.FlagSet, v *flagValues) {
		fs.BoolVar(value(v), flagName(syntax), false, "")
	}}
}

func intFlag(syntax string, def int, help string, value func(v *flagValues) *int) flagSpec {
	return flagSpec{syntax, help, func(fs *flag.FlagSet, v *flagValues) {
		fs.IntVar(value(v), flagName(syntax), def, "")
	}}
}

func stringFlag(syntax, def, help string, value func(v *flagValues) *string) flagSpec {
	return flagSpec{syntax, help, func(fs *flag.FlagSet, v *flagValues) {
		fs.StringVar(value(v), flagName(syntax), def, "")
	}}
}

// defineFlags defines the flags of every section on fs, storing their values
// in v.
func defineFlags(fs *flag.FlagSet, v *flagValues) {
	for _, section := range flagSections {
		for _, f := range section.flags {
			f.define(fs, v)
		}
	}
}

// usage returns the usage of tt, generated from flagSections.
func usage() string {
	var b strings.Builder
	b.WriteString("usage: tt [options] [file]\n")

	for _, section := range flagSections {
		fmt.Fprintf(&b, "\n%s\n", section.title)
		for _, f := range section.flags {
			for i, line := range strings.Split(f.help, "\n") {
				switch {
				case i == 0:
					fmt.Fprintf(&b, "    %-20s%s\n", f.syntax, line)
				case line == "":
					b.WriteString("\n")
				default:
					fmt.Fprintf(&b, "%24s%s\n", "", line)
				}
			}
		}

		if section.note != "" {
			b.WriteString("\n")
			for _, line := range strings.Split(section.note, "\n") {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
	}

	return b.String()
}

// flagSections are the flags of tt, in the order of the usage.
var flagSections = []flagSection{
	{
		title: "Modes",
		flags: []flagSpec{
			stringFlag("-words  WORDFILE", "", "Specifies the file from which words are randomly\n"+
				"drawn (default: 1000en).",
				func(v *flagValues) *string { return &v.wordFile }),
			stringFlag("-quotes QUOTEFILE", "", "Starts quote mode in which quotes are randomly drawn\n"+
				"from the given file. The file should be JSON encoded and\n"+
				"have the following form:\n"+
				"\n"+
				"[{\"text\": \"foo\", attribution: \"bar\"}]\n",
				func(v *flagValues) *string { return &v.quoteFile }),
			boolFlag("-mistakes", "Practice the recently mistyped words, the most\n"+
				"frequently mistyped ones coming up more often.",
				func(v *flagValues) *bool { return &v.mistakes }),
			boolFlag("-adaptive", "Practice the words of the word list (see -words) with\n"+
				"the pairs of letters you type the slowest or miss the\n"+
				"most.",
				func(v *flagValues) *bool { return &v.adaptive }),
			intFlag("-focus PERCENT", 40, "The percentage of the words of -adaptive drawn among\n"+
				"those with the pairs of letters being practiced\n"+
				"(default: 40).",
				func(v *flagValues) *int { return &v.focus }),
		},
		note: "Only one mode can be used at a time. Text piped to stdin is typed unless\n" +
			"a mode or a file is given.",
	},
	{
		title: "Word Mode (also applies to -mistakes and -adaptive)",
		flags: []flagSpec{
			intFlag("-n GROUPSZ", 50, "Sets the number of words which constitute a group.",
				func(v *flagValues) *int { return &v.n }),
			intFlag("-g NGROUPS", 1, "Sets the number of groups which constitute a test.",
				func(v *flagValues) *int { return &v.g }),
			boolFlag("-punct", "Turn the words into sentences with capital letters\n"+
				"and punctuation.",
				func(v *flagValues) *bool { return &v.punct }),
			boolFlag("-numbers", "Replace some of the words with numbers.",
				func(v *flagValues) *bool { return &v.numbers }),
		},
	},
	{
		title: "File Mode",
		flags: []flagSpec{
			intFlag("-start PARAGRAPH", -1, "The offset of the starting paragraph, set this to 0 to\n"+
				"reset progress on a given file.",
				func(v *flagValues) *int { return &v.startParagraph }),
		},
	},
	{
		title: "Aesthetics",
		flags: []flagSpec{
			boolFlag("-showwpm", "Display WPM whilst typing.",
				func(v *flagValues) *bool { return &v.showWpm }),
			boolFlag("-showaccuracy", "Display accuracy whilst typing.",
				func(v *flagValues) *bool { return &v.showAccuracy }),
			stringFlag("-theme THEMEFILE", "default", "The theme to use.",
				func(v *flagValues) *string { return &v.themeName }),
			intFlag("-w", 80, "The maximum line length in characters. This option is\n"+
				"ignored if -raw is present.",
				func(v *flagValues) *int { return &v.maxLineLen }),
			boolFlag("-notheme", "Attempt to use the default terminal theme.\n"+
				"This may produce odd results depending\n"+
				"on the theme colours.",
				func(v *flagValues) *bool { return &v.noTheme }),
			boolFlag("-notransparent", "Disable transparent background.",
				func(v *flagValues) *bool { return &v.opaqueBg }),
			boolFlag("-blockcursor", "Use the default cursor style.",
				func(v *flagValues) *bool { return &v.blockCursor }),
			boolFlag("-bold", "Embolden typed text.",
				func(v *flagValues) *bool { return &v.bold }),
		},
	},
	{
		title: "Test Parameters",
		flags: []flagSpec{
			intFlag("-t SECONDS", -1, "Terminate the test after the given number of seconds.\n"+
				"Word tests have enough words to last until then.",
				func(v *flagValues) *int { return &v.timeout }),
			boolFlag("-noskip", "Disable word skipping when space is pressed.",
				func(v *flagValues) *bool { return &v.noSkip }),
			boolFlag("-nobackspace", "Disable the backspace key.",
				func(v *flagValues) *bool { return &v.noBackspace }),
			boolFlag("-blind", "Don't show mistakes while typing, they are still\n"+
				"reported at the end of the test.",
				func(v *flagValues) *bool { return &v.blind }),
			stringFlag("-pace WPM|best|avg", "", "Show a pacer going through the text at the given speed,\n"+
				"or at your best or recent average speed on the same\n"+
				"kind of test.",
				func(v *flagValues) *string { return &v.pace }),
			boolFlag("-nohighlight", "Disable current and next word highlighting.",
				func(v *flagValues) *bool { return &v.noHighlight }),
			boolFlag("-highlight1", "Only highlight the current word.",
				func(v *flagValues) *bool { return &v.noHighlightNext }),
			boolFlag("-highlight2", "Only highlight the next word.",
				func(v *flagValues) *bool { return &v.noHighlightCurrent }),
		},
	},
	{
		title: "Scripting",
		flags: []flagSpec{
			boolFlag("-oneshot", "Automatically exit after a single run.",
				func(v *flagValues) *bool { return &v.oneShot }),
			boolFlag("-noreport", "Don't show a report at the end of a test.",
				func(v *flagValues) *bool { return &v.noReport }),
			boolFlag("-csv", "Print the test results to stdout in the form:\n"+
				"[type],[wpm],[cpm],[accuracy],[timestamp].",
				func(v *flagValues) *bool { return &v.csv }),
			boolFlag("-json", "Print the test output in JSON, see also -export.",
				func(v *flagValues) *bool { return &v.json }),
			boolFlag("-raw", "Don't reflow STDIN text or show one paragraph at a time.\n"+
				"Note that line breaks are determined exclusively by the\n"+
				"input.",
				func(v *flagValues) *bool { return &v.raw }),
			boolFlag("-multi", "Treat each input paragraph as a self contained test.",
				func(v *flagValues) *bool { return &v.multi }),
		},
	},
	{
		title: "Misc",
		flags: []flagSpec{
			stringFlag("-list TYPE", "", "Lists internal resources of the given type.\n"+
				"TYPE=[themes|quotes|words]",
				func(v *flagValues) *string { return &v.listType }),
			boolFlag("-stats", "Print statistics about the saved results: number of\n"+
				"tests, time spent typing, personal best, averages over\n"+
				"time, weekly trend, a chart of the daily progress,\n"+
				"results by kind of test and most mistyped words.",
				func(v *flagValues) *bool { return &v.stats }),
			boolFlag("-keys", "Print how accurately and quickly each key is typed on\n"+
				"a keyboard.",
				func(v *flagValues) *bool { return &v.keys }),
			stringFlag("-layout LAYOUT", "us", "The keyboard layout used by -keys: us, uk, it, es, de,\n"+
				"fr, dvorak, colemak, colemak-dh or workman\n"+
				"(default: us).",
				func(v *flagValues) *string { return &v.layout }),
			stringFlag("-export DATA", "", "Print the saved results or mistakes (DATA=results or\n"+
				"mistakes) as CSV, or as JSON with -json.",
				func(v *flagValues) *string { return &v.export }),
		},
	},
	{
		title: "Version",
		flags: []flagSpec{
			boolFlag("-v", "Print the current version.",
				func(v *flagValues) *bool { return &v.version }),
		},
	},
}
