package backgrounds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProviderCyclesInNameOrder(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"b.png", "a.jpg", "notes.txt", "c.JPEG"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jpg.credit"), []byte("Someone  /  CC0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &localProvider{dir: dir}
	want := []string{"a.jpg", "b.png", "c.JPEG", "a.jpg"}
	for i, w := range want {
		path, credit, err := p.Next(Size{})
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) != w {
			t.Errorf("image %d = %s; want %s", i, filepath.Base(path), w)
		}
		if w == "a.jpg" && credit != "Someone  /  CC0" {
			t.Errorf("credit = %q", credit)
		}
	}
}

func TestLocalProviderEmpty(t *testing.T) {
	if _, _, err := (&localProvider{dir: t.TempDir()}).Next(Size{}); err == nil {
		t.Fatal("want an error for a directory with no images")
	}
}
