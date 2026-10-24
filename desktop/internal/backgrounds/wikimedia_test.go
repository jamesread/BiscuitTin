package backgrounds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommonsPageUsable(t *testing.T) {
	land := commonsPage{
		Title: "File:Yorkshire Dales.jpg",
		ImageInfo: []commonsInfo{{
			Width: 4000, Height: 2000,
			ExtMeta: map[string]extVal{
				"ImageDescription": {Value: json.RawMessage(`"Sunset over the Dales"`)},
				"Categories":       {Value: json.RawMessage(`"Featured pictures of landscapes"`)},
			},
		}},
	}
	if !land.usable(Size{W: 1920, H: 1080}) {
		t.Fatal("landscape scenery should be usable")
	}

	church := commonsPage{
		Title: "File:Notre-Dame.jpg",
		ImageInfo: []commonsInfo{{
			Width: 4000, Height: 2000,
			ExtMeta: map[string]extVal{
				"ImageDescription": {Value: json.RawMessage(`"The cathedral at dusk"`)},
				"Categories":       {Value: json.RawMessage(`"Featured pictures of churches"`)},
			},
		}},
	}
	if church.usable(Size{W: 1920, H: 1080}) {
		t.Fatal("church metadata should be rejected")
	}

	png := commonsPage{
		Title:     "File:Map.svg",
		ImageInfo: []commonsInfo{{Width: 4000, Height: 2000}},
	}
	if png.usable(Size{W: 1920, H: 1080}) {
		t.Fatal("non-jpeg should be rejected")
	}

	portrait := commonsPage{
		Title:     "File:Cliff.jpg",
		ImageInfo: []commonsInfo{{Width: 2000, Height: 4000}},
	}
	if portrait.usable(Size{W: 1920, H: 1080}) {
		t.Fatal("portrait image should not match a landscape viewport")
	}
}

func TestWikimediaNextFromCategory(t *testing.T) {
	good := testJPEG(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("action") == "query" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"query":{"pages":{"1":{
				"title":"File:Dales.jpg",
				"imageinfo":[{
					"width":4000,"height":2000,
					"url":"` + srv.URL + `/orig.jpg",
					"thumburl":"` + srv.URL + `/thumb/800px-Dales.jpg",
					"extmetadata":{
						"Artist":{"value":"<b>Ada</b>"},
						"LicenseShortName":{"value":"CC BY-SA 4.0"},
						"ImageDescription":{"value":"Sunset over the Dales"},
						"Categories":{"value":"Featured pictures of landscapes"}
					}
				}]
			}}}}`))
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(good)
	}))
	t.Cleanup(srv.Close)

	p := &wikimediaProvider{
		dir:    t.TempDir(),
		api:    srv.URL,
		cats:   []string{"Category:Featured_pictures_of_landscapes"},
		client: srv.Client(),
	}

	path, credit, err := p.Next(Size{W: 1920, H: 1080})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(credit, "Ada") || !strings.Contains(credit, "CC BY-SA 4.0") {
		t.Fatalf("credit = %q", credit)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.dir, "lists")); err != nil {
		t.Fatal("category list should be cached")
	}
}

func TestWikimediaSkipsUnsafeInCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":{"pages":{"1":{
			"title":"File:Madonna.jpg",
			"imageinfo":[{
				"width":4000,"height":2000,
				"url":"https://example.invalid/madonna.jpg",
				"extmetadata":{
					"ImageDescription":{"value":"The Madonna and Child"},
					"Categories":{"value":"Featured pictures of paintings"}
				}
			}]
		}}}}`))
	}))
	t.Cleanup(srv.Close)

	p := &wikimediaProvider{
		dir:    t.TempDir(),
		api:    srv.URL,
		cats:   []string{"Category:X"},
		client: srv.Client(),
	}
	if _, _, err := p.Next(Size{W: 1920, H: 1080}); err == nil {
		t.Fatal("unsafe-only category should not yield an image")
	}
}

func TestExtValString(t *testing.T) {
	if got := (extVal{Value: json.RawMessage(`"Ada"`)}).String(); got != "Ada" {
		t.Fatalf("string = %q", got)
	}
	if got := (extVal{Value: json.RawMessage(`42`)}).String(); got != "42" {
		t.Fatalf("number = %q", got)
	}
}

func TestPurgeLegacyPOTD(t *testing.T) {
	wiki := t.TempDir()
	recents := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wiki, "feeds"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWiki := filepath.Join(wiki, "potd.jpg")
	oldRecent := filepath.Join(recents, "old.jpg")
	keepRecent := filepath.Join(recents, "keep.jpg")
	for _, p := range []string{oldWiki, oldRecent, keepRecent} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(oldRecent+".credit", []byte("Ada  /  Wikimedia Commons\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepRecent+".credit", []byte("Ada  /  Picsum\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	purgeLegacyPOTD(wiki, recents)

	if _, err := os.Stat(filepath.Join(wiki, "feeds")); !os.IsNotExist(err) {
		t.Fatal("old POTD feed cache should be removed")
	}
	if _, err := os.Stat(oldWiki); !os.IsNotExist(err) {
		t.Fatal("old Wikimedia images should be removed")
	}
	if _, err := os.Stat(oldRecent); !os.IsNotExist(err) {
		t.Fatal("recent Wikimedia stills should be removed")
	}
	if _, err := os.Stat(keepRecent); err != nil {
		t.Fatal("non-Wikimedia recents should stay")
	}

	// Second call is a no-op once feeds/ is gone.
	if err := os.WriteFile(filepath.Join(wiki, "new.jpg"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	purgeLegacyPOTD(wiki, recents)
	if _, err := os.Stat(filepath.Join(wiki, "new.jpg")); err != nil {
		t.Fatal("new cache should not be purged after migration")
	}
}

func TestSafeCacheName(t *testing.T) {
	if got := safeCacheName("Category:Featured_pictures_of_landscapes"); got != "featured_pictures_of_landscapes" {
		t.Fatalf("name = %q", got)
	}
}
