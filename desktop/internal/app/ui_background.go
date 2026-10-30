package app

import (
	"image"
	"math"
	"runtime"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/backgrounds"
	"github.com/jamesread/biscuittin/internal/contrast"
)

const bgFadeSeconds = 1.2

type uiBackground struct {
	curGrid  *contrast.LumGrid
	prevGrid *contrast.LumGrid
	credit   string
	title    string
	cur      rl.Texture2D
	prev     rl.Texture2D
	fade     float32
}

// setBackground uploads a decoded image and starts a crossfade to it.
// Must run on the main thread.
func (u *ui) setBackground(img backgrounds.Image) {
	t := textureFromRGBA(img.Pix)
	if t.ID == 0 {
		return
	}
	if u.bgTex.prev.ID != 0 {
		rl.UnloadTexture(u.bgTex.prev)
	}
	u.bgTex.prev, u.bgTex.prevGrid = u.bgTex.cur, u.bgTex.curGrid
	u.bgTex.cur, u.bgTex.curGrid = t, img.Grid
	u.bgTex.credit, u.bgTex.title = img.Credit, img.Title
	u.bgTex.fade = 0
}

// clearBackground fades the current image out.
func (u *ui) clearBackground() {
	if u.bgTex.cur.ID == 0 {
		return
	}
	if u.bgTex.prev.ID != 0 {
		rl.UnloadTexture(u.bgTex.prev)
	}
	u.bgTex.prev, u.bgTex.prevGrid = u.bgTex.cur, u.bgTex.curGrid
	u.bgTex.cur, u.bgTex.curGrid = rl.Texture2D{}, nil
	u.bgTex.credit, u.bgTex.title = "", ""
	u.bgTex.fade = 0
}

func (u *ui) unloadPrevBackground() {
	b := &u.bgTex
	if b.prev.ID == 0 {
		return
	}
	rl.UnloadTexture(b.prev)
	b.prev, b.prevGrid = rl.Texture2D{}, nil
}

func (u *ui) advanceBgFade(dt float32) {
	b := &u.bgTex
	if b.fade >= 1 {
		return
	}
	b.fade += dt / bgFadeSeconds
	if b.fade < 1 {
		return
	}
	b.fade = 1
	u.unloadPrevBackground()
}

func (u *ui) drawBgOverlays(w, h float32) {
	b := &u.bgTex
	if b.prev.ID == 0 && b.cur.ID == 0 {
		return
	}
	o := u.overlays(h)
	rl.DrawRectangle(0, 0, int32(w), int32(h), rl.ColorAlpha(rl.Black, o.dim))
	band := int32(o.band)
	clear := rl.ColorAlpha(rl.Black, 0)
	rl.DrawRectangleGradientV(0, 0, int32(w), band, rl.ColorAlpha(rl.Black, o.top), clear)
	rl.DrawRectangleGradientV(0, int32(h)-band, int32(w), band, clear, rl.ColorAlpha(rl.Black, o.bottom))
}

func (u *ui) drawBackground(w, h, dt float32) {
	u.advanceBgFade(dt)
	b := &u.bgTex
	if b.prev.ID != 0 {
		drawCover(b.prev, w, h, 1-b.fade)
	}
	if b.cur.ID != 0 {
		drawCover(b.cur, w, h, b.fade)
	}
	u.drawBgOverlays(w, h)
}

// overlayParams describes the black layers drawn over the image: a uniform
// dim plus gradient bands at the top and bottom. The contrast check uses the
// same values, so its estimate matches what is on screen.
type overlayParams struct {
	dim, top, bottom, band, h float32
}

func (u *ui) overlays(h float32) overlayParams {
	b := &u.bgTex
	shown := b.fade
	if b.cur.ID == 0 {
		shown = 1 - b.fade
	}
	return overlayParams{
		// Lighter while idle so the image itself can be enjoyed; the top band
		// stays to keep the title and clock readable.
		dim:    u.cfg.Backgrounds.Dim * shown * (0.3 + 0.7*u.bodyAlpha),
		top:    0.55 * shown,
		bottom: 0.55 * shown * (0.4 + 0.6*u.bodyAlpha),
		band:   h * 0.28,
		h:      h,
	}
}

// keep is the fraction of the image's sRGB value left after the overlays at
// screen row y.
func (o overlayParams) keep(y float32) float64 {
	k := float64(1 - o.dim)
	if y < o.band {
		k *= float64(1 - o.top*(1-y/o.band))
	}
	if y > o.h-o.band {
		k *= float64(1 - o.bottom*((y-(o.h-o.band))/o.band))
	}
	return k
}

// backdropFor returns a conservative (brightest) estimate of the colour behind
// a screen rectangle: the image under it after overlays, or the plain UI
// background when no image is shown.
func coverScale(tw, th, w, h float32) float32 {
	k := w / tw
	if th*k < h {
		return h / th
	}
	return k
}

