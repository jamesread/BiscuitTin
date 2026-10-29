package tray

import (
	"image"
	"image/color"
	"testing"
)

func TestBlendOverOpaque(t *testing.T) {
	dst := []byte{1, 2, 3, 0}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	blendOver(dst, img, 1, 1)
	if dst[0] != 30 || dst[1] != 20 || dst[2] != 10 {
		t.Fatalf("got %v", dst)
	}
}

func TestBlendOverSkipTransparent(t *testing.T) {
	dst := []byte{9, 8, 7, 0}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	blendOver(dst, img, 1, 1)
	if dst[0] != 9 || dst[1] != 8 || dst[2] != 7 {
		t.Fatalf("got %v", dst)
	}
}

func TestScaleRGBASameSize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if got := scaleRGBA(img, 2, 2); got != img {
		t.Fatal("same size should return the source")
	}
}
