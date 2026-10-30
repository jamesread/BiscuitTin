package backgrounds

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRecentKeepsFiveNewest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "src.png")
	writeTestPNG(t, src)

	for i := range 7 {
		if err := saveRecent(dir, src, fmt.Sprintf("credit-%d", i)); err != nil {
			t.Fatal(err)
		}
		// Distinct mtimes so prune order is deterministic.
		files := cachedImages(dir)
		if len(files) == 0 {
			t.Fatal("save wrote nothing")
		}
		sortByModTimeNewest(files)
		if err := os.Chtimes(files[0], time.Unix(int64(100+i), 0), time.Unix(int64(100+i), 0)); err != nil {
			t.Fatal(err)
		}
	}

	left := cachedImages(dir)
	if len(left) != recentKeep {
		t.Fatalf("kept %d files; want %d", len(left), recentKeep)
	}
	for _, p := range left {
		if _, err := os.Stat(p + ".credit"); err != nil {
			t.Errorf("missing credit for %s: %v", p, err)
		}
	}
}

func TestLoadRecentReturnsNewestAndPrunes(t *testing.T) {
	cache := t.TempDir()
	dir := recentDir(cache)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 7 {
		p := filepath.Join(dir, fmt.Sprintf("%d.png", i))
		writeTestPNG(t, p)
		if err := os.WriteFile(p+".credit", fmt.Appendf(nil, "pic-%d\n", i), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, time.Unix(int64(i), 0), time.Unix(int64(i), 0)); err != nil {
			t.Fatal(err)
		}
	}

	img, ok := LoadRecent(cache)
	if !ok {
		t.Fatal("expected a recent image")
	}
	if img.Credit != "pic-6" {
		t.Fatalf("credit = %q; want pic-6 (newest)", img.Credit)
	}
	if img.Pix == nil || img.Grid == nil {
		t.Fatal("decoded image missing pixels or contrast grid")
	}
	if n := len(cachedImages(dir)); n != recentKeep {
		t.Fatalf("after load, %d files; want %d", n, recentKeep)
	}
	if _, err := os.Stat(filepath.Join(dir, "0.png")); !os.IsNotExist(err) {
		t.Fatal("oldest file should have been pruned")
	}
}

func TestLoadRecentEmpty(t *testing.T) {
	if _, ok := LoadRecent(t.TempDir()); ok {
		t.Fatal("empty cache should not load a recent image")
	}
}

func TestLoadRecentSkipsCorrupt(t *testing.T) {
	cache := t.TempDir()
	dir := recentDir(cache)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(bad, time.Unix(2, 0), time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.png")
	writeTestPNG(t, good)
	if err := os.WriteFile(good+".credit", []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(good, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}

	img, ok := LoadRecent(cache)
	if !ok || img.Credit != "ok" {
		t.Fatalf("ok=%v credit=%q; want the older valid image", ok, img.Credit)
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("corrupt recent image should be removed")
	}
}

func TestFetchSavesRecent(t *testing.T) {
	cache := t.TempDir()
	src := t.TempDir()
	writeTestPNG(t, filepath.Join(src, "a.png"))
	if err := os.WriteFile(filepath.Join(src, "a.png.credit"), []byte("Author  /  CC0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := NewFetcher(&config.Config{
		CacheDir: cache,
		Backgrounds: config.BackgroundConfig{
			Providers: []string{"local"},
			Dir:       src,
		},
	})
	f.Request(1920, 1080)
	img := <-f.Out
	if img.Err != nil {
		t.Fatal(img.Err)
	}
	if img.Credit != "Author  /  CC0" {
		t.Fatalf("credit = %q", img.Credit)
	}

	files := cachedImages(recentDir(cache))
	if len(files) != 1 {
		t.Fatalf("recent files = %v", files)
	}
	if readCredit(files[0]) != "Author  /  CC0" {
		t.Fatalf("recent credit = %q", readCredit(files[0]))
	}

	loaded, ok := LoadRecent(cache)
	if !ok || loaded.Credit != "Author  /  CC0" {
		t.Fatalf("LoadRecent after fetch: ok=%v credit=%q", ok, loaded.Credit)
	}
}

func TestSaveRecentSkipsCopyFromRecentDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "already.png")
	writeTestPNG(t, src)
	if err := saveRecent(dir, src, "x"); err != nil {
		t.Fatal(err)
	}
	if n := len(cachedImages(dir)); n != 1 {
		t.Fatalf("should not duplicate a file already in recent: %d", n)
	}
}

func TestNewFetcherPrunesRecentOnStart(t *testing.T) {
	cache := t.TempDir()
	dir := recentDir(cache)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 7 {
		p := filepath.Join(dir, fmt.Sprintf("%d.png", i))
		writeTestPNG(t, p)
		if err := os.Chtimes(p, time.Unix(int64(i), 0), time.Unix(int64(i), 0)); err != nil {
			t.Fatal(err)
		}
	}

	_ = NewFetcher(&config.Config{CacheDir: cache})
	if n := len(cachedImages(dir)); n != recentKeep {
		t.Fatalf("startup prune left %d; want %d", n, recentKeep)
	}
	if _, err := os.Stat(filepath.Join(dir, "0.png")); !os.IsNotExist(err) {
		t.Fatal("oldest leftover should be deleted on startup")
	}
}
