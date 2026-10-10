package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-isatty"
	"io"
)

var scr tcell.Screen
var csvMode bool
var jsonMode bool
var themeToPersist string
var persistThemeOnExit bool

func die(format string, args ...interface{}) {
	if scr != nil {
		scr.Fini()
	}
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

var results []result

func exit(rc int) {
	if scr != nil {
		scr.Fini()
		scr = nil
	}

	if persistThemeOnExit && themeToPersist != "" {
		if err := writeAppConfig(appConfig{Theme: themeToPersist}); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not save theme selection: %v\n", err)
		}
	}

	if jsonMode {
		//Avoid null in serialized JSON.
		for i := range results {
			if results[i].Mistakes == nil {
				results[i].Mistakes = []mistake{}
			}
		}

		b, err := json.Marshal(results)
		if err != nil {
			panic(err)
		}
		os.Stdout.Write(b)
	}

	if csvMode {
		for _, r := range results {
			fmt.Printf("test,%d,%d,%.2f,%d\n", r.Wpm, r.Cpm, r.Accuracy, r.Timestamp)
			for _, m := range r.Mistakes {
				fmt.Printf("mistake,%s,%s\n", m.Word, m.Typed)
			}
		}
	}

	os.Exit(rc)
}

var usage = `usage: tt [options] [file]

Modes
    -words  WORDFILE    Specifies the file from which words are randomly
                        drawn (default: 1000en).
    -quotes QUOTEFILE   Starts quote mode in which quotes are randomly drawn
                        from the given file. The file should be JSON encoded and
                        have the following form:

                        [{"text": "foo", attribution: "bar"}]

Word Mode
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
    -stats              Print average WPM and accuracy over the last hour,
                        day and week.

Version
    -v                  Print the current version.
`

