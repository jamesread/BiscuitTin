package backgrounds

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPicsumFallsBackFromExactSize(t *testing.T) {
	good := testJPEG(t)
	var requested []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		// /id/99/2160/3840.jpg
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, ".jpg"), "/")
		if len(parts) < 5 {
			http.NotFound(w, r)
			return
		}
		wth, _ := strconv.Atoi(parts[3])
		hgt, _ := strconv.Atoi(parts[4])
		if wth == 2160 && hgt == 3840 {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		if wth == 1080 && hgt == 1920 {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(good)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lists"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := `[{"id":"99","author":"Ada","width":2000,"height":4000}]`
	if err := os.WriteFile(filepath.Join(dir, "lists", "page-1.json"), []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &picsumProvider{
		dir:      dir,
		client:   srv.Client(),
		imageFmt: srv.URL + "/id/%s/%d/%d.jpg",
		pageN:    1,
	}
	path, credit, err := p.Next(Size{W: 2160, H: 3840})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "picsum-99-1080x1920.jpg") {
		t.Fatalf("path = %s; want the first standard portrait size", path)
	}
	if credit != "Ada  /  Unsplash via Lorem Picsum" {
		t.Fatalf("credit = %q", credit)
	}
	if !containsPath(requested, "/id/99/2160/3840.jpg") || !containsPath(requested, "/id/99/1080/1920.jpg") {
		t.Fatalf("requests = %v", requested)
	}
}

func TestPicsumSkipsCorruptThenFallsBack(t *testing.T) {
	good := testJPEG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		if strings.Contains(r.URL.Path, "2160/3840") {
			_, _ = w.Write([]byte("not a jpeg"))
			return
		}
		_, _ = w.Write(good)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lists"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lists", "page-1.json"), []byte(`[{"id":"7","author":"Bea","width":1000,"height":2000}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &picsumProvider{
		dir:      dir,
		client:   srv.Client(),
		imageFmt: srv.URL + "/id/%s/%d/%d.jpg",
		pageN:    1,
	}
	path, _, err := p.Next(Size{W: 2160, H: 3840})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "1080x1920") {
		t.Fatalf("path = %s", path)
	}
	if _, err := os.Stat(filepath.Join(dir, "picsum-7-2160x3840.jpg")); !os.IsNotExist(err) {
		t.Fatal("corrupt viewport-sized file should be removed")
	}
}

func TestWikimediaThumbWidthsAndURL(t *testing.T) {
	got := wikimediaThumbWidths(Size{W: 2160, H: 3840}, 5000)
	if fmt.Sprint(got) != "[2160 1920 5000]" {
		t.Fatalf("widths = %v", got)
	}

	thumb := "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/Foo.jpg/800px-Foo.jpg"
	orig := "https://upload.wikimedia.org/wikipedia/commons/a/ab/Foo.jpg"
	u, ok := wikimediaURL(orig, thumb, 5000, 2160)
	if !ok || !strings.Contains(u, "/2160px-Foo.jpg") {
		t.Fatalf("thumb url = %q ok=%v", u, ok)
	}
	u, ok = wikimediaURL(orig, thumb, 1800, 2160)
	if !ok || u != orig {
		t.Fatalf("smaller original should be used as-is: %q ok=%v", u, ok)
	}
}

func TestWikimediaOrientationOK(t *testing.T) {
	if !wikimediaOrientationOK(2000, 4000, Size{W: 2160, H: 3840}) {
		t.Fatal("portrait image should match a portrait viewport")
	}
	if wikimediaOrientationOK(4000, 2000, Size{W: 2160, H: 3840}) {
		t.Fatal("landscape image should not match a portrait viewport")
	}
	if !wikimediaOrientationOK(4000, 2000, Size{W: 1920, H: 1080}) {
		t.Fatal("landscape image should match a landscape viewport")
	}
}

func containsPath(got []string, want string) bool {
	return slices.Contains(got, want)
}
