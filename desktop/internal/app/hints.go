package app

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
)

// hint is one entry of the control help, e.g. "(A) Launch". pad lists the
// gamepad glyphs and keys the keyboard keys for the same action; a hint with
// neither for the current device is skipped.
type hint struct {
	label string
	pad   []glyphRole
	keys  []string
}

func backHint() hint {
	return hint{pad: []glyphRole{glyphBack}, keys: []string{"Esc"}, label: "Back"}
}

func homeHints() []hint {
	return []hint{
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Launch"},
		{pad: []glyphRole{glyphMove}, keys: []string{"Arrows"}, label: "Move"},
		{pad: []glyphRole{glyphBack}, label: "Hide"},
		{keys: []string{"Esc"}, label: "Exit"},
		{pad: []glyphRole{glyphGuide}, label: "Hold to return from an app"},
	}
}

func actionListHints() []hint {
	return []hint{
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Pin / unpin"},
		{pad: []glyphRole{glyphMove}, keys: []string{"Up", "Down"}, label: "Move"},
		{pad: []glyphRole{glyphLeftRight}, keys: []string{"Left", "Right"}, label: "Page"},
		backHint(),
	}
}

func gamepadHints() []hint {
	// B is shown as a held button on this screen rather than used to leave.
	return []hint{{pad: []glyphRole{glyphConfirm}, keys: []string{"Esc"}, label: "Back"}}
}

func diagnosticsHints() []hint {
	return []hint{{pad: []glyphRole{glyphConfirm, glyphBack}, keys: []string{"Esc"}, label: "Back"}}
}

func confirmHints() []hint {
	return []hint{
		{pad: []glyphRole{glyphMove}, keys: []string{"Up", "Down"}, label: "Move"},
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Select"},
		backHint(),
	}
}

func listMenuHints() []hint {
	return []hint{
		{pad: []glyphRole{glyphMove}, keys: []string{"Arrows"}, label: "Move"},
		{pad: []glyphRole{glyphLeftRight}, keys: []string{"Left", "Right"}, label: "Change"},
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Select"},
		backHint(),
	}
}

var screenHints = map[screen]func() []hint{
	screenHome:        homeHints,
	screenActionList:  actionListHints,
	screenGamepads:    gamepadHints,
	screenDiagnostics: diagnosticsHints,
	screenAudio:       audioHints,
	screenConfirm:     confirmHints,
	screenRunning:     confirmHints,
}

// hintsFor returns the control help for the current screen.
func (u *ui) hintsFor() []hint {
	if fn, ok := screenHints[u.screen]; ok {
		return fn()
	}
	return listMenuHints()
}

type hintPiece struct {
	glyph string // texture name, or "" for a keycap/label
	key   string
	label string
	w     float32
}

type hintLayout struct {
	pieces    []hintPiece
	size      float32
	glyph     float32
	gap       float32
	textColor rl.Color
}

func (u *ui) hintDevice() (bool, padLayout) {
	if u.input == nil {
		return false, layoutXbox
	}
	if u.input.primary != deviceGamepad {
		return false, layoutXbox
	}
	return true, detectLayout(u.input.padName())
}

func (u *ui) hintPieces(s float32) hintLayout {
	usePad, layout := u.hintDevice()
	size := 28 * s
	glyph := size * 1.35
	var pieces []hintPiece
	for _, hn := range u.hintsFor() {
		pieces = u.appendHint(pieces, hn, usePad, layout, size, glyph, s)
	}
	return hintLayout{pieces: pieces, size: size, glyph: glyph, gap: 10 * s, textColor: rl.ColorAlpha(rl.RayWhite, 0.9)}
}

func (u *ui) appendHint(pieces []hintPiece, hn hint, usePad bool, layout padLayout, size, glyph, s float32) []hintPiece {
	row := u.hintRow(hn, usePad, layout, size, glyph, s)
	if len(row) == 0 {
		return pieces
	}
	if len(pieces) > 0 {
		pieces = append(pieces, hintPiece{w: 44 * s})
	}
	pieces = append(pieces, row...)
	return append(pieces, hintPiece{label: hn.label, w: u.measure(hn.label, size).X})
}

