package backgrounds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileTitle(t *testing.T) {
	cases := map[string]string{
		"Lake_Bled-in winter.jpg": "Lake Bled in winter",
		"sunset.PNG":              "sunset",
		"  spaced__out .jpeg":     "spaced out",
	}
	for in, want := range cases {
		if got := fileTitle(in); got != want {
			t.Errorf("fileTitle(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestImageTitleSidecarThenProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Mount_Fuji.jpg")
	local := &localProvider{dir: dir}
	if got := imageTitle(local, path); got != "Mount Fuji" {
		t.Fatalf("local without .title = %q", got)
	}
	if got := imageTitle(&picsumProvider{}, path); got != "" {
		t.Fatalf("picsum has no titles, got %q", got)
	}
	if err := os.WriteFile(path+".title", []byte("Fuji at dawn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := imageTitle(local, path); got != "Fuji at dawn" {
		t.Fatalf(".title should win, got %q", got)
	}
}

func TestCommonsTitle(t *testing.T) {
	named := commonsPage{Title: "File:DSC_0042.jpg", ImageInfo: []commonsInfo{{ExtMeta: map[string]extVal{
		"ObjectName": {Value: json.RawMessage(`"<b>Lake Bled</b> in winter"`)},
	}}}}
	if got := commonsTitle(named); got != "Lake Bled in winter" {
		t.Fatalf("object name = %q", got)
	}
	if got := commonsTitle(commonsPage{Title: "File:Grand_Canyon_view.jpg"}); got != "Grand Canyon view" {
		t.Fatalf("file name = %q", got)
	}
}

func TestRecentKeepsTitle(t *testing.T) {
	cache := t.TempDir()
	src := filepath.Join(t.TempDir(), "a.png")
	writeTestPNG(t, src)
	if err := saveRecent(recentDir(cache), src, "Someone", "A title"); err != nil {
		t.Fatal(err)
	}
	img, ok := LoadRecent(cache)
	if !ok || img.Credit != "Someone" || img.Title != "A title" {
		t.Fatalf("recent = ok %v credit %q title %q", ok, img.Credit, img.Title)
	}
	removeImage(src)
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("removeImage left the image")
	}
}
