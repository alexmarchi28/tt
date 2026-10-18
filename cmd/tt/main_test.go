package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexmarchi28/tt"
	"github.com/gdamore/tcell"
)

// newTestResources returns the resources of the calling test: user resources
// in a directory of its own, whose path is returned (see writeUserResource),
// then the builtin ones. Resources named "-" are read from an empty stdin.
func newTestResources(t *testing.T) (*resources, string) {
	t.Helper()

	dir := t.TempDir()
	res := &resources{
		dirs:     []fs.FS{os.DirFS(filepath.Join(dir, ".tt")), tt.Builtin},
		stdin:    &onceReader{r: strings.NewReader("")},
		readFile: os.ReadFile,
	}

	return res, dir
}

// writeUserResource creates a user resource (theme, word list, quote file)
// inside the directory returned by newTestResources.
func writeUserResource(t *testing.T, dir, typ, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, ".tt", typ, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	return path
}

// testWait is how long tests wait for the screen or the typer before failing.
// It is generous for slow machines, waits end as soon as possible.
const testWait = 5 * time.Second

func newSimScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()

	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(sim.Fini)

	return sim
}

// screenLine returns row y of the screen as last shown, without trailing
// blanks.
func screenLine(sim tcell.SimulationScreen, y int) string {
	sim.Show()
	return strings.TrimRight(strings.Split(simulationScreenText(sim), "\n")[y], " ")
}
