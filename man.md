% tt(1)

# NAME

tt - A terminal based typing test

# SYNOPSIS

usage: tt \[OPTION\]... \[FILE\]

# DESCRIPTION

  By default tt creates a test consisting of 50 randomly generated words from
  the top 1000 words in the English language. If provided with a path, tt will
  use the given file as input treating each paragraph as a separate segment of
  the test. The program will automatically keep track of your position in the
  file so subsequent invocations on the same path will place you at the most
  recent paragraph (-start 0 can be used to reset your position).  
  
  Arbitrary text can also be piped directly into the program to create a custom
  test. Each paragraph of the input is treated as a segment unless '-multi' is
  supplied in which case each paragraph is treated as a separate test. 

  At the end of each test a report shows the speed, accuracy and mistakes, and
  whether a new personal best was set for that kind of test: the same word
  list, quote file or file, with the same time limit, number of words,
  punctuation and numbers.

# OPTIONS

## Modes

-words  *WORDFILE*

: Specifies the file from which words are randomly drawn (default: 1000en).

-quotes *QUOTEFILE*

: Starts quote mode in which quotes are randomly drawn from the given file. The file should be JSON encoded and have the following form:

    [{"text": "foo", attribution: "bar"}]

-mistakes

: Practice the words mistyped in the last 500 mistakes, the most frequently
mistyped ones coming up more often. -n and -g apply.

-adaptive

: Practice the words of the word list (see -words) containing the pairs of
letters costing you the most time: those you miss the most or type the
slowest compared to the others, as recorded during the previous tests, and
that are frequent in the word list. Pairs typed only a few times are judged
cautiously. The 3 pairs being practiced are shown at the top of the screen.
-n and -g apply.

-focus *PERCENT*

: The percentage of the words of -adaptive drawn among those containing the
pairs being practiced (default: 40). The other words are drawn among all the
words of the list, favouring those with weak pairs, so that the pairs
practiced make up a bit more. 30 to 50 suits regular practice, 70 to 100 short
drills, 0 only favours the words with weak pairs.

## Word Mode (also applies to -mistakes and -adaptive)

-n *GROUPSZ*

: Sets the number of words which constitute a group.

-g *NGROUPS*

: Sets the number of groups which constitute a test.

-punct

: Turn the words into sentences with capital letters and punctuation marks.

-numbers

: Replace some of the words with numbers.

## File Mode
-start *PARAGRAPH*

: The offset of the starting paragraph, set this to 0 to reset progress on a given file.

## Aesthetics

-showwpm

: Display WPM whilst typing.

-showaccuracy

: Display accuracy whilst typing.

-theme *THEMEFILE*

: The theme to use. 
  If omitted, tt reuses the last selected theme saved in
  ~/.config/tt/config.json when available.

-notheme

: Attempt to use the default terminal theme. This may produce odd results depending on the theme colours.

-notransparent

: Disable transparent background.

-blockcursor

: Use the default cursor style.

-bold

: Embolden typed text.

-w 

: The maximum line length in characters. This option is ignored if -raw is present.

## Test Parameters

-t *SECONDS*

: Terminate the test after the given number of seconds. Word tests (including
-mistakes and -adaptive) have enough words to last until then, -g only sets
the least number of groups.

-noskip

: Disable word skipping when space is pressed.

-nobackspace

: Disable the backspace key.

-blind

: Don't show mistakes while typing, they are still reported at the end of the
test.

-pace *WPM*|best|avg

: Show a pacer going through the text at the given speed, starting with the
first key press. best sets the speed to your best result on the same kind of
test (see DESCRIPTION), avg to your average over its last 10 results.

-nohighlight

: Disable highlighting.

-highlight1

: Only highlight the current word.

-highlight2

: Only highlight the next word.

## Scripting

-oneshot

: Automatically exit after a single run.

-noreport

: Don't show a report at the end of a test.

-csv

: Print CSV formatted results.

	Tests have the form:

	```
	test,[wpm],[cpm],[accuracy],[timestamp].
	```

	Mistakes have the form:

	```
	mistake,[word],[typed]
	```

-json

