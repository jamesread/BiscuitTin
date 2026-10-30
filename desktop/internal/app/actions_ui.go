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
	err       string
	actions   []integrations.ListedAction
	sel       int
	loading   bool
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
		pins = append(pins[:i:i], pins[i+1:]...)
		u.settings.PinnedActions = pins
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
	if a == actionBack || a == actionExit {
		u.screen = screenIntegrations
		return
	}
	u.moveActionSel(l, a)
	u.activateAction(l, a)
	l.sel = clampInt(l.sel, max(len(l.actions)-1, 0))
}

func (u *ui) moveActionSel(l *uiActionList, a action) {
	delta := map[action]int{
		actionUp:    -1,
		actionDown:  1,
		actionLeft:  -actionPageRows,
		actionRight: actionPageRows,
	}
	l.sel += delta[a]
}

func (u *ui) activateAction(l *uiActionList, a action) {
	if a != actionActivate {
		return
	}
	if len(l.actions) > 0 {
		u.togglePin(l.actions[l.sel])
		return
	}
	if !l.loading {
		u.openActionList(u.listSource) // retry after an error
	}
}

func (u *ui) drawActionList(w, h, s, margin float32) {
	src := u.sourceByID(u.listSource)
	if src == nil {
		return
	}
	l := u.list(u.listSource)
	heading := fmt.Sprintf("%s   %d pinned", src.Title(), u.pinnedCount(src.ID()))
	u.text(u.fitText(heading, 52*s, w-2*margin), margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))
	if u.drawActionStatus(src, l, w, s, margin) {
		return
	}
	u.drawActionRows(l, w, h, s, margin)
}

func (u *ui) drawActionStatus(src integrations.Source, l *uiActionList, w, s, margin float32) bool {
	top := 280 * s
	if l.loading {
		u.text("Loading from "+src.Where()+"...", margin, top, 34*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return true
	}
	if l.err != "" {
		u.text(u.fitText("Could not load: "+l.err, 30*s, w-2*margin), margin, top, 30*s, colorBad)
		u.text("Press A to retry", margin, top+50*s, 30*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return true
	}
	if len(l.actions) == 0 {
		u.text("Nothing to pin was found on "+src.Where(), margin, top, 30*s, rl.ColorAlpha(rl.RayWhite, 0.75))
		return true
	}
	return false
}

func (u *ui) drawActionRows(l *uiActionList, w, h, s, margin float32) {
	top := 280 * s
	rowH := 58 * s
	visible := max(int((h-130*s-top)/rowH), 1)
	first := clampInt(l.sel-visible/2, max(len(l.actions)-visible, 0))
	for row := range visible {
		i := first + row
		if i >= len(l.actions) {
			return
		}
		u.drawActionRow(l.actions[i], i == l.sel, margin, top+float32(row)*rowH, w-2*margin, rowH, s)
	}
}

func actionRowFill(focus rl.Color, selected bool) rl.Color {
	if selected {
		return rl.ColorAlpha(focus, 0.22)
	}
	return rl.ColorAlpha(rl.Black, 0.3)
}

func (u *ui) actionRowLabels(a integrations.ListedAction) (box string, textColor rl.Color, group string) {
	box, textColor, group = "[   ]", rl.RayWhite, a.Group
	if u.pinnedIndex(a.Key()) >= 0 {
		box = "[ x ]"
	}
	if a.NeedsInput {
		box = "  -  "
		textColor = rl.ColorAlpha(rl.RayWhite, 0.6)
		group = "needs input  /  " + group
	}
	return box, textColor, group
}

func (u *ui) drawActionRow(a integrations.ListedAction, selected bool, x, y, listW, rowH, s float32) {
	r := rl.Rectangle{X: x, Y: y, Width: listW, Height: rowH - 8*s}
	fill := actionRowFill(u.focusFill(), selected)
	rl.DrawRectangleRounded(r, 0.3, 8, u.fa(fill))
	if selected {
		rl.DrawRectangleRoundedLinesEx(r, 0.3, 8, 3*s, u.fa(u.focusBorder()))
	}
	box, textColor, group := u.actionRowLabels(a)
	u.surface = &surfaceTint{contrast.RGBOf(fill), float64(fill.A) / 255}
	u.paintActionRow(r, y, listW, rowH, s, box, a.Title, group, textColor)
	u.surface = nil
}

func (u *ui) paintActionRow(r rl.Rectangle, y, listW, rowH, s float32, box, title, group string, textColor rl.Color) {
	size := 30 * s
	ty := y + (rowH-8*s-size)/2
	u.text(box, r.X+20*s, ty, size, u.accentText())
	groupW := listW * 0.38
	u.text(u.fitText(title, size, listW-groupW-170*s), r.X+130*s, ty, size, textColor)
	group = u.fitText(group, 26*s, groupW-30*s)
	gm := u.measure(group, 26*s)
	u.text(group, r.X+r.Width-24*s-gm.X, y+(rowH-8*s-26*s)/2, 26*s, rl.ColorAlpha(rl.RayWhite, 0.65))
}
