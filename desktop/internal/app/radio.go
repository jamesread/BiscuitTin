package app

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
)

// radioGroup is the controller version of picocrank's RadioGroup: a
// segmented row of options with the checked one filled. In a menu row,
// left/right pick the neighbouring option and A cycles through them. It is
// generic over its options; boolRadio covers the common two-state case.
type radioGroup struct {
	options []string
	get     func() int
	set     func(int)
}

// boolRadio makes a two-option group, e.g. "Shown"/"Hidden", bound to a bool.
func boolRadio(trueLabel, falseLabel string, get func() bool, set func(bool)) *radioGroup {
	return &radioGroup{
		options: []string{trueLabel, falseLabel},
		get: func() int {
			if get() {
				return 0
			}
			return 1
		},
		set: func(i int) { set(i == 0) },
	}
}

func (rg *radioGroup) step(dir int) {
	i := clampInt(rg.get()+dir, 0, len(rg.options)-1)
	if i != rg.get() {
		rg.set(i)
	}
}

func (rg *radioGroup) cycle() {
	rg.set((rg.get() + 1) % len(rg.options))
}

// drawRadioGroup draws the group right-aligned to right, at y with height h.
// focused draws the checked segment in the accent colour.
func (u *ui) drawRadioGroup(rg *radioGroup, right, y, h, s float32, focused bool) {
	size := h * 0.52
	padX := size * 0.9
	widths := make([]float32, len(rg.options))
	total := float32(0)
	for i, o := range rg.options {
		widths[i] = u.measure(o, size).X + 2*padX
		total += widths[i]
	}

	outer := rl.Rectangle{X: right - total, Y: y, Width: total, Height: h}
	rl.DrawRectangleRounded(outer, 0.5, 10, u.fa(rl.ColorAlpha(rl.Black, 0.45)))
	rl.DrawRectangleRoundedLinesEx(outer, 0.5, 10, 2*s, u.fa(rl.ColorAlpha(rl.White, 0.35)))

	checked := rg.get()
	x := outer.X
	for i, o := range rg.options {
		seg := rl.Rectangle{X: x, Y: y, Width: widths[i], Height: h}
		ty := y + (h-size)/2
		tx := x + padX
		if i == checked {
			fill := rl.ColorAlpha(rl.RayWhite, 0.9)
			if focused {
				fill = u.accent
			}
			inset := seg
			inset.X += 3 * s
			inset.Y += 3 * s
			inset.Width -= 6 * s
			inset.Height -= 6 * s
			rl.DrawRectangleRounded(inset, 0.5, 10, u.fa(fill))
			// Dark text on the light, opaque checked segment.
			u.onTile = true
			u.text(o, tx, ty, size, u.bg)
			u.onTile = false
		} else {
			prev := u.surface
			u.surface = &surfaceTint{contrast.RGB{}, 0.45}
			u.text(o, tx, ty, size, rl.ColorAlpha(rl.RayWhite, 0.8))
			u.surface = prev
		}
		x += widths[i]
	}
}
