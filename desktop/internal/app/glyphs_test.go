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

// Every glyph a hint can ask for, and the header's gamepad icon, must be
// embedded and render to a visible, non-solid icon.
func TestGlyphsEmbeddedAndRender(t *testing.T) {
	names := []string{padIndicatorGlyph}
	for _, layout := range []padLayout{layoutXbox, layoutPlayStation} {
		for _, role := range []glyphRole{glyphConfirm, glyphBack, glyphMove, glyphLeftRight, glyphGuide} {
			names = append(names, glyphFile(role, layout))
		}
	}
	for _, name := range names {
		checkGlyphRenders(t, name)
	}
}

func checkGlyphRenders(t *testing.T, name string) {
	t.Helper()
	raw, err := gamepadSVGs.ReadFile("assets/gamepad/" + name + ".svg")
	if err != nil {
		t.Errorf("glyph %q not embedded: %v", name, err)
		return
	}
	img, err := icons.RasterizeSVG(raw, 64)
	if err != nil {
		t.Errorf("glyph %q: %v", name, err)
		return
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

func TestPadCountLabel(t *testing.T) {
	for n, want := range map[int]string{0: "", 1: "", 2: "2", 4: "4"} {
		if got := padCountLabel(n); got != want {
			t.Errorf("padCountLabel(%d) = %q; want %q", n, got, want)
		}
	}
}
