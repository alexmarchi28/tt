package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// TestMain re-executes main() when asked to by runCLI, and otherwise points
// the user resources at a throwaway directory so that tests never read the
// real ones.
func TestMain(m *testing.M) {
	if os.Getenv("TT_TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}

	dir, err := os.MkdirTemp("", "tt-test")
	if err != nil {
		panic(err)
	}

	setStorageDir(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func setStorageDir(dir string) {
	configDirs = []string{filepath.Join(dir, ".tt")}
}

// useTempStorage gives the calling test its own empty directory of user
// resources and returns its path. User resources can be placed in
// <dir>/.tt/<type>/<name>.
func useTempStorage(t *testing.T) string {
	t.Helper()

	oldConfigDirs := configDirs
	t.Cleanup(func() { configDirs = oldConfigDirs })

	dir := t.TempDir()
	setStorageDir(dir)
	return dir
}

// writeUserResource creates a user resource (theme, word list, quote file)
// inside the storage directory returned by useTempStorage.
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
