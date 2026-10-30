package app

import (
	"slices"
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/integrations"
	"github.com/jamesread/biscuittin/internal/settings"
)

func arrangeUI() *ui {
	cfg := &config.Config{Entries: []config.Entry{{Title: "Kodi"}, {Title: "Steam"}, {Title: "RetroArch"}}}
	st := &settings.Settings{PinnedActions: []integrations.PinnedAction{{Source: "olivetin", BindingID: "lights", Title: "Lights"}}}
	u := &ui{cfg: cfg, settings: st, scales: map[string]float32{}}
	u.rebuildTiles()
	return u
}

func homeTitles(u *ui) []string {
	var out []string
	for _, t := range u.tiles {
		out = append(out, t.title)
	}
	return out
}

func TestArrangeMoveAndHide(t *testing.T) {
	u := arrangeUI()
	u.openMenu(screenArrange)
	if got := menuTitles(u.arrangeMenu()); !slices.Equal(got, []string{"Kodi", "Steam", "RetroArch", "Lights", "Back"}) {
		t.Fatalf("arrange = %v", got)
	}

	// Pick up Kodi, move it down twice, drop it.
	u.handleMenu(actionActivate)
	u.handleMenu(actionDown)
	u.handleMenu(actionDown)
	u.handleMenu(actionActivate)
	if u.arrangeMoving || u.menuSel != 2 {
		t.Fatalf("moving=%v sel=%d", u.arrangeMoving, u.menuSel)
	}
	if got := homeTitles(u); !slices.Equal(got, []string{"Steam", "RetroArch", "Kodi", "Lights", "More"}) {
		t.Fatalf("home = %v", got)
	}

	// Right on Lights hides it; it stays in the arrange list.
	u.menuSel = 3
	u.handleMenu(actionRight)
	if got := homeTitles(u); !slices.Equal(got, []string{"Steam", "RetroArch", "Kodi", "More"}) {
		t.Fatalf("home after hide = %v", got)
	}
	if !slices.Equal(u.settings.HiddenTiles, []string{"act:olivetin:lights"}) || !u.settingsChanged {
		t.Fatalf("hidden = %v changed = %v", u.settings.HiddenTiles, u.settingsChanged)
	}
	if got := u.arrangeItem().value(); got != "1 hidden" {
		t.Fatalf("arrange value = %q", got)
	}
}

func TestArrangeOrderSurvivesConfigEdits(t *testing.T) {
	u := arrangeUI()
	u.settings.TileOrder = []string{"entry/RetroArch", "entry/Kodi", "entry/Gone"}
	u.cfg.Entries = append([]config.Entry{{Title: "New"}}, u.cfg.Entries...)
	u.rebuildTiles()
	// Saved tiles first, in order; the rest as configured; stale IDs ignored.
	if got := homeTitles(u); !slices.Equal(got, []string{"RetroArch", "Kodi", "New", "Steam", "Lights", "More"}) {
		t.Fatalf("home = %v", got)
	}
	if u.tiles[0].entry != 3 {
		t.Fatalf("RetroArch should launch entry 3, got %d", u.tiles[0].entry)
	}
}

func TestArrangeBackWhileMovingDrops(t *testing.T) {
	u := arrangeUI()
	u.openMenu(screenArrange)
	u.handleMenu(actionActivate)
	u.handleMenu(actionBack)
	if u.screen != screenArrange || u.arrangeMoving {
		t.Fatalf("screen=%d moving=%v", u.screen, u.arrangeMoving)
	}
	u.handleMenu(actionBack)
	if u.screen != screenSettings {
		t.Fatalf("second Back should leave, screen=%d", u.screen)
	}
}
