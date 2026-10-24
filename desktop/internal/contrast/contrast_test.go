package contrast

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestContrastRatioReferenceValues(t *testing.T) {
	white, black := RGB{1, 1, 1}, RGB{0, 0, 0}
	if got := Ratio(white, black); math.Abs(got-21) > 0.01 {
		t.Fatalf("white/black = %.3f, want 21", got)
	}
	// #767676 on white is the well-known 4.54:1 AA threshold grey.
	grey := RGBOf(color.RGBA{0x76, 0x76, 0x76, 255})
	if got := Ratio(grey, white); math.Abs(got-4.54) > 0.02 {
		t.Fatalf("#767676/white = %.3f, want 4.54", got)
	}
}

func TestSolveBackingMeetsAA(t *testing.T) {
	white := RGB{0.96, 0.96, 0.96}
	cases := []struct {
		name     string
		backdrop RGB
		alpha    float64
	}{
		{"bright sky", RGB{0.9, 0.92, 0.95}, 1},
		{"mid grey", RGB{0.5, 0.5, 0.5}, 1},
		{"faint hint on white", RGB{1, 1, 1}, 0.45},
		{"faint hint on dark ui", RGBOf(color.RGBA{20, 22, 31, 255}), 0.45},
	}
	for _, c := range cases {
		backing, ta := SolveBacking(white, c.alpha, c.backdrop)
		bg := c.backdrop.Scale(1 - backing)
		got := Ratio(white.Over(bg, ta), bg)
		if got < minContrast-0.01 {
			t.Errorf("%s: contrast %.2f < %.1f (backing %.2f, text alpha %.2f)", c.name, got, minContrast, backing, ta)
		}
		if backing > maxBackingAlpha+1e-9 || ta < c.alpha {
			t.Errorf("%s: backing %.2f / text alpha %.2f out of range", c.name, backing, ta)
		}
	}

	// Already-dark backdrops need no backing at all.
	if backing, _ := SolveBacking(white, 1, RGB{0.05, 0.05, 0.05}); backing != 0 {
		t.Errorf("dark backdrop should need no backing, got %.2f", backing)
	}
}

func TestLumGridFindsBrightestCell(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 192, 108))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255 // opaque black
	}
	// A bright spot in the right half only.
	for y := 40; y < 50; y++ {
		for x := 150; x < 160; x++ {
			o := y*img.Stride + x*4
			img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 250, 250, 250
		}
	}
	g := BuildLumGrid(img)
	if l := RelativeLuminance(g.Brightest(0, 0, 90, 108)); l > 0.01 {
		t.Errorf("left half should be dark, luminance %.3f", l)
	}
	if l := RelativeLuminance(g.Brightest(100, 0, 191, 107)); l < 0.9 {
		t.Errorf("right half should contain the bright spot, luminance %.3f", l)
	}
}

func TestAccessibleTileColor(t *testing.T) {
	white := RGB{1, 1, 1}
	for _, hex := range []string{"#3d5a80", "#f2b134", "#ffffff", "#e4572e", "#76c893"} {
		c := AccessibleTileColor(ParseHex(hex, color.RGBA{}))
		bg := white.Over(RGBOf(c), TileHighlightAlpha)
		if got := Ratio(white.Over(bg, TileGlyphAlpha), bg); got < minContrast {
			t.Errorf("%s -> %v: glyph contrast %.2f < %.1f", hex, c, got, minContrast)
		}
	}
	// A colour that already passes is left alone.
	dark := ParseHex("#1f2a44", color.RGBA{})
	if AccessibleTileColor(dark) != dark {
		t.Error("an already accessible colour should not be changed")
	}
}
