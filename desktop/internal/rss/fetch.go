// Package rss fetches RSS and Atom feeds and keeps a list of article
// summaries for the idle home screen.
package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/fsutil"
	"go.uber.org/zap"
)

const (
	userAgent     = "biscuittin/0.1 (https://github.com/jamesread/biscuittin)"
	maxBody       = 4 << 20
	maxPerFeed    = 20
	maxItems      = 60
	clientTimeout = 30 * time.Second
)

// Fetcher refreshes configured feeds on a background goroutine so downloads
// never stall rendering. Items() is the current merged list, newest first.
type Fetcher struct {
	client   *http.Client
	dir      string
	feeds    []config.RSSFeed
	items    []Item
	interval time.Duration
	mu       sync.RWMutex
}

func NewFetcher(cfg *config.Config) *Fetcher {
	feeds := configuredFeeds(cfg.RSS.Feeds)
	if len(feeds) == 0 {
		return nil
	}
	f := &Fetcher{
		feeds:    feeds,
		dir:      filepath.Join(cfg.CacheDir, "rss"),
		client:   &http.Client{Timeout: clientTimeout},
		interval: time.Duration(cfg.RSS.Interval) * time.Second,
	}
	if f.interval < time.Minute {
		f.interval = 15 * time.Minute
	}
	f.loadCaches()
	go f.run()
	return f
}

func validFeedURL(u string) (*url.URL, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "http" {
		return parsed, nil
	}
	if parsed.Scheme == "https" {
		return parsed, nil
	}
	return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
}

func cleanedFeed(feed config.RSSFeed) (config.RSSFeed, bool) {
	u := strings.TrimSpace(feed.URL)
	if u == "" {
		return feed, false
	}
	if _, err := validFeedURL(u); err != nil {
		lg().Warn("skipping RSS feed", zap.String("url", u), zap.Error(err))
		return feed, false
	}
	feed.URL = u
	feed.Name = strings.TrimSpace(feed.Name)
	return feed, true
}

func configuredFeeds(in []config.RSSFeed) []config.RSSFeed {
	var out []config.RSSFeed
	for _, feed := range in {
		cleaned, ok := cleanedFeed(feed)
		if !ok {
			continue
		}
		out = append(out, cleaned)
	}
	return out
}

func (f *Fetcher) run() {
	f.refresh()
	t := time.NewTicker(f.interval)
	defer t.Stop()
	for range t.C {
		f.refresh()
	}
}

func (f *Fetcher) Items() []Item {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]Item, len(f.items))
	copy(out, f.items)
	return out
}

func (f *Fetcher) Status() string {
	if f == nil {
		return "Off"
	}
	n := len(f.Items())
	if n == 0 {
		return fmt.Sprintf("%d feeds, no articles yet", len(f.feeds))
	}
	return fmt.Sprintf("%d feeds, %d articles", len(f.feeds), n)
}

func (f *Fetcher) refresh() {
	var merged []Item
	for _, feed := range f.feeds {
		items, err := f.fetchFeed(feed)
		if err != nil {
			lg().Warn("RSS feed failed, using cache", zap.String("url", feed.URL), zap.Error(err))
			items = f.readCache(feed)
		}
		merged = append(merged, items...)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Published.After(merged[j].Published)
	})
	if len(merged) > maxItems {
		merged = merged[:maxItems]
	}
	f.mu.Lock()
	f.items = merged
	f.mu.Unlock()
}

func capFeedItems(items []Item, name string) []Item {
	if len(items) > maxPerFeed {
		items = items[:maxPerFeed]
	}
	for i := range items {
		items[i].Feed = name
	}
	return items
}

func (f *Fetcher) storeFeed(feed config.RSSFeed, items []Item) []Item {
	if err := f.writeCache(feed, items); err != nil {
		lg().Warn("could not cache RSS feed", zap.String("url", feed.URL), zap.Error(err))
	}
	return items
}

func (f *Fetcher) fetchFeed(feed config.RSSFeed) ([]Item, error) {
	raw, ctype, err := httpGet(f.client, feed.URL, maxBody)
	if err != nil {
		return nil, err
	}
	name, items, err := parseFeedTyped(raw, ctype, feed.Name)
	if err != nil {
		return nil, err
	}
	if feed.Name != "" {
		name = feed.Name
	}
	return f.storeFeed(feed, capFeedItems(items, name)), nil
}

type feedCache struct {
	Items []Item `json:"items"`
}

func (f *Fetcher) cachePath(feed config.RSSFeed) string {
	sum := sha256.Sum256([]byte(feed.URL))
	return filepath.Join(f.dir, hex.EncodeToString(sum[:8])+".json")
}

func (f *Fetcher) writeCache(feed config.RSSFeed, items []Item) error {
	if err := os.MkdirAll(f.dir, 0o750); err != nil {
		return err
	}
	raw, err := json.Marshal(feedCache{Items: items})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(f.cachePath(feed), raw)
}

func (f *Fetcher) readCache(feed config.RSSFeed) []Item {
	raw, err := os.ReadFile(f.cachePath(feed))
	if err != nil {
		return nil
	}
	doc := feedCache{}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	return doc.Items
}

func (f *Fetcher) loadCaches() {
	var merged []Item
	for _, feed := range f.feeds {
		merged = append(merged, f.readCache(feed)...)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Published.After(merged[j].Published)
	})
	if len(merged) > maxItems {
		merged = merged[:maxItems]
	}
	f.items = merged
}

func readFeedBody(rawURL string, resp *http.Response, limit int64) ([]byte, string, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", err
	}
	return raw, resp.Header.Get("Content-Type"), nil
}

func httpGet(client *http.Client, rawURL string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	return readFeedBody(rawURL, resp, limit)
}

func lg() *zap.Logger { return zap.L().Named("rss") }
