
https://github.com/user-attachments/assets/f8582968-4dd2-4ee1-930c-8107469b3cfd

# What

A terminal based typing test.

# Installation

## Linux

```
sudo curl -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt-linux -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -o /usr/share/man/man1/tt.1.gz -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt.1.gz
```

## OSX

```
mkdir -p /usr/local/bin /usr/local/share/man/man1 # Usually created by brew

sudo curl -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt-osx -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -o /usr/local/share/man/man1/tt.1.gz -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt.1.gz
```

## Uninstall

```
sudo rm /usr/local/bin/tt /usr/share/man/man1/tt.1.gz
```

## From source

```
# debian dependencies
sudo apt install golang

# clone and make
git clone https://github.com/lemnos/tt
cd tt
make && sudo make install
```

Best served on a terminal with truecolor and cursor shape support (e.g kitty, iterm)

# Usage

By default 50 words from the top 1000 words in the English language are used to
constitute the test. Custom text can be supplied by piping arbitrary text to the
program. Each paragraph in the input is shown as a separate segment of the text.
At the end of each test a report shows your speed, accuracy and mistakes, and
whether you set a new personal best for that kind of test.
See `man tt` or `man.md` for a complete description and a comprehensive set of
options.

## Keys

- `?` shows or hides the keyboard shortcuts help.
- Pressing `escape` at any point restarts the test.
- `C-c` exits the test.
- `C-l` redraws the terminal.
- `C-backspace`, `Alt-backspace`, or `C-w` delete the previous word when backspace is enabled.
- `right` moves to the next test.
- `left` moves to the previous test.
- `C-n` switches to the next theme and briefly shows its name.
- `C-p` switches to the previous theme and briefly shows its name.

## Examples

 - `tt -quotes en` Starts quote mode with the builtin quote list 'en'.
 - `tt -mistakes` Practices the words you recently mistyped, the most frequent ones more often.
 - `tt -adaptive` Practices the words containing the pairs of letters you miss or type slowly.
 - `tt -n 10 -g 5` produces a test consisting of 50 randomly drawn words in 5 groups of 10 words each.
 - `tt -punct -numbers` adds capital letters, punctuation and numbers to the words.
 - `tt -t 10` starts a timed test lasting 10 seconds.
 - `tt -pace 70` shows a pacer typing at 70 wpm to keep up with.
 - `tt -theme gruvbox` Starts tt with the gruvbox theme.
 - `tt -keys -layout it` Shows on an Italian keyboard which keys you miss most and type slowest.
 - `tt -stats` Prints your personal best, average WPM and accuracy over time, weekly trend, a chart of your daily progress, results by kind of test and most mistyped words.

`tt` is designed to be easily scriptable and integrate nicely with
other *nix tools. With a little shell scripting most features the user can
conceive of should be possible to implement. Below are some simple examples of
what can be achieved.

 - `shuf -n 40 /usr/share/dict/words|tt`  Produces a test consisting of 40 random words drawn from your system's dictionary.
 - `curl http://api.quotable.io/random|jq '[.text=.content|.attribution=.author]'|tt -quotes -` Produces a test consisting of a random quote.
 - `alias ttd='tt -csv >> ~/wpm.csv'` Creates an alias called ttd which keeps a log of progress in your home directory`.

The default behaviour is equivalent to `tt -n 50`.

See `-help` for an exhaustive list of options.

## Configuration

Custom themes and word lists can be defined in `~/.tt/themes` and `~/.tt/words`
and used in conjunction with the `-theme` and `-words` flags. A list of
preloaded themes and word lists can be found in `words/` and `themes/` and are
accessible by default using the respective flags.

When no `-theme` flag is supplied, tt reuses the last selected theme saved in
`~/.config/tt/config.json`.

Completed tests are saved automatically, and `tt -stats` reads the saved
history to show your progress.

# Development

The program lives in `cmd/tt`, while `embed.go` at the root embeds the builtin
themes, word lists and quotes into the binary.

| File | Contents |
|---|---|
| `tt.go` | `main` |
| `options.go` | Command line options and usage |
| `app.go` | The sequence of tests: navigation, reports, saving the results |
| `typer.go` | A test on the screen: styles, themes, drawing |
| `session.go` | Typing a segment of a test: keys, timer, redrawing |
| `typing.go` | The typing logic, independent of the screen |
| `generators.go` | The texts of the tests: words, quotes, files, stdin, mistakes |
| `adaptive.go` | The choice of words of `-adaptive` |
| `keystats.go` | Statistics about each key and pair of keys |
| `db.go` | The saved data: results, mistakes, key statistics, settings |
| `stats.go`, `chart.go` | `-stats` |
| `heatmap.go` | `-keys` |
| `report.go` | The report and the `-json`/`-csv` output |
| `theme.go`, `resources.go` | Themes and builtin or user resources |
| `util.go` | Text wrapping and drawing helpers |

Run the tests with `go test ./...`. They never touch your own data: saved data
goes to temporary directories (see `main_test.go`), command line tests run tt in
a separate process with a temporary home, and screen tests use the simulation
screen of tcell with a clock they control.
