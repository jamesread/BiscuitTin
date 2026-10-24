package icons

import "testing"

func TestTheSVGMatchPrefersSlug(t *testing.T) {
	p := &theSVGProvider{icons: []theSVGIcon{
		{Slug: "steam-badge", Title: "Steam"},
		{Slug: "steam", Title: "Steam"},
		{Slug: "retroarch", Title: "RetroArch"},
		{Slug: "nintendo-switch", Title: "Switch", Aliases: []string{"NX"}},
	}}

	cases := map[string]string{
		"Steam":      "steam",
		"RetroArch":  "retroarch",
		"retro-arch": "retroarch",
		"nx":         "nintendo-switch",
	}
	for q, want := range cases {
		got, ok := p.match(q)
		if !ok || got.Slug != want {
			t.Errorf("match(%q) = %q, %v; want %q", q, got.Slug, ok, want)
		}
	}

	if got, ok := p.match("Ste"); ok {
		t.Errorf("match(\"Ste\") should not partially match, got %q", got.Slug)
	}
}

// Iconify serves width="1em" height="1em"; this outline icon used to render
// as a solid square.
func TestRasterizeIconifyOutlineIcon(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 24 24"><g fill="none" stroke="#000000" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"><path d="M14 4h-4C6.229 4 4.343 4 3.172 5.172S2 8.229 2 12s0 5.657 1.172 6.828S6.229 20 10 20h4c3.771 0 5.657 0 6.828-1.172S22 15.771 22 12s0-5.657-1.172-6.828S17.771 4 14 4"/><path d="m7 9l1.84 1.586c.773.666 1.16 1 1.16 1.414s-.387.748-1.16 1.414L7 15m6 1h4"/></g></svg>`
	img, err := RasterizeSVG([]byte(svg), 128)
	if err != nil {
		t.Fatal(err)
	}
	covered := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 128 {
			covered++
		}
	}
	// A 1.5-unit stroke in a 24-unit viewBox is 8px at 128px: roughly 15-25%
	// of the image. Below that the stroke width wasn't scaled (a hairline);
	// near 100% the icon was filled solid.
	frac := float64(covered) / float64(128*128)
	if frac < 0.1 || frac > 0.4 {
		t.Fatalf("outline icon coverage %.2f; want a scaled stroke, not a hairline or solid", frac)
	}
	if c := img.RGBAAt(64, 64); c.A != 0 {
		t.Errorf("centre of the outline should be empty, got %v", c)
	}
}

func TestRasterizeAndTint(t *testing.T) {
	svg := []byte(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path fill="#ff0000" d="M0 0h24v12H0z"/></svg>`)
	img, err := RasterizeSVG(svg, 64)
	if err != nil {
		t.Fatal(err)
	}
	top, bottom := img.RGBAAt(32, 8), img.RGBAAt(32, 56)
	if top.A != 255 || top.R != 255 || top.G != 0 {
		t.Fatalf("top half should be opaque red, got %v", top)
	}
	if bottom.A != 0 {
		t.Fatalf("bottom half should be transparent, got %v", bottom)
	}

	TintWhite(img)
	if c := img.RGBAAt(32, 8); c.R != 255 || c.G != 255 || c.B != 255 || c.A != 255 {
		t.Fatalf("tinted pixel should be white, got %v", c)
	}
}
