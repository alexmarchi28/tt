
https://github.com/user-attachments/assets/f8582968-4dd2-4ee1-930c-8107469b3cfd

# What

A terminal based typing test, to measure and improve your typing speed.

tt was originally written by Aetnaeus ([lemnos/tt](https://github.com/lemnos/tt)).

# Installation

Binaries for Linux, macOS and Windows are attached to the [latest release](https://github.com/alexmarchi28/tt/releases/latest),
along with the man page.

## Linux

```
sudo curl -L https://github.com/alexmarchi28/tt/releases/latest/download/tt-linux-amd64 -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -L https://github.com/alexmarchi28/tt/releases/latest/download/tt.1.gz -o /usr/share/man/man1/tt.1.gz
```

On ARM machines (e.g. a Raspberry Pi) use `tt-linux-arm64` or `tt-linux-arm`.

## macOS

```
mkdir -p /usr/local/bin /usr/local/share/man/man1 # Usually created by brew

sudo curl -L https://github.com/alexmarchi28/tt/releases/latest/download/tt-darwin-arm64 -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -L https://github.com/alexmarchi28/tt/releases/latest/download/tt.1.gz -o /usr/local/share/man/man1/tt.1.gz
```

On Intel Macs use `tt-darwin-amd64`.

## Uninstall

```
sudo rm /usr/local/bin/tt /usr/share/man/man1/tt.1.gz
```

The man page is in `/usr/local/share/man/man1` on macOS.

## With Go

```
go install github.com/alexmarchi28/tt/cmd/tt@latest
```

## From source

tt needs Go 1.20 or later.

```
# debian dependencies
sudo apt install golang

# clone and make
git clone https://github.com/alexmarchi28/tt
cd tt
make && sudo make install
```

Best served on a terminal with truecolor and cursor shape support (e.g kitty, iterm)

# Usage

By default 50 words from the top 1000 words in the English language are used to
constitute the test. Custom text can be supplied by piping arbitrary text to the
program. Each paragraph in the input is shown as a separate segment of the text.
At the end of each test a report shows your speed, accuracy and mistakes, and
whether you set a new personal best for that kind of test: the same word list,
quote file or file, with the same time limit, number of words, punctuation and
numbers.
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

 - `tt -quotes en` Starts quote mode with the builtin quote list 'en' ('it' for Italian literature).
 - `tt -words 1000it` Draws the words from 1000 common Italian words, see `tt -list words` for the other languages.
 - `tt -mistakes` Practices the words you recently mistyped, the most frequent ones more often.
 - `tt -adaptive` Practices the words containing the pairs of letters you miss or type slowly.
 - `tt -n 10 -g 5` produces a test consisting of 50 randomly drawn words in 5 groups of 10 words each.
 - `tt -punct -numbers` adds capital letters, punctuation and numbers to the words.
 - `tt -t 10` starts a timed test lasting 10 seconds.
 - `tt -pace 70` shows a pacer typing at 70 wpm to keep up with.
 - `tt -t 30 -pace best` races a pacer at your personal best on 30 second tests (`-pace avg` for your recent average).
 - `tt -theme gruvbox` Starts tt with the gruvbox theme.
 - `tt -keys -layout it` Shows on an Italian keyboard which keys you miss most and type slowest.
 - `tt -export results > results.csv` Exports all your saved results, e.g. to a spreadsheet (`tt -export mistakes` for the mistakes, `-json` for JSON).
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
| `kind.go` | The kinds of tests whose results are compared |
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

Pushing a tag such as `v1.1.0` publishes the release of that version with the
binaries of every platform, the man page and their checksums (see `make rel`).
Its notes are taken from the `# 1.1.0:` section of `CHANGELOG.md`, so write it
before tagging. The release of an earlier tag can be published by running the
Release workflow by hand from the Actions tab.
