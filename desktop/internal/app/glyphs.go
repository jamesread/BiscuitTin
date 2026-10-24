package app

import (
	"embed"
	"path"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/icons"
	"go.uber.org/zap"
)

// Gamepad button glyphs from Tabler Icons (MIT, assets/gamepad/LICENSE),
// embedded so control help needs no network. Rendered white at startup.
//
//go:embed assets/gamepad/*.svg
var gamepadSVGs embed.FS

// glyphRole is what a button does in the UI; the file drawn for it depends on
// the controller's layout.
type glyphRole int

const (
	glyphConfirm glyphRole = iota
	glyphBack
	glyphMove
	glyphLeftRight
	glyphGuide
)

type padLayout int

const (
	layoutXbox padLayout = iota // Xbox and most generic pads (A bottom, B right)
	layoutPlayStation
)

// detectLayout guesses the button labels from the gamepad's name.
func detectLayout(name string) padLayout {
	n := strings.ToLower(name)
	for _, s := range []string{"playstation", "sony", "dualshock", "dualsense", "ps3", "ps4", "ps5"} {
		if strings.Contains(n, s) {
			return layoutPlayStation
		}
	}
	return layoutXbox
}

var glyphXbox = map[glyphRole]string{
	glyphConfirm:   "xbox-a",
	glyphBack:      "xbox-b",
	glyphMove:      "device-gamepad-3",
	glyphLeftRight: "arrows-left-right",
	glyphGuide:     "brand-xbox",
}

var glyphPS = map[glyphRole]string{
	glyphConfirm: "playstation-x",
	glyphBack:    "playstation-circle",
	glyphGuide:   "home",
}

func glyphFile(role glyphRole, layout padLayout) string {
	if layout == layoutPlayStation {
		if name, ok := glyphPS[role]; ok {
			return name
		}
	}
	return glyphXbox[role]
}

const glyphRasterSize = 96

// loadGlyphs renders every embedded glyph to a white texture. Must run on the
// main thread.
func loadGlyph(name string) (rl.Texture2D, bool) {
	raw, err := gamepadSVGs.ReadFile("assets/gamepad/" + name)
	if err != nil {
		return rl.Texture2D{}, false
	}
	img, err := icons.RasterizeSVG(raw, glyphRasterSize)
	if err != nil {
		lg().Warn("gamepad glyph", zap.String("file", name), zap.Error(err))
		return rl.Texture2D{}, false
	}
	icons.TintWhite(img)
	t := textureFromRGBA(img)
	return t, t.ID != 0
}

func loadGlyphs() map[string]rl.Texture2D {
	out := map[string]rl.Texture2D{}
	entries, err := gamepadSVGs.ReadDir("assets/gamepad")
	if err != nil {
		lg().Warn("gamepad glyphs", zap.Error(err))
		return out
	}
	for _, e := range entries {
		t, ok := loadGlyph(e.Name())
		if !ok {
			continue
		}
		out[strings.TrimSuffix(e.Name(), path.Ext(e.Name()))] = t
	}
	return out
}
