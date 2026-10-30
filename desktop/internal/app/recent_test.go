package app

import (
	"slices"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/settings"
)

func recentUI(names ...string) *ui {
	var entries []config.Entry
	for _, n := range names {
		entries = append(entries, config.Entry{Title: n, Command: []string{"true"}})
	}
	cfg := &config.Config{Entries: entries}
	u := &ui{cfg: cfg, settings: &settings.Settings{}, scales: map[string]float32{}}
	u.rebuildTiles()
	return u
}

func tileTitles(u *ui) []string {
	var out []string
	for _, t := range u.tiles {
		out = append(out, t.title)
	}
	return out
}

func TestRecentFirstOrder(t *testing.T) {
	u := recentUI("A", "B", "C", "D")
	u.settings.TileOrder = []string{"entry/D", "entry/C", "entry/B", "entry/A"}
	now := time.Unix(10000, 0)
	u.noteUsed(tile{kind: tileEntry, title: "B"}, now)
	u.noteUsed(tile{kind: tileEntry, title: "A"}, now.Add(time.Minute))
	if !u.settingsChanged {
		t.Fatal("a launch should be saved")
	}

	u.rebuildTiles()
	if got := tileTitles(u); !slices.Equal(got, []string{"D", "C", "B", "A", "More"}) {
		t.Fatalf("your order = %v", got)
	}
	u.settings.RecentFirst = true
	u.selected = 3
	u.refreshRecent()
	// Most recent first, then never used in the saved order.
	if got := tileTitles(u); !slices.Equal(got, []string{"A", "B", "D", "C", "More"}) {
		t.Fatalf("recently used = %v", got)
	}
	if u.selected != 0 {
		t.Fatalf("selected = %d, want the most recent", u.selected)
	}
}

func TestNoteUsedDoesNotReorderUntilShown(t *testing.T) {
	u := recentUI("A", "B")
	u.settings.RecentFirst = true
	u.rebuildTiles()
	u.noteUsed(tile{kind: tileEntry, title: "B"}, time.Now())
	if got := tileTitles(u); !slices.Equal(got, []string{"A", "B", "More"}) {
		t.Fatalf("tiles moved on launch: %v", got)
	}
}

func TestMovingTileReturnsToYourOrder(t *testing.T) {
	u := recentUI("A", "B")
	u.settings.RecentFirst = true
	u.moveTile(0, 1)
	if u.settings.RecentFirst {
		t.Fatal("arranging should switch back to your order")
	}
}

func TestLaunchRecordsUse(t *testing.T) {
	s := testSession([]config.Entry{{Title: "True", Command: []string{"true"}}})
	s.cfg.LogDir = t.TempDir()
	s.launchEntry(s.u.tiles[0])
	if s.u.settings.LastUsed["entry/True"] == 0 {
		t.Fatalf("lastUsed = %v", s.u.settings.LastUsed)
	}
}
