package app

import (
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/rss"
)

func TestPickUnseenRSS(t *testing.T) {
	items := []rss.Item{
		{Title: "A", Link: "https://a"},
		{Title: "B", Link: "https://b"},
		{Title: "C", Link: "https://c"},
	}
	seen := map[string]bool{"https://a": true}
	got, ok := pickUnseenRSS(items, seen)
	if !ok || got.Link != "https://b" {
		t.Fatalf("next = %+v ok=%v", got, ok)
	}
	seen["https://b"] = true
	seen["https://c"] = true
	if _, ok := pickUnseenRSS(items, seen); ok {
		t.Fatal("should stop once every article has been shown")
	}
}

func TestEaseInOut(t *testing.T) {
	if easeInOut(0) != 0 || easeInOut(1) != 1 {
		t.Fatal(easeInOut(0), easeInOut(1))
	}
	if mid := easeInOut(0.5); mid != 0.5 {
		t.Fatalf("mid = %v", mid)
	}
}

func TestRSSSourceLine(t *testing.T) {
	now := time.Date(2026, 10, 24, 21, 0, 0, 0, time.UTC)
	item := rss.Item{Feed: "BBC News", Published: now.Add(-2 * time.Hour)}
	if got := rssSourceLine(item, now); got != "BBC News  ·  2 hours ago" {
		t.Fatalf("got %q", got)
	}
	if got := rssSourceLine(rss.Item{Feed: "BBC News"}, now); got != "BBC News" {
		t.Fatalf("no date = %q", got)
	}
	if got := rssSourceLine(rss.Item{Published: now.Add(-time.Minute)}, now); got != "1 minute ago" {
		t.Fatalf("no feed = %q", got)
	}
}