func gridSample(g *contrast.LumGrid, w, h, keep, x0, y0, x1, y1 float32) contrast.RGB {
	tw, th := float32(g.W), float32(g.H)
	k := coverScale(tw, th, w, h)
	ox, oy := (tw-w/k)/2, (th-h/k)/2
	return g.Brightest(float64(ox+x0/k), float64(oy+y0/k), float64(ox+x1/k), float64(oy+y1/k)).Scale(float64(keep))
}

func brighterGrid(out contrast.RGB, g *contrast.LumGrid, w, h, keep, x0, y0, x1, y1 float32) contrast.RGB {
	if g == nil {
		return out
	}
	c := gridSample(g, w, h, keep, x0, y0, x1, y1)
	if contrast.RelativeLuminance(c) > contrast.RelativeLuminance(out) {
		return c
	}
	return out
}

func (u *ui) brightestBackdrop(out contrast.RGB, x0, y0, x1, y1 float32) contrast.RGB {
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	o := u.overlays(h)
	keep := float32(math.Max(o.keep(y0), math.Max(o.keep(y1), o.keep((y0+y1)/2))))
	for _, g := range []*contrast.LumGrid{u.bgTex.curGrid, u.bgTex.prevGrid} {
		out = brighterGrid(out, g, w, h, keep, x0, y0, x1, y1)
	}
	return out
}

func (u *ui) backdropFor(x0, y0, x1, y1 float32) contrast.RGB {
	// Plain background, including the accent glow at the top.
	out := contrast.RGBOf(u.accent).Over(contrast.RGBOf(u.bg), 0.16)
	out = u.brightestBackdrop(out, x0, y0, x1, y1)
	if u.surface != nil {
		out = u.surface.c.Over(out, u.surface.a)
	}
	return out
}

// drawCredit draws the image title and attribution bottom right, each
// with its own setting. The title sits above the credit when both show.
func (u *ui) drawCredit(w, h, s, margin float32) {
	if u.bgTex.cur.ID == 0 {
		return
	}
	y := h - 36*s
	if u.showCredit() {
		u.drawCornerLine(u.bgTex.credit, w, y, margin, 22*s, 0.45)
		y -= 34 * s
	}
	if u.showImageTitle() {
		u.drawCornerLine(u.bgTex.title, w, y-6*s, margin, 28*s, 0.75)
	}
}

func (u *ui) showCredit() bool {
	return u.bgTex.credit != "" && !u.settings.HideAttribution
}

func (u *ui) showImageTitle() bool {
	return u.bgTex.title != "" && u.settings.ShowImageTitle
}

// drawCornerLine right-aligns one line, kept to the right of the hints as
// some credits and titles are long.
func (u *ui) drawCornerLine(line string, w, y, margin, size, alpha float32) {
	line = u.fitText(line, size, w*0.5)
	m := u.measure(line, size)
	u.text(line, w-margin-m.X, y, size, rl.ColorAlpha(rl.RayWhite, alpha*u.bgTex.fade))
}

// drawCover scales a texture to fill the screen, cropping the overflow.
func drawCover(t rl.Texture2D, w, h, alpha float32) {
	tw, th := float32(t.Width), float32(t.Height)
	k := coverScale(tw, th, w, h)
	sw, sh := w/k, h/k
	src := rl.Rectangle{X: (tw - sw) / 2, Y: (th - sh) / 2, Width: sw, Height: sh}
	rl.DrawTexturePro(t, src, rl.Rectangle{Width: w, Height: h}, rl.Vector2{}, 0, rl.ColorAlpha(rl.White, alpha))
}

// textureFromRGBA uploads pixels straight from Go memory. raylib's
// NewImageFromImage makes a cgo call per pixel, far too slow for photos.
func textureFromRGBA(img *image.RGBA) rl.Texture2D {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 || img.Stride != 4*b.Dx() {
		return rl.Texture2D{}
	}

	var pin runtime.Pinner
	pin.Pin(&img.Pix[0])
	defer pin.Unpin()

	ri := rl.Image{
		Data:    unsafe.Pointer(&img.Pix[0]),
		Width:   int32(b.Dx()),
		Height:  int32(b.Dy()),
		Mipmaps: 1,
		Format:  rl.UncompressedR8g8b8a8,
	}
	t := rl.LoadTextureFromImage(&ri)
	rl.SetTextureFilter(t, rl.FilterBilinear)
	return t
}

func (u *ui) unloadBackground() {
	for _, t := range []rl.Texture2D{u.bgTex.cur, u.bgTex.prev} {
		if t.ID != 0 {
			rl.UnloadTexture(t)
		}
	}
}