: Print the test output in JSON. See also -export.

-raw

: Don't reflow STDIN text or show one paragraph at a time.  Note that line breaks
are determined exclusively by the input. Tabs, e.g. the indentation of source
code, are typed with the tab key.

-multi 

: Treat each input paragraph as a self contained test.

## Misc

**-list** *TYPE*\

    Lists internal resources of the given type. TYPE=[themes|quotes|words].

**-stats**\

    Print statistics about the saved results: number of tests, time spent
    typing, personal best, average WPM and accuracy over the last hour, day,
    week and all time, trend compared to the previous week, a chart of the
    average WPM of each of the last 30 days, results by kind of test (word list,
    quotes, file..., time limit, number of words, punctuation and numbers) and
    most mistyped words.

**-keys**\

    Print how accurately and quickly each key is typed on a keyboard, from the
    best key to the worst one compared to the others, as well as the most
    missed and slowest keys. Keys need a few keystrokes before being rated.

**-layout** *LAYOUT*\

    The keyboard layout used by -keys: us, uk, it, es, de, fr, dvorak, colemak,
    colemak-dh or workman (default: us).

**-export** *DATA*\

    Print the saved results (DATA=results) or the recorded mistakes
    (DATA=mistakes) as CSV, or as JSON with -json. Results are made of the
    timestamp, the WPM, the accuracy, the duration in seconds, the mode (words,
    quotes, mistakes, adaptive, file or text), its source (word list, quote
    file or file), the time limit in seconds, the number of words of untimed
    word tests and whether punctuation and numbers were added. Mistakes are made
    of the word and what was typed instead.

**-v**\

    Print the current version.

# EXAMPLES

Creates a series of tests each consisting of a random quote drawn from the
builtin quote file 'en'.
```
tt -quotes en
```

Creates a series of tests each consisting of 10 random words drawn from
words.txt
```
tt -words words.txt -n 10
```

Prints statistics about the saved results.
```
tt -stats
```

Starts a sequence of tests in which each test consists of a paragraph from war
and peace starting with paragraph 1.
```
tt ~/war_and_peace.txt -start 1
```

Produces a test consisting of 40 random words draw from 
the system dictionary (similar to 'tt -n 40').
```
shuf -n 40 /usr/share/dict/words|tt
```

Starts a test consisting of two randomly drawn quotes from api.quotable.io and
prints the output of each test to STDOUT in csv format.
```
curl https://api.quotable.io/quotes|\
    jq '[.results[]|.text=.content|.attribution=.author][:2]'|\
    tt -quotes - -noreport -csv
```

Starts a new typing test which uses the tt source as input:

```
curl -LsS https://raw.githubusercontent.com/alexmarchi28/tt/master/cmd/tt/tt.go | head -n 20 | tt -noskip -raw
```

Modify to taste.

# PATHS

  Some options like **-words** and **-theme** accept a path. If the given path does
  not exist, the following directories are searched for a file with the given
  name before falling back to internal resources:

  ~/.tt/words\
  ~/.tt/themes\
  /etc/tt/words\
  /etc/tt/themes

  The last selected theme is stored in:

  ~/.config/tt/config.json

  Results, mistakes and statistics about each key are stored in:

  ~/.local/share/tt (or $XDG_DATA_HOME/tt)

# KEYS

  **?: ** Shows or hides the keyboard shortcuts help\
  **esc: ** Restarts the test\
  **C-c: ** Terminates tt\
  **C-l: ** Redraws the terminal\
  **C-backspace: ** Deletes the previous word when backspace is enabled\
  **Alt-backspace: ** Deletes the previous word when backspace is enabled\
  **C-w: ** Deletes the previous word when backspace is enabled\
  **right** Move to the next test.\
  **left** Move to the previous test.\
  **C-n: ** Switch to the next theme and briefly show its name\
  **C-p: ** Switch to the previous theme and briefly show its name.

# AUTHOR

Aetnaeus (aetnaeus@protonmail.com)

# SEE ALSO

## Project Page

    https://github.com/alexmarchi28/tt

## Original Project

    https://github.com/lemnos/tt

# LICENSE

MIT
