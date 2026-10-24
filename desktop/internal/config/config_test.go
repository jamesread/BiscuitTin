package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWritesDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("default not written: %v", err)
	}
	if !strings.Contains(string(raw), "entries:") {
		t.Fatal("written default has no entries section")
	}
	if len(cfg.Entries) == 0 {
		t.Fatal("default config has no entries")
	}
	for i, e := range cfg.Entries {
		if e.Title == "" || len(e.Command) == 0 {
			t.Fatalf("entry %d missing title or command: %+v", i, e)
		}
	}

	if _, err = Load(path); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatal("second load rewrote the config")
	}
}

func TestLoadDoesNotOverwriteExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	custom := []byte("title: Custom\nentries:\n  - title: Foo\n    command: [true]\n")
	if err := os.WriteFile(path, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "Custom" {
		t.Fatalf("title = %q; want Custom", cfg.Title)
	}
	if len(cfg.Entries) != 1 || cfg.Entries[0].Title != "Foo" {
		t.Fatalf("entries = %+v", cfg.Entries)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatal("existing config was rewritten")
	}
}

func TestLoadRejectsBadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(":\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadRSSFeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte(`
entries:
  - title: Foo
    command: [true]
rss:
  interval: 30
  rotateSeconds: 2
  feeds:
    - https://example.com/a.xml
    - name: Named
      url: https://example.com/b.xml
    - url: ""
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RSS.Interval != 60 {
		t.Fatalf("interval clamped to 60, got %d", cfg.RSS.Interval)
	}
	if cfg.RSS.RotateSeconds != 60 {
		t.Fatalf("rotate clamped to 60, got %d", cfg.RSS.RotateSeconds)
	}
	if len(cfg.RSS.Feeds) != 2 {
		t.Fatalf("feeds = %+v", cfg.RSS.Feeds)
	}
	if cfg.RSS.Feeds[0].URL != "https://example.com/a.xml" || cfg.RSS.Feeds[0].Name != "" {
		t.Fatalf("string feed = %+v", cfg.RSS.Feeds[0])
	}
	if cfg.RSS.Feeds[1].Name != "Named" || cfg.RSS.Feeds[1].URL != "https://example.com/b.xml" {
		t.Fatalf("map feed = %+v", cfg.RSS.Feeds[1])
	}
}

func TestLoadRejectsEmptyEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("title: Empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for no entries")
	}
}
