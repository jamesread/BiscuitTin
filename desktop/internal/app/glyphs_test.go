package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/icons"
)

func TestDetectLayout(t *testing.T) {
	cases := map[string]padLayout{
		"Microsoft X-Box 360 pad":                            layoutXbox,
		"Xbox Wireless Controller":                           layoutXbox,
		"8BitDo SN30 Pro":                                    layoutXbox,
		"Sony Interactive Entertainment Wireless Controller": layoutPlayStation,
		"PS4 Controller":                                     layoutPlayStation,
		"DualSense Wireless Controller":                      layoutPlayStation,
	}
	for name, want := range cases {
		if got := detectLayout(name); got != want {
			t.Errorf("detectLayout(%q) = %v; want %v", name, got, want)
		}
	}
}

// Every glyph a hint can ask for must be embedded and render to a visible,
// non-solid icon.
func TestGlyphsEmbeddedAndRender(t *testing.T) {
	for _, layout := range []padLayout{layoutXbox, layoutPlayStation} {
		for _, role := range []glyphRole{glyphConfirm, glyphBack, glyphMove, glyphLeftRight, glyphGuide} {
			name := glyphFile(role, layout)
			raw, err := gamepadSVGs.ReadFile("assets/gamepad/" + name + ".svg")
			if err != nil {
				t.Errorf("glyph %q (role %d, layout %d) not embedded: %v", name, role, layout, err)
				continue
			}
			img, err := icons.RasterizeSVG(raw, 64)
			if err != nil {
				t.Errorf("glyph %q: %v", name, err)
				continue
			}
			covered := 0
			for i := 3; i < len(img.Pix); i += 4 {
				if img.Pix[i] > 128 {
					covered++
				}
			}
			if frac := float64(covered) / (64 * 64); frac < 0.05 || frac > 0.8 {
				t.Errorf("glyph %q coverage %.2f; want a visible outline, not empty or solid", name, frac)
			}
		}
	}
}
