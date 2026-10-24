package app

import "testing"

func TestLayoutScaleLandscapeMatchesHeight(t *testing.T) {
	if got := layoutScale(1920, 1080); got != 1 {
		t.Fatalf("1080p = %v", got)
	}
	if got := layoutScale(3840, 2160); got != 2 {
		t.Fatalf("4K = %v", got)
	}
	if got := layoutScale(1280, 720); got != 720.0/1080.0 {
		t.Fatalf("720p = %v", got)
	}
}

func TestLayoutScalePortraitFollowsWidth(t *testing.T) {
	s := layoutScale(2160, 3840)
	if s != 2160.0/1920.0 {
		t.Fatalf("portrait scale = %v; want width/1920 = %v", s, 2160.0/1920.0)
	}
	// A 1000px-at-1080p menu row plus margins must still fit.
	if 1000*s+2*90*s > 2160 {
		t.Fatalf("portrait scale %v would overflow a 2160-wide window", s)
	}
}

func TestLayoutScaleNarrowWindow(t *testing.T) {
	s := layoutScale(800, 1200)
	if s != 800.0/1920.0 {
		t.Fatalf("narrow = %v", s)
	}
}

func TestWrapLines(t *testing.T) {
	width := func(s string) float32 { return float32(len(s)) }

	got := wrapLines("the quick brown fox", 9, 2, width)
	if len(got) != 2 || got[0] != "the quick" || got[1] != "brown fox" {
		t.Fatalf("wrap = %#v", got)
	}

	got = wrapLines("the quick brown fox", 5, 2, width)
	if len(got) != 2 || got[0] != "the" || got[1] != "qu..." {
		t.Fatalf("ellipsis leftover = %#v", got)
	}

	got = wrapLines("supercalifragilistic", 8, 1, width)
	if len(got) != 1 || got[0] != "super..." {
		t.Fatalf("long word = %#v", got)
	}
	if wrapLines("", 10, 2, width) != nil {
		t.Fatal("empty should be nil")
	}
}

func TestColumnsFitCapsOnNarrowGrid(t *testing.T) {
	// 5 configured columns, but only ~2 tiles of 200 plus gaps fit in 500px.
	got := columnsFit(5, 8, 500, 50, 200)
	if got != 2 {
		t.Fatalf("columnsFit = %d; want 2", got)
	}
	if got := columnsFit(5, 2, 2000, 50, 200); got != 2 {
		t.Fatalf("cannot exceed tile count: %d", got)
	}
	if got := columnsFit(0, 10, 2000, 50, 200); got != 5 {
		t.Fatalf("default columns = %d", got)
	}
}
