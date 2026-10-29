package tray

import (
	"image"
	"image/color"

	"github.com/jezek/xgb/xproto"
)

func (xe *xembed) redraw() {
	if xe.skipDraw() {
		return
	}
	data := xe.bgPixels()
	blendOver(data, scaleRGBA(xe.img, int(xe.w), int(xe.h)), int(xe.w), int(xe.h))
	_ = xproto.PutImageChecked(xe.x, xproto.ImageFormatZPixmap, xproto.Drawable(xe.win), xe.gc, xe.w, xe.h, 0, 0, 0, xe.depth, data).Check()
}

func (xe *xembed) skipDraw() bool {
	if xe.w == 0 {
		return true
	}
	if xe.h == 0 {
		return true
	}
	return xe.img == nil
}

func (xe *xembed) bgPixels() []byte {
	n := int(xe.w) * int(xe.h) * 4
	reply, err := xproto.GetImage(xe.x, xproto.ImageFormatZPixmap, xproto.Drawable(xe.win), 0, 0, xe.w, xe.h, 0xffffffff).Reply()
	if imageDataOK(reply, err, n) {
		return reply.Data[:n]
	}
	return make([]byte, n)
}

func imageDataOK(reply *xproto.GetImageReply, err error, n int) bool {
	if err != nil {
		return false
	}
	if reply == nil {
		return false
	}
	return len(reply.Data) >= n
}

func scaleRGBA(src *image.RGBA, w, h int) *image.RGBA {
	if src.Bounds().Dx() == w && src.Bounds().Dy() == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	writeScaled(src, dst, w, h)
	return dst
}

func writeScaled(src, dst *image.RGBA, w, h int) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	for y := range h {
		writeScaledRow(src, dst, y, w, h, sw, sh)
	}
}

func writeScaledRow(src, dst *image.RGBA, y, w, h, sw, sh int) {
	sy := y * sh / h
	for x := range w {
		dst.SetRGBA(x, y, src.RGBAAt(x*sw/w, sy))
	}
}

func blendOver(dst []byte, img *image.RGBA, w, h int) {
	for y := range h {
		blendRow(dst, img, y, w)
	}
}

func blendRow(dst []byte, img *image.RGBA, y, w int) {
	for x := range w {
		blendPixel(dst, img.RGBAAt(x, y), (y*w+x)*4)
	}
}

func blendPixel(dst []byte, c color.RGBA, i int) {
	if c.A == 0 {
		return
	}
	if c.A == 255 {
		dst[i], dst[i+1], dst[i+2] = c.B, c.G, c.R
		return
	}
	blendPartial(dst, c, i)
}

func blendPartial(dst []byte, c color.RGBA, i int) {
	a := uint32(c.A)
	ia := 255 - a
	dst[i] = byte((uint32(c.B)*a + uint32(dst[i])*ia) / 255)
	dst[i+1] = byte((uint32(c.G)*a + uint32(dst[i+1])*ia) / 255)
	dst[i+2] = byte((uint32(c.R)*a + uint32(dst[i+2])*ia) / 255)
}