func main() {
	var n int
	var g int

	var rawMode bool
	var oneShotMode bool
	var noHighlightCurrent bool
	var noHighlightNext bool
	var noHighlight bool
	var maxLineLen int
	var noSkip bool
	var noBackspace bool
	var noReport bool
	var noTheme bool
	var disableTransparentBg bool
	var normalCursor bool
	var timeout int
	var startParagraph int

	var listFlag string
	var wordFile string
	var quoteFile string

	var themeName string
	var showWpm bool
	var showAccuracy bool
	var multiMode bool
	var statsFlag bool
	var versionFlag bool
	var boldFlag bool

	var err error
	var testFn func() []segment

	flag.IntVar(&n, "n", 50, "")
	flag.IntVar(&g, "g", 1, "")
	flag.IntVar(&startParagraph, "start", -1, "")

	flag.IntVar(&maxLineLen, "w", 80, "")
	flag.IntVar(&timeout, "t", -1, "")

	flag.BoolVar(&versionFlag, "v", false, "")

	flag.StringVar(&wordFile, "words", "", "")
	flag.StringVar(&quoteFile, "quotes", "", "")

	flag.BoolVar(&showWpm, "showwpm", false, "")
	flag.BoolVar(&showAccuracy, "showaccuracy", false, "")
	flag.BoolVar(&noSkip, "noskip", false, "")
	flag.BoolVar(&normalCursor, "blockcursor", false, "")
	flag.BoolVar(&noBackspace, "nobackspace", false, "")
	flag.BoolVar(&noTheme, "notheme", false, "")
	flag.BoolVar(&disableTransparentBg, "notransparent", false, "")
	flag.BoolVar(&oneShotMode, "oneshot", false, "")
	flag.BoolVar(&noHighlight, "nohighlight", false, "")
	flag.BoolVar(&noHighlightCurrent, "highlight2", false, "")
	flag.BoolVar(&noHighlightNext, "highlight1", false, "")
	flag.BoolVar(&noReport, "noreport", false, "")
	flag.BoolVar(&boldFlag, "bold", false, "")
	flag.BoolVar(&csvMode, "csv", false, "")
	flag.BoolVar(&jsonMode, "json", false, "")
	flag.BoolVar(&rawMode, "raw", false, "")
	flag.BoolVar(&multiMode, "multi", false, "")
	flag.BoolVar(&statsFlag, "stats", false, "")
	flag.StringVar(&themeName, "theme", "default", "")
	flag.StringVar(&listFlag, "list", "", "")

	flag.Usage = func() { os.Stdout.Write([]byte(usage)) }
	flag.Parse()

	themeFlagExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "theme" {
			themeFlagExplicit = true
		}
	})

	if listFlag != "" {
		for _, name := range listResourceNames(listFlag) {
			fmt.Println(name)
		}

		os.Exit(0)
	}

	if versionFlag {
		fmt.Println("tt version 0.4.4")
		os.Exit(0)
	}

	if statsFlag {
		printStats()
		os.Exit(0)
	}

	if noTheme {
		os.Setenv("TCELL_TRUECOLOR", "disable")
	} else if !themeFlagExplicit {
		if rememberedTheme := loadRememberedTheme(disableTransparentBg); rememberedTheme != "" {
			themeName = rememberedTheme
		}
	}

	reflow := func(s string) string {
		sw, _ := scr.Size()

		wsz := maxLineLen
		if wsz > sw {
			wsz = sw - 8
		}

		s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
		return strings.ReplaceAll(
			wordWrap(strings.Trim(s, " "), wsz),
			"\n", " \n")
	}

	switch {
	case wordFile != "":
		testFn = generateWordTest(wordFile, n, g)
	case quoteFile != "":
		testFn = generateQuoteTest(quoteFile)
	case !isatty.IsTerminal(os.Stdin.Fd()):
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		testFn = generateTestFromData(b, rawMode, multiMode)
	case len(flag.Args()) > 0:
		path := flag.Args()[0]
		testFn = generateTestFromFile(path, startParagraph)
	default:
		testFn = generateWordTest("1000en", n, g)
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

	var typer *typer
	if noTheme {
		typer = NewTyper(scr, true, terminalTheme)
	} else {
		th, err := loadTheme(themeName, disableTransparentBg)
		if err != nil {
			die("%s theme is invalid: %v", themeName, err)
		}

		typer = NewTyper(scr, boldFlag, th)
		themeToPersist = themeName
		persistThemeOnExit = true
	}

	typer.SetHighlightMode(
		!(noHighlightCurrent || noHighlight),
		!(noHighlightNext || noHighlight),
	)

	if !noTheme {
		themeNames := listResourceNames("themes")
		themeIdx := indexResourceName(themeNames, themeName)

		typer.CycleTheme = func(direction int) (string, error) {
			if len(themeNames) == 0 {
				return "", nil
			}

			for attempts := 0; attempts < len(themeNames); attempts++ {
				if themeIdx == -1 {
					if direction < 0 {
						themeIdx = len(themeNames) - 1
					} else {
						themeIdx = 0
					}
				} else {
					themeIdx = (themeIdx + direction + len(themeNames)) % len(themeNames)
				}

				nextTheme := themeNames[themeIdx]
				th, err := loadTheme(nextTheme, disableTransparentBg)
				if err != nil {
					continue
				}

				typer.SetTheme(th)
				typer.RefreshScreen()
				themeName = nextTheme
				themeToPersist = nextTheme
				return nextTheme, nil
			}

			return "", fmt.Errorf("no valid themes available")
		}
	}

	typer.SkipWord = !noSkip
	typer.DisableBackspace = noBackspace
	typer.BlockCursor = normalCursor
	typer.ShowWpm = showWpm
	typer.ShowAccuracy = showAccuracy

	if timeout != -1 {
		timeout *= 1e9
	}

	var tests [][]segment
	var idx = 0

	for {
		if idx >= len(tests) {
			tests = append(tests, testFn())
		}

		if tests[idx] == nil {
			exit(0)
		}

		if !rawMode {
			for i := range tests[idx] {
				tests[idx][i].Text = reflow(tests[idx][i].Text)
			}
		}

		res := typer.Start(tests[idx], time.Duration(timeout))

		switch res.rc {
		case TyperNext:
			idx++
		case TyperPrevious:
			if idx > 0 {
				idx--
			}
		case TyperComplete:
			cpm := res.cpm()
			wpm := res.wpm()
			accuracy := res.accuracy()
			timestamp := time.Now().Unix()

			results = append(results, result{wpm, cpm, accuracy, timestamp, res.mistakes})
			savePersistedResult(wpm, accuracy, timestamp)
			saveMistakes(res.mistakes)
			if !noReport {
				attribution := ""
				if len(tests[idx]) == 1 {
					attribution = tests[idx][0].Attribution
				}
				showReport(scr, cpm, wpm, accuracy, attribution, res.mistakes)
			}
			if oneShotMode {
				exit(0)
			}

			idx++
		case TyperSigInt:
			exit(1)

		case TyperResize:
			//Resize events restart the test, this shouldn't be a problem in the vast majority of cases
			//and allows us to avoid baking rewrapping logic into the typer.

			//TODO: implement state-preserving resize (maybe)
		}
	}
}
