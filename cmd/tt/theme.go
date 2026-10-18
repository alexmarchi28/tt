package main

import (
	"bytes"
	"fmt"
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

// loadRememberedTheme returns the theme selected last, if it is still valid.
func loadRememberedTheme(st configStore, disableTransparentBg bool) string {
	cfg, err := st.Config()
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

// themeCycler steps through the available themes, skipping invalid ones.
type themeCycler struct {
	names    []string
	idx      int // Index of the current theme in names, -1 if it is not one of them.
	opaqueBg bool
}

func newThemeCycler(names []string, current string, opaqueBg bool) *themeCycler {
	return &themeCycler{names, indexResourceName(names, current), opaqueBg}
}

// next loads the next (direction > 0) or previous valid theme. The name is
// empty if there are no themes at all.
func (c *themeCycler) next(direction int) (string, theme, error) {
	if len(c.names) == 0 {
		return "", theme{}, nil
	}

	for attempts := 0; attempts < len(c.names); attempts++ {
		if c.idx == -1 {
			if direction < 0 {
				c.idx = len(c.names) - 1
			} else {
				c.idx = 0
			}
		} else {
			c.idx = (c.idx + direction + len(c.names)) % len(c.names)
		}

		name := c.names[c.idx]
		if th, err := loadTheme(name, c.opaqueBg); err == nil {
			return name, th, nil
		}
	}

	return "", theme{}, fmt.Errorf("no valid themes available")
}
