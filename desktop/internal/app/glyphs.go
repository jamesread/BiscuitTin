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

func glyphFile(role glyphRole, layout padLayout) string {
	ps := layout == layoutPlayStation
	switch role {
	case glyphConfirm:
		if ps {
			return "playstation-x"
		}
		return "xbox-a"
	case glyphBack:
		if ps {
			return "playstation-circle"
		}
		return "xbox-b"
	case glyphMove:
		return "device-gamepad-3"
	case glyphLeftRight:
		return "arrows-left-right"
	case glyphGuide:
		if ps {
			return "home"
		}
		return "brand-xbox"
	}
	return ""
}

const glyphRasterSize = 96

// loadGlyphs renders every embedded glyph to a white texture. Must run on the
// main thread.
func loadGlyphs() map[string]rl.Texture2D {
	out := map[string]rl.Texture2D{}
	entries, err := gamepadSVGs.ReadDir("assets/gamepad")
	if err != nil {
		lg().Warn("gamepad glyphs", zap.Error(err))
		return out
	}
	for _, e := range entries {
		raw, err := gamepadSVGs.ReadFile("assets/gamepad/" + e.Name())
		if err != nil {
			continue
		}
		img, err := icons.RasterizeSVG(raw, glyphRasterSize)
		if err != nil {
			lg().Warn("gamepad glyph", zap.String("file", e.Name()), zap.Error(err))
			continue
		}
		icons.TintWhite(img)
		t := textureFromRGBA(img)
		if t.ID != 0 {
			out[strings.TrimSuffix(e.Name(), path.Ext(e.Name()))] = t
		}
	}
	return out
}
