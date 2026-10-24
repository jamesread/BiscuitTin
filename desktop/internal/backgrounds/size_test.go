package backgrounds

import (
	"slices"
	"testing"
)

func TestRequestSizesStartsWithViewport(t *testing.T) {
	got := requestSizes(Size{W: 2160, H: 3840})
	if got[0] != (Size{W: 2160, H: 3840}) {
		t.Fatalf("first = %v; want the viewport", got[0])
	}
	if !slices.Contains(got, Size{W: 1080, H: 1920}) || !slices.Contains(got, Size{W: 1440, H: 2560}) {
		t.Fatalf("portrait fallbacks missing: %v", got)
	}
	if slices.Contains(got, Size{W: 1920, H: 1080}) {
		t.Fatalf("landscape standard should not follow a portrait viewport: %v", got)
	}
}

func TestRequestSizesLandscape(t *testing.T) {
	got := requestSizes(Size{W: 1920, H: 1080})
	if got[0] != (Size{W: 1920, H: 1080}) {
		t.Fatalf("first = %v", got[0])
	}
	if slices.Contains(got[1:], Size{W: 1920, H: 1080}) {
		t.Fatalf("viewport size should not be repeated: %v", got)
	}
	if !slices.Contains(got, Size{W: 2560, H: 1440}) {
		t.Fatalf("landscape fallbacks missing: %v", got)
	}
}

func TestRequestSizesZeroUses1080p(t *testing.T) {
	got := requestSizes(Size{})
	if got[0] != (Size{W: 1920, H: 1080}) {
		t.Fatalf("default first = %v", got[0])
	}
}

func TestPicsumOrientedPrefersMatching(t *testing.T) {
	photos := []picsumPhoto{
		{ID: "land", Width: 4000, Height: 2000},
		{ID: "port", Width: 2000, Height: 4000},
	}
	got := picsumOriented(photos, Size{W: 2160, H: 3840})
	if len(got) != 1 || got[0].ID != "port" {
		t.Fatalf("portrait viewport: %v", got)
	}
	got = picsumOriented(photos, Size{W: 1920, H: 1080})
	if len(got) != 1 || got[0].ID != "land" {
		t.Fatalf("landscape viewport: %v", got)
	}
}

func TestPicsumOrientedFallsBackToAny(t *testing.T) {
	photos := []picsumPhoto{{ID: "land", Width: 4000, Height: 2000}}
	got := picsumOriented(photos, Size{W: 1080, H: 1920})
	if len(got) != 1 || got[0].ID != "land" {
		t.Fatalf("should use the only photos on the page: %v", got)
	}
}
