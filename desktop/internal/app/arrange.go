package app

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

// tileID names a tile in settings. Entries go by title, as their keys are
// positions that change when the config is edited; pinned actions by key.
func (t tile) tileID() string {
	if t.kind == tileEntry {
		return "entry/" + t.title
	}
	return t.key
}

// orderedTiles is every entry and pinned tile in the saved order, hidden
// ones included. Tiles not in the saved order (new ones) follow, as they
// come in the config and pins.
func (u *ui) orderedTiles() []tile {
	all := u.configuredTiles()
	rank := map[string]int{}
	for i, id := range u.settings.TileOrder {
		rank[id] = i
	}
	sort.SliceStable(all, func(i, j int) bool {
		return rankedBefore(rank, all[i].tileID(), all[j].tileID())
	})
	return all
}

// homeTiles is the home screen order: the saved order, or with Recently
// used, the most recently launched first and never-launched ones after in
// the saved order.
func (u *ui) homeTiles() []tile {
	tiles := u.orderedTiles()
	if !u.settings.RecentFirst {
		return tiles
	}
	used := u.settings.LastUsed
	sort.SliceStable(tiles, func(i, j int) bool {
		return used[tiles[i].tileID()] > used[tiles[j].tileID()]
	})
	return tiles
}

// noteUsed records a launch for Recently used. Tiles don't move until the
// launcher comes back (refreshRecent), so nothing shifts under the cursor.
func (u *ui) noteUsed(t tile, now time.Time) {
	if u.settings.LastUsed == nil {
		u.settings.LastUsed = map[string]int64{}
	}
	u.settings.LastUsed[t.tileID()] = now.Unix()
	u.settingsChanged = true
}

// refreshRecent re-sorts the home screen for Recently used and selects the
// first tile, the one used last. Called when the launcher comes back.
func (u *ui) refreshRecent() {
	if !u.settings.RecentFirst {
		return
	}
	u.rebuildTiles()
	u.selected = 0
}

func (u *ui) tileOrderItem() menuItem {
	return menuItem{
		title: "Tile order",
		radio: boolRadio("Your order", "Recently used", func() bool { return !u.settings.RecentFirst }, func(v bool) {
			u.settings.RecentFirst = !v
			u.tilesChanged()
		}),
	}
}

// configuredTiles is the entries, then the pinned actions.
func (u *ui) configuredTiles() []tile {
	var all []tile
	for i, e := range u.cfg.Entries {
		all = append(all, tile{kind: tileEntry, key: entryKey(i), entry: i, title: e.Title, subtitle: e.Subtitle})
	}
	for _, a := range u.settings.PinnedActions {
		all = append(all, tile{kind: tileAction, key: a.Key(), action: a, title: a.Title, subtitle: a.Group})
	}
	return all
}

// rankedBefore orders saved tiles by rank, ahead of unsaved ones.
func rankedBefore(rank map[string]int, a, b string) bool {
	ra, aok := rank[a]
	rb, bok := rank[b]
	if aok && bok {
		return ra < rb
	}
	return aok && !bok
}

func (u *ui) tileHidden(t tile) bool {
	return slices.Contains(u.settings.HiddenTiles, t.tileID())
}

func (u *ui) setTileHidden(t tile, hidden bool) {
	id := t.tileID()
	u.settings.HiddenTiles = slices.DeleteFunc(u.settings.HiddenTiles, func(h string) bool { return h == id })
	if hidden {
		u.settings.HiddenTiles = append(u.settings.HiddenTiles, id)
	}
	u.tilesChanged()
}

func (u *ui) tilesChanged() {
	u.settingsChanged = true
	u.rebuildTiles()
}

// moveTile swaps tile i of orderedTiles with its neighbour in dir, saving
// the whole order. It returns the tile's new index.
func (u *ui) moveTile(i, dir int) int {
	tiles := u.orderedTiles()
	j := i + dir
	if i < 0 || j < 0 || j >= len(tiles) {
		return i
	}
	tiles[i], tiles[j] = tiles[j], tiles[i]
	order := make([]string, len(tiles))
	for k, t := range tiles {
		order[k] = t.tileID()
	}
	u.settings.TileOrder = order
	// Moving a tile means you want your order on the home screen.
	u.settings.RecentFirst = false
	u.tilesChanged()
	return j
}

func (u *ui) hiddenCount() int {
	n := 0
	for _, t := range u.orderedTiles() {
		if u.tileHidden(t) {
			n++
		}
	}
	return n
}

func (u *ui) arrangeItem() menuItem {
	return menuItem{
		title: "Arrange tiles",
		open:  func() { u.arrangeMoving = false; u.openMenu(screenArrange) },
		value: func() string { return fmt.Sprintf("%d hidden", u.hiddenCount()) },
	}
}

// arrangeMenu lists the tiles in home screen order. Left and right show or
// hide a tile; A picks it up so up and down move it, and A or Back drops it.
func (u *ui) arrangeMenu() []menuItem {
	var items []menuItem
	for i, t := range u.orderedTiles() {
		items = append(items, u.arrangeTileItem(i, t))
	}
	return append(items, menuItem{title: "Back", open: func() { u.openMenu(screenSettings) }})
}

func (u *ui) arrangeTileItem(i int, t tile) menuItem {
	if u.arrangeMoving && i == u.menuSel {
		return menuItem{title: "> " + t.title, value: func() string { return "Moving" }}
	}
	return menuItem{
		title: t.title,
		radio: boolRadio("Shown", "Hidden", func() bool { return !u.tileHidden(t) }, func(v bool) { u.setTileHidden(t, !v) }),
	}
}

// handleArrange takes the actions that differ from a plain list menu.
func (u *ui) handleArrange(a action, items []menuItem) bool {
	if u.arrangeMoving {
		u.handleArrangeMoving(a)
		return true
	}
	if a == actionActivate && u.menuSel < len(items)-1 {
		u.arrangeMoving = true
		return true
	}
	return false
}

func (u *ui) handleArrangeMoving(a action) {
	switch a {
	case actionUp:
		u.menuSel = u.moveTile(u.menuSel, -1)
	case actionDown:
		u.menuSel = u.moveTile(u.menuSel, 1)
	case actionActivate, actionBack, actionExit:
		u.arrangeMoving = false
	}
}

func (u *ui) arrangeHints() []hint {
	if u.arrangeMoving {
		return []hint{
			{pad: []glyphRole{glyphMove}, keys: []string{"Up", "Down"}, label: "Move tile"},
			{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Drop"},
		}
	}
	return []hint{
		{pad: []glyphRole{glyphMove}, keys: []string{"Up", "Down"}, label: "Select"},
		{pad: []glyphRole{glyphLeftRight}, keys: []string{"Left", "Right"}, label: "Show / hide"},
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Pick up"},
		backHint(),
	}
}
