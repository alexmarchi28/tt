# Unreleased:

New:
 - -focus sets the percentage of the words of -adaptive with the pairs of
   letters being practiced, 40 by default.
 - 1000 common Russian words (-words 1000ru): the most frequent pronouns,
   prepositions, verbs and adjectives besides the nouns of the ru list.

Changed:
 - -adaptive practices the weak pairs of letters costing the most time, those
   frequent in the word list, and judges the pairs typed a few times
   cautiously, rather than chasing rare pairs missed once.
 - Options which used to be silently ignored are rejected: modes which can't be
   combined (e.g. -adaptive -quotes), more than one file, -focus without
   -adaptive, several of -list, -stats, -keys, -export and -v, an unknown
   -list type and an unknown -layout, and -csv with -json.
 - A file given on the command line is typed even when stdin is not a
   terminal, rather than the text piped to stdin.

Fixed:
 - A test whose outcome can't be saved ends tt with an error, after printing
   the results of the tests typed (-csv, -json), rather than with a crash.
 - A word list, quote file or theme read from stdin (-) which can't be read is
   reported as an error rather than with a crash.
 - The paragraph reached in a file which can't be saved ends tt with an error
   rather than with a crash.
 - The Russian word list contained the English word TRUE.
 - Resizing the terminal no longer restarts the test: the text is wrapped again
   for the new width, keeping what was typed and the time.
 - -adaptive -words - (the word list read from stdin) no longer fails with an
   empty word list.
 - A file of recorded mistakes which can't be read is no longer replaced, losing
   its mistakes: tt refuses to start until it is fixed or removed, as with the
   saved results.
 - The report after a test is shown in the colours of the theme, and centered
   again when the terminal is resized.
 - Wide characters (e.g. Chinese, Japanese, Korean) take two cells: the text,
   the cursor and the wrapping of the lines no longer overlap or drift.
 - A terminal narrower than the margins no longer wraps the text at zero or
   negative widths.
 - The screen updates stop with the test, rather than blocking when events
   pile up.

# 1.1.0:

New:
 - -export prints the saved results or mistakes as CSV, or as JSON with -json.
 - -pace best and -pace avg show a pacer at your best speed or at your average
   over the last 10 tests of the same kind.
 - The uk, es, colemak-dh and workman layouts for -keys.
 - 1000 common Italian words (-words 1000it) and quotes from Italian literature
   (-quotes it).

Changed:
 - Timed word tests (-t, also with -mistakes and -adaptive) have enough words to
   last until the time is up.
 - Personal bests and the results of -stats are kept apart for each time limit,
   number of words, punctuation and numbers. Results saved by earlier versions,
   whose variant is unknown, are shown on their own.

Fixed:
 - Timed tests ended up to half a second after the time limit, counting the keys
   pressed in the meantime.

# 1.0.0:

New:
 - -mistakes practices the recently mistyped words, the most frequent ones more often.
 - -adaptive practices the words with the pairs of letters typed the slowest or missed the most.
 - -punct and -numbers add capital letters, punctuation and numbers to word tests.
 - -blind hides mistakes while typing.
 - -pace WPM shows a pacer typing at the given speed.
 - -keys (and -layout) shows on a keyboard how accurately and quickly each key is typed.
 - -stats also shows the number of tests, the time spent typing, the personal best,
   the all time average, the weekly trend, a chart of the daily progress, the results
   by kind of test and the most mistyped words.
 - The report shows new personal bests.
 - Tabs can be typed, e.g. source code with -raw.
 - Binaries for macOS on Apple Silicon, and tt can be installed with go install.

Changed:
 - -v prints the version on stdout and exits successfully.
 - -csv quotes words containing commas or quotes, -json prints [] rather than null
   when no test was completed.
 - Invalid numeric options and texts with nothing to type are rejected.
 - The release binaries are named tt-OS-ARCH.

Fixed:
 - The accuracy of tests made of several segments only counted the last one.
 - Crash when a test was completed without any counted keystroke.
 - Pressing space at the start of a line skipped its first word.
 - Mistakes at the start of a line were recorded with a line break.
 - Typing a '?' of the text toggled the help.
 - The default word list contained an empty word, a word list with a single word made
   tt hang and an empty one made it crash.
 - Lines were wrapped by bytes rather than characters, too early with accented letters.
 - The builtin quotes were out of date and the maia theme had an invalid colour.
 - A truncated results file made tt crash after every test. Saved files are now
   written atomically.
 - The mistakes of abandoned tests were recorded with -showwpm only.
 - tt 0.4.5 reported version 0.4.4.

# 0.4.5:
 - Ctrl-N and Ctrl-P cycle themes, the last used theme is remembered.
 - Results are saved and -stats prints the average WPM and accuracy.
 - -showaccuracy displays the accuracy whilst typing.
 - The background is transparent by default, -notransparent disables it.
 - The next word is highlighted correctly across line breaks.

# 0.4.4:
 - The accuracy is calculated in real time rather than at the end of the test.

# 0.4.3:
 - Added support for a transparent background.

# 0.4.2:
  Added -notheme, -blockcursor and -bold.

# 0.4.0:
  Too numerous to list (see the man page)

  Highlights:
  
 - Added -quotes.
 - Added support for navigating between tests via right/left.
 - Now store the user's position within a file if one is specified.
 - Improved documentation.

# 0.3.0:
 - Added support for custom word lists (`-words).
 - `-theme` now accepts a path.
 - Added `~/.tt/themes` and `~/.tt/words`.
 - Scrapped ~/.ttrc in favour of aliases/flags.
 - Included more default word lists. (`-list words`)

# 0.2.2:
 - Modified -g to correspond to the number of groups rather than the group size.
 - Added -multi
 - Added -v
 - Changed the default behaviour to restart the currently generated test rather than generating a new one
 - Added a CHANGELOG :P