func (u *ui) hintRow(hn hint, usePad bool, layout padLayout, size, glyph, s float32) []hintPiece {
	if usePad {
		return u.hintPadRow(hn, layout, glyph)
	}
	return u.hintKeyRow(hn, size, s)
}

func (u *ui) hintPadRow(hn hint, layout padLayout, glyph float32) []hintPiece {
	var row []hintPiece
	for _, r := range hn.pad {
		if name := glyphFile(r, layout); u.glyphs[name].ID != 0 {
			row = append(row, hintPiece{glyph: name, w: glyph})
		}
	}
	return row
}

func (u *ui) hintKeyRow(hn hint, size, s float32) []hintPiece {
	row := make([]hintPiece, 0, len(hn.keys))
	for _, k := range hn.keys {
		row = append(row, hintPiece{key: k, w: u.measure(k, size*0.85).X + 22*s})
	}
	return row
}

// drawHints draws the control help for the primary input device at the
// bottom left: gamepad button glyphs, or keyboard keycaps.
func (u *ui) drawHints(h, s, margin float32) {
	if u.settings.HideHelp {
		return
	}
	lay := u.hintPieces(s)
	if len(lay.pieces) == 0 {
		return
	}
	u.paintHints(lay, h, s, margin)
}

func (u *ui) paintHints(lay hintLayout, h, s, margin float32) {
	total := hintRowWidth(lay)
	rowH := lay.glyph
	y := h - 60*s - rowH/2
	// One translucent backing for the row, sized for WCAG AA against the
	// brightest part of the image behind it.
	back := rl.Rectangle{X: margin - 16*s, Y: y - 8*s, Width: total + 24*s, Height: rowH + 16*s}
	u.drawHintBacking(back, lay.textColor)
	u.onTile = true // the backing above already guarantees contrast
	u.paintHintRow(lay, margin, y, rowH, s)
	u.onTile = false
}

func hintRowWidth(lay hintLayout) float32 {
	total := float32(0)
	for _, p := range lay.pieces {
		total += p.w + lay.gap
	}
	return total
}

func (u *ui) drawHintBacking(back rl.Rectangle, textColor rl.Color) {
	backing, _ := contrast.SolveBacking(contrast.RGBOf(textColor), float64(textColor.A)/255, u.backdropFor(back.X, back.Y, back.X+back.Width, back.Y+back.Height))
	if backing > 0.01 {
		rl.DrawRectangleRounded(back, 0.5, 10, u.fa(rl.ColorAlpha(rl.Black, float32(backing))))
	}
}

func (u *ui) paintHintRow(lay hintLayout, x, y, rowH, s float32) {
	for _, p := range lay.pieces {
		u.paintHintPiece(p, lay, x, y, rowH, s)
		x += p.w + lay.gap
	}
}

func (u *ui) paintHintPiece(p hintPiece, lay hintLayout, x, y, rowH, s float32) {
	if p.glyph != "" {
		u.paintHintGlyph(p.glyph, x, y, lay.glyph)
		return
	}
	if p.key != "" {
		u.paintHintKey(p, lay, x, y, rowH, s)
		return
	}
	if p.label != "" {
		u.text(p.label, x, y+(rowH-lay.size)/2, lay.size, lay.textColor)
	}
}

func (u *ui) paintHintGlyph(name string, x, y, glyph float32) {
	t := u.glyphs[name]
	rl.DrawTexturePro(t,
		rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
		rl.Rectangle{X: x, Y: y, Width: glyph, Height: glyph},
		rl.Vector2{}, 0, u.fa(rl.White))
}

func (u *ui) paintHintKey(p hintPiece, lay hintLayout, x, y, rowH, s float32) {
	kr := rl.Rectangle{X: x, Y: y + (rowH-lay.size*1.25)/2, Width: p.w, Height: lay.size * 1.25}
	rl.DrawRectangleRoundedLinesEx(kr, 0.35, 8, 2*s, u.fa(lay.textColor))
	ks := lay.size * 0.85
	km := u.measure(p.key, ks)
	u.text(p.key, kr.X+(kr.Width-km.X)/2, kr.Y+(kr.Height-ks)/2, ks, lay.textColor)
}
