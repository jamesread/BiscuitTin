package app

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/integrations"
)

// uiActionList is the state of one source's pin list (e.g. More > OliveTin
// actions).
type uiActionList struct {
	actions []integrations.ListedAction
	loading bool
	err     string
	sel     int
	// requested asks the main loop to (re)load the list.
	requested bool
}

const actionPageRows = 8

func (u *ui) sourceByID(id string) integrations.Source {
	for _, s := range u.sources {
		if s.ID() == id {
			return s
		}
	}
	return nil
}

func (u *ui) list(id string) *uiActionList {
	l, ok := u.lists[id]
	if !ok {
		l = &uiActionList{}
		u.lists[id] = l
	}
	return l
}

func (u *ui) openActionList(id string) {
	u.screen = screenActionList
	u.listSource = id
	l := u.list(id)
	l.sel = 0
	if !l.loading {
		l.loading = true
		l.err = ""
		l.requested = true
	}
}

func (u *ui) pinnedCount(source string) int {
	n := 0
	for _, a := range u.settings.PinnedActions {
		if a.SourceID() == source {
			n++
		}
	}
	return n
}

func (u *ui) pinnedIndex(key string) int {
	for i, a := range u.settings.PinnedActions {
		if a.Key() == key {
			return i
		}
	}
	return -1
}

func (u *ui) togglePin(a integrations.ListedAction) {
	if i := u.pinnedIndex(a.Key()); i >= 0 {
		pins := u.settings.PinnedActions
		u.settings.PinnedActions = append(pins[:i:i], pins[i+1:]...)
	} else {
		if a.NeedsInput {
			u.setToast(a.Title+" needs input, so it can't be a button", colorBad, 4)
			return
		}
		u.settings.PinnedActions = append(u.settings.PinnedActions, a.PinnedAction)
	}
	u.settingsChanged = true
	u.rebuildTiles()
}

func (u *ui) handleActionList(a action) {
	l := u.list(u.listSource)
	n := len(l.actions)
	switch a {
	case actionBack, actionExit:
		u.screen = screenIntegrations
		return
	case actionUp:
		l.sel--
	case actionDown:
		l.sel++
	case actionLeft:
		l.sel -= actionPageRows
	case actionRight:
		l.sel += actionPageRows
	case actionActivate:
		if n > 0 {
			u.togglePin(l.actions[l.sel])
		} else if !l.loading {
			u.openActionList(u.listSource) // retry after an error
		}
	}
	l.sel = clampInt(l.sel, 0, max(n-1, 0))
}

func (u *ui) drawActionList(w, h, s, margin float32) {
	src := u.sourceByID(u.listSource)
	if src == nil {
		return
	}
	l := u.list(u.listSource)

	heading := fmt.Sprintf("%s   %d pinned", src.Title(), u.pinnedCount(src.ID()))
	u.text(u.fitText(heading, 52*s, w-2*margin), margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))

	top := 280 * s
	switch {
	case l.loading:
		u.text("Loading from "+src.Where()+"...", margin, top, 34*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return
	case l.err != "":
		u.text(u.fitText("Could not load: "+l.err, 30*s, w-2*margin), margin, top, 30*s, colorBad)
		u.text("Press A to retry", margin, top+50*s, 30*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return
	case len(l.actions) == 0:
		u.text("Nothing to pin was found on "+src.Where(), margin, top, 30*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return
	}

	rowH := 58 * s
	visible := max(int((h-130*s-top)/rowH), 1)
	first := clampInt(l.sel-visible/2, 0, max(len(l.actions)-visible, 0))

	listW := w - 2*margin
	size := 30 * s
	for row := 0; row < visible && first+row < len(l.actions); row++ {
		i := first + row
		a := l.actions[i]
		y := top + float32(row)*rowH
		r := rl.Rectangle{X: margin, Y: y, Width: listW, Height: rowH - 8*s}
		selected := i == l.sel

		fill := rl.ColorAlpha(rl.Black, 0.3)
		if selected {
			fill = rl.ColorAlpha(u.accent, 0.22)
		}
		rl.DrawRectangleRounded(r, 0.3, 8, u.fa(fill))
		if selected {
			rl.DrawRectangleRoundedLinesEx(r, 0.3, 8, 3*s, u.fa(u.accent))
		}

		box := "[   ]"
		if u.pinnedIndex(a.Key()) >= 0 {
			box = "[ x ]"
		}
		textColor := rl.RayWhite
		group := a.Group
		if a.NeedsInput {
			box = "  -  "
			textColor = rl.ColorAlpha(rl.RayWhite, 0.6)
			group = "needs input  /  " + group
		}

		u.surface = &surfaceTint{contrast.RGBOf(fill), float64(fill.A) / 255}
		ty := y + (rowH-8*s-size)/2
		u.text(box, r.X+20*s, ty, size, u.accent)
		groupW := listW * 0.38
		u.text(u.fitText(a.Title, size, listW-groupW-170*s), r.X+130*s, ty, size, textColor)
		group = u.fitText(group, 26*s, groupW-30*s)
		gm := u.measure(group, 26*s)
		u.text(group, r.X+r.Width-24*s-gm.X, y+(rowH-8*s-26*s)/2, 26*s, rl.ColorAlpha(rl.RayWhite, 0.65))
		u.surface = nil
	}
}
