package tray

import (
	_ "embed"
	"fmt"
	"image"

	"github.com/jamesread/biscuittin/internal/icons"
)

//go:embed icon.svg
var iconSVG []byte

const iconSize = 48

type iconPixmap struct {
	Width  int32
	Height int32
	Pixels []byte
}

func itemPixmaps() ([]iconPixmap, error) {
	img, err := icons.RasterizeSVG(iconSVG, iconSize)
	if err != nil {
		return nil, fmt.Errorf("tray icon: %w", err)
	}
	return []iconPixmap{rgbaPixmap(img)}, nil
}

func rgbaPixmap(img *image.RGBA) iconPixmap {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	data := make([]byte, w*h*4)
	writeARGB(img, data, w)
	return iconPixmap{Width: int32(w), Height: int32(h), Pixels: data}
}

func writeARGB(img *image.RGBA, data []byte, w int) {
	for y := range img.Bounds().Dy() {
		writeARGBRow(img, data, y, w)
	}
}

func writeARGBRow(img *image.RGBA, data []byte, y, w int) {
	for x := range w {
		c := img.RGBAAt(x, y)
		i := (y*w + x) * 4
		data[i] = c.A
		data[i+1] = c.R
		data[i+2] = c.G
		data[i+3] = c.B
	}
}
