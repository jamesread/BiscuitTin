package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
)

func TestFindMissing(t *testing.T) {
	old := entryInstalled
	t.Cleanup(func() { entryInstalled = old })
	entryInstalled = func(cmd []string) (bool, bool) {
		switch cmd[0] {
		case "present":
			return true, true
		case "gone":
			return false, true
		}
		return false, false // couldn't check: not flagged
	}
	r := findMissing([]config.Entry{
		{Title: "A", Command: []string{"present"}},
		{Title: "B", Command: []string{"gone"}},
		{Title: "C", Command: []string{"unknown"}},
	}, 3)
	if r.gen != 3 || len(r.missing) != 1 || !r.missing[entryKey(1)] {
		t.Fatalf("result = %+v", r)
	}
}

func TestDrainInstalledDropsStale(t *testing.T) {
	s := testSession([]config.Entry{{Title: "Gone", Command: []string{"gone"}}})
	s.u.tiles[0].key = entryKey(0)
	s.installedGen = 2
	s.installedResults <- installedResult{gen: 1, missing: map[string]bool{entryKey(0): true}}
	s.drainInstalled()
	if s.u.tileMissing(s.u.tiles[0]) {
		t.Fatal("a result from before a reload should be dropped")
	}
	s.installedResults <- installedResult{gen: 2, missing: map[string]bool{entryKey(0): true}}
	s.drainInstalled()
	if !s.u.tileMissing(s.u.tiles[0]) {
		t.Fatal("current result should mark the tile")
	}
}

func TestCheckInstalledSkippedInScripts(t *testing.T) {
	s := testSession([]config.Entry{{Title: "Gone", Command: []string{"gone"}}})
	s.sc = &script{}
	s.checkInstalled(s.cfg)
	if s.installedGen != 0 {
		t.Fatal("scripted runs should not check")
	}
}
