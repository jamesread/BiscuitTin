package rss

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
)

func TestRefreshMergesAndCaches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			_, _ = w.Write([]byte(`<rss><channel><title>Feed A</title>
				<item><title>Older</title><pubDate>Mon, 01 Jan 2006 00:00:00 GMT</pubDate></item>
				<item><title>Newer</title><pubDate>Tue, 02 Jan 2006 00:00:00 GMT</pubDate></item>
			</channel></rss>`))
		case "/b":
			_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom">
				<title>Feed B</title>
				<entry><title>Middle</title><updated>2006-01-01T12:00:00Z</updated></entry>
			</feed>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{
		dir:    t.TempDir(),
		client: srv.Client(),
		feeds: []config.RSSFeed{
			{Name: "Named A", URL: srv.URL + "/a"},
			{URL: srv.URL + "/b"},
		},
	}
	f.refresh()

	items := f.Items()
	if len(items) != 3 {
		t.Fatalf("items = %d; want 3", len(items))
	}
	if items[0].Title != "Newer" || items[1].Title != "Middle" || items[2].Title != "Older" {
		t.Fatalf("order = %q, %q, %q", items[0].Title, items[1].Title, items[2].Title)
	}
	if items[0].Feed != "Named A" {
		t.Fatalf("configured name should win: %q", items[0].Feed)
	}
	if items[1].Feed != "Feed B" {
		t.Fatalf("channel title = %q", items[1].Feed)
	}
	if matches, _ := filepath.Glob(filepath.Join(f.dir, "*.json")); len(matches) != 2 {
		t.Fatalf("cache files = %v", matches)
	}
}

func TestRefreshUsesCacheWhenFetchFails(t *testing.T) {
	dir := t.TempDir()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel><title>Cached</title>
			<item><title>From cache</title><pubDate>Mon, 02 Jan 2006 00:00:00 GMT</pubDate></item>
		</channel></rss>`))
	}))
	url := good.URL
	f := &Fetcher{
		dir:    dir,
		client: good.Client(),
		feeds:  []config.RSSFeed{{URL: url}},
	}
	f.refresh()
	good.Close()

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(down.Close)
	f.client = down.Client()
	f.feeds[0].URL = down.URL
	// Cache is keyed by URL, so copy the saved cache to the new URL's name.
	old := f.cachePath(config.RSSFeed{URL: url})
	f.feeds[0].URL = down.URL
	if err := os.Rename(old, f.cachePath(f.feeds[0])); err != nil {
		t.Fatal(err)
	}

	f.refresh()
	items := f.Items()
	if len(items) != 1 || items[0].Title != "From cache" {
		t.Fatalf("items = %+v", items)
	}
}

func TestConfiguredFeedsRejectsNonHTTP(t *testing.T) {
	got := configuredFeeds([]config.RSSFeed{
		{URL: "file:///etc/passwd"},
		{URL: ""},
		{URL: "https://example.com/feed.xml"},
	})
	if len(got) != 1 || got[0].URL != "https://example.com/feed.xml" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadCachesOnStart(t *testing.T) {
	dir := t.TempDir()
	feed := config.RSSFeed{URL: "https://example.com/feed.xml"}
	f := &Fetcher{dir: dir, feeds: []config.RSSFeed{feed}}
	if err := f.writeCache(feed, []Item{{
		Feed: "X", Title: "Cached", Published: time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatal(err)
	}
	f.loadCaches()
	items := f.Items()
	if len(items) != 1 || items[0].Title != "Cached" {
		t.Fatalf("items = %+v", items)
	}
}

func TestNilFetcher(t *testing.T) {
	var f *Fetcher
	if f.Items() != nil {
		t.Fatal("nil Items should be nil")
	}
	if f.Status() != "Off" {
		t.Fatalf("status = %q", f.Status())
	}
}

func TestLoadItemsAndStatic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	raw := `{"items":[{"feed":"Gazette","title":"Hello","summary":"A note.","link":"https://news.example.com/a"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := LoadItems(path)
	if err != nil || len(items) != 1 || items[0].Title != "Hello" {
		t.Fatalf("LoadItems = %+v, %v", items, err)
	}
	f := Static(items)
	if got := f.Items(); len(got) != 1 || got[0].Feed != "Gazette" {
		t.Fatalf("Static items = %+v", got)
	}
}
