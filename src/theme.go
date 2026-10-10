package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell"
)

// theme holds the colours used to draw a test, see themes/ for the format
// of theme files.
type theme struct {
	fgcol  tcell.Color // Untyped text.
	bgcol  tcell.Color // Background.
	hicol  tcell.Color // Correctly typed text.
	hicol2 tcell.Color // Current word.
	hicol3 tcell.Color // Next word.
	errcol tcell.Color // Mistakes.
}

// terminalTheme is used with -notheme and relies on the terminal colours.
var terminalTheme = theme{
	fgcol:  tcell.ColorDefault,
	bgcol:  tcell.ColorDefault,
	hicol:  tcell.ColorWhite,
	hicol2: tcell.ColorGreen,
	hicol3: tcell.ColorGreen,
	errcol: tcell.ColorMaroon,
}

// loadTheme reads the named theme. Unless opaqueBg is set the background is
// left transparent and bgcol is not required.
func loadTheme(name string, opaqueBg bool) (theme, error) {
	b := readResource("themes", name)
	if b == nil {
		return theme{}, fmt.Errorf("%s does not appear to be a valid theme", name)
	}

	cfg := parseConfig(b)
	th := theme{bgcol: tcell.ColorDefault}

	type field struct {
		key string
		col *tcell.Color
	}

	fields := []field{
		{"fgcol", &th.fgcol},
		{"hicol", &th.hicol},
		{"hicol2", &th.hicol2},
		{"hicol3", &th.hicol3},
		{"errcol", &th.errcol},
	}
	if opaqueBg {
		fields = append([]field{{"bgcol", &th.bgcol}}, fields...)
	}

	for _, f := range fields {
		col, err := newTcellColor(cfg[f.key])
		if err != nil {
			return theme{}, fmt.Errorf("%s is not defined and/or a valid hex colour", f.key)
		}
		*f.col = col
	}

	return th, nil
}

func parseConfig(b []byte) map[string]string {
	if b == nil {
		return nil
	}

	cfg := map[string]string{}
	for _, ln := range bytes.Split(b, []byte("\n")) {
		a := strings.SplitN(string(ln), ":", 2)
		if len(a) == 2 {
			cfg[a[0]] = strings.Trim(a[1], " ")
		}
	}

	return cfg
}

func newTcellColor(s string) (tcell.Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return 0, fmt.Errorf("%s is not a valid hex color", s)
	}

	rgb, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid hex color", s)
	}

	return tcell.NewHexColor(int32(rgb)), nil
}

func loadRememberedTheme(disableTransparentBg bool) string {
	cfg, err := readAppConfig()
	if err != nil {
		return ""
	}

	themeName := strings.TrimSpace(cfg.Theme)
	if themeName == "" {
		return ""
	}

	if _, err := loadTheme(themeName, disableTransparentBg); err != nil {
		return ""
	}

	return themeName
}

func indexResourceName(names []string, name string) int {
	for i, candidate := range names {
		if candidate == name {
			return i
		}
	}

	base := filepath.Base(name)
	if base != name {
		for i, candidate := range names {
			if candidate == base {
				return i
			}
		}
	}

	return -1
}
