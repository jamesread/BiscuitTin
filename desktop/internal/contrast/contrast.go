// Package contrast has the colour maths that keeps on-screen text readable:
// WCAG contrast ratios, text backings and image brightness grids.
package contrast

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
)

// WCAG 2.x AA minimum contrast for normal-size text. Applied to all text,
// including large text (which only needs 3:1), so small TV text is covered.
const minContrast = 4.5

var srgbToLinear [256]float64

func init() {
	for i := range srgbToLinear {
		c := float64(i) / 255
		if c <= 0.04045 {
			srgbToLinear[i] = c / 12.92
		} else {
			srgbToLinear[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
}

// RGB is an sRGB colour with channels in 0..1, used for blending maths.
type RGB struct{ r, g, b float64 }

func RGBOf(c color.RGBA) RGB {
	return RGB{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

func (c RGB) Scale(k float64) RGB { return RGB{c.r * k, c.g * k, c.b * k} }

// Over composites c at alpha a Over bg. raylib blends in sRGB space, so this
// matches what reaches the screen.
func (c RGB) Over(bg RGB, a float64) RGB {
	return RGB{c.r*a + bg.r*(1-a), c.g*a + bg.g*(1-a), c.b*a + bg.b*(1-a)}
}

func linear(v float64) float64 {
	return srgbToLinear[int(math.Round(math.Max(0, math.Min(1, v))*255))]
}

// RelativeLuminance is the WCAG definition.
func RelativeLuminance(c RGB) float64 {
	return 0.2126*linear(c.r) + 0.7152*linear(c.g) + 0.0722*linear(c.b)
}

func Ratio(a, b RGB) float64 {
	la, lb := RelativeLuminance(a), RelativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// maxBackingAlpha caps the black backing so it still reads as a translucent
// scrim rather than a solid box; past this, the text itself is made more
// opaque instead.
const maxBackingAlpha = 0.85

// SolveBacking returns the smallest black backing alpha, and the smallest
// text alpha not below the requested one, that give text of colour fg at
// alpha fgAlpha at least minContrast against backdrop.
func contrastOK(fg, backdrop RGB, backing, ta float64) bool {
	bg := backdrop.Scale(1 - backing)
	return Ratio(fg.Over(bg, ta), bg) >= minContrast
}

func searchBacking(fg RGB, fgAlpha float64, backdrop RGB) float64 {
	lo, hi := 0.0, maxBackingAlpha
	for range 12 {
		mid := (lo + hi) / 2
		if contrastOK(fg, backdrop, mid, fgAlpha) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

func searchText(fg RGB, fgAlpha float64, backdrop RGB) float64 {
	lo, hi := fgAlpha, 1.0
	for range 12 {
		mid := (lo + hi) / 2
		if contrastOK(fg, backdrop, maxBackingAlpha, mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

func SolveBacking(fg RGB, fgAlpha float64, backdrop RGB) (backing, textAlpha float64) {
	if contrastOK(fg, backdrop, 0, fgAlpha) {
		return 0, fgAlpha
	}
	if contrastOK(fg, backdrop, maxBackingAlpha, fgAlpha) {
		return searchBacking(fg, fgAlpha, backdrop), fgAlpha
	}
	return maxBackingAlpha, searchText(fg, fgAlpha, backdrop)
}

// Tile glyphs: white icons, and letters drawn at this alpha. The top half of
// each tile has a white highlight at TileHighlightAlpha, the brightest point
// a glyph can sit on.
const (
	TileGlyphAlpha     = 0.92
	TileHighlightAlpha = 0.07
)

// AccessibleTileColor darkens c until white glyphs on it, including over the
// highlight, reach minContrast. Unselected tiles are drawn darker still.
func AccessibleTileColor(c color.RGBA) color.RGBA {
	white := RGB{1, 1, 1}
	for range 64 {
		bg := white.Over(RGBOf(c), TileHighlightAlpha)
		if Ratio(white.Over(bg, TileGlyphAlpha), bg) >= minContrast {
			return c
		}
		c = Shade(c, 0.95)
	}
	return c
}

// LumGrid records the brightest pixel in each cell of an image, so the worst
// case behind any piece of text can be looked up cheaply every frame.
type LumGrid struct {
	cells []RGB
	lums  []float64
	W     int
	H     int
	cols  int
	rows  int
}

const (
	lumGridCols = 96
	lumGridRows = 54
)

func BuildLumGrid(img *image.RGBA) *LumGrid {
	b := img.Bounds()
	g := &LumGrid{W: b.Dx(), H: b.Dy(), cols: lumGridCols, rows: lumGridRows}
	g.cells = make([]RGB, g.cols*g.rows)
	best := make([]float64, len(g.cells))
	for i := range best {
		best[i] = -1
	}
	g.lums = best

	for y := 0; y < g.H; y += 2 {
		row := (y * g.rows / g.H) * g.cols
		for x := 0; x < g.W; x += 2 {
			o := y*img.Stride + x*4
			r, gg, bb := img.Pix[o], img.Pix[o+1], img.Pix[o+2]
			l := 0.2126*srgbToLinear[r] + 0.7152*srgbToLinear[gg] + 0.0722*srgbToLinear[bb]
			i := row + x*g.cols/g.W
			if l > best[i] {
				best[i] = l
				g.cells[i] = RGB{float64(r) / 255, float64(gg) / 255, float64(bb) / 255}
			}
		}
	}
	return g
}

// Brightest returns the Brightest cell colour within an image-space rectangle.
func (g *LumGrid) Brightest(x0, y0, x1, y1 float64) RGB {
	c0 := clampInt(int(x0*float64(g.cols)/float64(g.W)), g.cols-1)
	c1 := clampInt(int(x1*float64(g.cols)/float64(g.W)), g.cols-1)
	r0 := clampInt(int(y0*float64(g.rows)/float64(g.H)), g.rows-1)
	r1 := clampInt(int(y1*float64(g.rows)/float64(g.H)), g.rows-1)

	var out RGB
	best := -1.0
	for r := r0; r <= r1; r++ {
		for c := c0; c <= c1; c++ {
			if l := g.lums[r*g.cols+c]; l > best {
				best, out = l, g.cells[r*g.cols+c]
			}
		}
	}
	return out
}

func clampInt(v, hi int) int {
	return min(max(v, 0), hi)
}

// ParseHex parses #rrggbb, returning def if it is malformed.
func ParseHex(s string, def color.RGBA) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// Shade darkens c by k (0 to 1), keeping its alpha.
func Shade(c color.RGBA, k float32) color.RGBA {
	return color.RGBA{uint8(float32(c.R) * k), uint8(float32(c.G) * k), uint8(float32(c.B) * k), c.A}
}
