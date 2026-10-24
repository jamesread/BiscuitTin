package app

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
)

// hint is one entry of the control help, e.g. "(A) Launch". pad lists the
// gamepad glyphs and keys the keyboard keys for the same action; a hint with
// neither for the current device is skipped.
type hint struct {
	pad   []glyphRole
	keys  []string
	label string
}

// hintsFor returns the control help for the current screen.
func (u *ui) hintsFor() []hint {
	back := hint{[]glyphRole{glyphBack}, []string{"Esc"}, "Back"}
	switch u.screen {
	case screenHome:
		return []hint{
			{[]glyphRole{glyphConfirm}, []string{"Enter"}, "Launch"},
			{[]glyphRole{glyphMove}, []string{"Arrows"}, "Move"},
			{[]glyphRole{glyphBack}, nil, "Hide"},
			{nil, []string{"Esc"}, "Exit"},
			{[]glyphRole{glyphGuide}, nil, "Hold to return from an app"},
		}
	case screenActionList:
		return []hint{
			{[]glyphRole{glyphConfirm}, []string{"Enter"}, "Pin / unpin"},
			{[]glyphRole{glyphMove}, []string{"Up", "Down"}, "Move"},
			{[]glyphRole{glyphLeftRight}, []string{"Left", "Right"}, "Page"},
			back,
		}
	case screenGamepads:
		// B is shown as a held button on this screen rather than used to leave.
		return []hint{{[]glyphRole{glyphConfirm}, []string{"Esc"}, "Back"}}
	case screenDiagnostics:
		return []hint{{[]glyphRole{glyphConfirm, glyphBack}, []string{"Esc"}, "Back"}}
	case screenConfirm:
		return []hint{
			{[]glyphRole{glyphMove}, []string{"Up", "Down"}, "Move"},
			{[]glyphRole{glyphConfirm}, []string{"Enter"}, "Select"},
			back,
		}
	default: // list menus
		return []hint{
			{[]glyphRole{glyphMove}, []string{"Arrows"}, "Move"},
			{[]glyphRole{glyphLeftRight}, []string{"Left", "Right"}, "Change"},
			{[]glyphRole{glyphConfirm}, []string{"Enter"}, "Select"},
			back,
		}
	}
}

// drawHints draws the control help for the primary input device at the
// bottom left: gamepad button glyphs, or keyboard keycaps.
func (u *ui) drawHints(h, s, margin float32) {
	if u.settings.HideHelp {
		return
	}

	usePad := u.input != nil && u.input.primary == deviceGamepad
	layout := layoutXbox
	if usePad {
		layout = detectLayout(u.input.padName())
	}

	size := 28 * s
	glyph := size * 1.35
	gap := 10 * s
	sep := 44 * s
	textColor := rl.ColorAlpha(rl.RayWhite, 0.9)

	// Lay out first, to size the backing behind the whole row.
	type piece struct {
		glyph string // texture name, or "" for a keycap/label
		key   string
		label string
		w     float32
	}
	var pieces []piece
	for _, hn := range u.hintsFor() {
		var row []piece
		if usePad {
			for _, r := range hn.pad {
				if name := glyphFile(r, layout); u.glyphs[name].ID != 0 {
					row = append(row, piece{glyph: name, w: glyph})
				}
			}
		} else {
			for _, k := range hn.keys {
				row = append(row, piece{key: k, w: u.measure(k, size*0.85).X + 22*s})
			}
		}
		if len(row) == 0 {
			continue
		}
		if len(pieces) > 0 {
			pieces = append(pieces, piece{w: sep})
		}
		pieces = append(pieces, row...)
		pieces = append(pieces, piece{label: hn.label, w: u.measure(hn.label, size).X})
	}
	if len(pieces) == 0 {
		return
	}

	total := float32(0)
	for _, p := range pieces {
		total += p.w + gap
	}
	rowH := glyph
	y := h - 60*s - rowH/2

	// One translucent backing for the row, sized for WCAG AA against the
	// brightest part of the image behind it.
	back := rl.Rectangle{X: margin - 16*s, Y: y - 8*s, Width: total + 24*s, Height: rowH + 16*s}
	backing, _ := contrast.SolveBacking(contrast.RGBOf(textColor), float64(textColor.A)/255, u.backdropFor(back.X, back.Y, back.X+back.Width, back.Y+back.Height))
	if backing > 0.01 {
		rl.DrawRectangleRounded(back, 0.5, 10, u.fa(rl.ColorAlpha(rl.Black, float32(backing))))
	}

	u.onTile = true // the backing above already guarantees contrast
	x := margin
	for _, p := range pieces {
		switch {
		case p.glyph != "":
			t := u.glyphs[p.glyph]
			rl.DrawTexturePro(t,
				rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
				rl.Rectangle{X: x, Y: y, Width: glyph, Height: glyph},
				rl.Vector2{}, 0, u.fa(rl.White))
		case p.key != "":
			kr := rl.Rectangle{X: x, Y: y + (rowH-size*1.25)/2, Width: p.w, Height: size * 1.25}
			rl.DrawRectangleRoundedLinesEx(kr, 0.35, 8, 2*s, u.fa(textColor))
			ks := size * 0.85
			km := u.measure(p.key, ks)
			u.text(p.key, kr.X+(kr.Width-km.X)/2, kr.Y+(kr.Height-ks)/2, ks, textColor)
		case p.label != "":
			u.text(p.label, x, y+(rowH-size)/2, size, textColor)
		}
		x += p.w + gap
	}
	u.onTile = false
}
