package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
)

func TestStepInterval(t *testing.T) {
	cases := []struct{ cur, dir, want int }{
		{60, 1, 120},
		{60, -1, 60},      // clamps at the 1 minute minimum
		{86400, 1, 86400}, // clamps at the 1 day maximum
		{86400, -1, 43200},
		{90, 1, 120}, // off-step values snap to the next step
		{90, -1, 60},
	}
	for _, c := range cases {
		if got := StepInterval(c.cur, c.dir); got != c.want {
			t.Errorf("stepInterval(%d, %d) = %d; want %d", c.cur, c.dir, got, c.want)
		}
	}
}

func TestClampAndLabel(t *testing.T) {
	if config.ClampInterval(5) != 60 || config.ClampInterval(999999) != 86400 {
		t.Fatal("clampInterval should keep values within 60..86400")
	}
	labels := map[int]string{60: "1 minute", 300: "5 minutes", 3600: "1 hour", 21600: "6 hours", 86400: "1 day", 90: "90 seconds"}
	for in, want := range labels {
		if got := IntervalLabel(in); got != want {
			t.Errorf("intervalLabel(%d) = %q; want %q", in, got, want)
		}
	}
}

func TestSettingsMigratesOliveTinPins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "biscuittin", "settings.yaml")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("olivetinActions:\n  - bindingId: date\n    title: Date\n"), 0o644)

	cfg := &config.Config{}
	config.ApplyDefaults(cfg)
	s := Load(cfg)
	if len(s.PinnedActions) != 1 || s.PinnedActions[0].SourceID() != "olivetin" || s.PinnedActions[0].Key() != "act:olivetin:date" {
		t.Fatalf("legacy pins not migrated: %+v", s.PinnedActions)
	}
	if len(s.LegacyOliveTinActions) != 0 {
		t.Error("legacy field should be cleared after migration")
	}
}

// Web control has no login, so it must stay off until deliberately switched
// on in Settings.
func TestRSSIntervalDefaultsFromConfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := &config.Config{}
	cfg.RSS.RotateSeconds = 300
	config.ApplyDefaults(cfg)
	s := Load(cfg)
	if s.RSSInterval != 300 {
		t.Fatalf("RSSInterval = %d; want config value 300", s.RSSInterval)
	}
	if !s.RSSEnabled {
		t.Fatal("RSS headlines should default on")
	}
}

func TestWebControlOffByDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := &config.Config{}
	config.ApplyDefaults(cfg)
	s := Load(cfg)
	if s.WebControl || s.QRPlace != QRNone {
		t.Fatalf("fresh settings: WebControl %v, QRPlace %v; both must default to off", s.WebControl, s.QRPlace)
	}
}

func TestMigratesLegacyShowQR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "biscuittin", "settings.yaml")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("showQR: true\n"), 0o644)
	cfg := &config.Config{}
	config.ApplyDefaults(cfg)
	s := Load(cfg)
	if s.QRPlace != QRMenu {
		t.Fatalf("showQR true should become menu, got %v", s.QRPlace)
	}
}
