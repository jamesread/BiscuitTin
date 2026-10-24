// Package config loads the hand-written biscuittin.yaml config file.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamesread/biscuittin/internal/fsutil"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultYAML []byte

// Config is the top level biscuittin.yaml document.
type Config struct {
	Title      string `yaml:"title"`
	Fullscreen *bool  `yaml:"fullscreen"`
	Font       string `yaml:"font"`
	Columns    int    `yaml:"columns"`
	Background string `yaml:"background"`
	Accent     string `yaml:"accent"`

	// TileColor fills every tile. It is darkened automatically if needed so
	// the white icons and letters on it meet WCAG AA contrast.
	TileColor string `yaml:"tileColor"`
	LogDir    string `yaml:"logDir"`

	// CacheDir holds downloaded and rendered icons. Default ~/.cache/biscuittin.
	CacheDir string `yaml:"cacheDir"`

	// IconProviders are searched in order for entries without an icon file.
	// Default [thesvg]; an empty list disables icon lookups.
	IconProviders []string `yaml:"iconProviders"`

	Backgrounds BackgroundConfig `yaml:"backgrounds"`

	// RSS feeds are shown as article summaries on the idle home screen.
	RSS RSSConfig `yaml:"rss"`

	// OliveTin enables More > OliveTin actions when url is set.
	OliveTin OliveTinConfig `yaml:"olivetin"`

	// WebControl configures the web remote, switched on from Settings.
	WebControl WebControlConfig `yaml:"webControl"`

	// HomeAssistant enables More > Home Assistant automations when url is set.
	HomeAssistant HomeAssistantConfig `yaml:"homeassistant"`

	// IdleFadeSeconds fades the tiles and menus out after this many seconds
	// without input, while a background image or RSS headlines can be shown.
	// 0 disables. Default 20.
	IdleFadeSeconds *float64 `yaml:"idleFadeSeconds"`

	// HideOnLaunch unmaps the launcher window while a launched app is in front.
	HideOnLaunch *bool `yaml:"hideOnLaunch"`

	// RestoreHoldSeconds is how long the gamepad Guide button must be held to
	// bring a hidden launcher back.
	RestoreHoldSeconds float64 `yaml:"restoreHoldSeconds"`

	Entries []Entry `yaml:"entries"`
}

// BackgroundConfig sets defaults for background images. Enabled and Interval
// can be changed from the More menu; those choices are saved as settings and
// take precedence.
type BackgroundConfig struct {
	Enabled *bool `yaml:"enabled"`
	// Providers are used in rotation. Default [wikimedia, picsum].
	Providers []string `yaml:"providers"`
	// Dir holds your own images for the local provider.
	Dir string `yaml:"dir"`
	// Interval is seconds between images, 60 to 86400. Default 60.
	Interval int `yaml:"interval"`
	// Dim darkens images so text stays readable, 0 to 1. Default 0.45.
	Dim float32 `yaml:"dim"`
}

// Entry is one launchable tile.
type Entry struct {
	Title    string            `yaml:"title"`
	Subtitle string            `yaml:"subtitle"`
	Command  []string          `yaml:"command"`
	Workdir  string            `yaml:"workdir"`
	Env      map[string]string `yaml:"env"`
	Icon     string            `yaml:"icon"`

	// IconQuery is the provider search term. Default: the entry title.
	IconQuery string `yaml:"iconQuery"`
	// IconStyle is "mono" (white glyph, the default) or "color" (brand colours).
	IconStyle string `yaml:"iconStyle"`

	// RestoreOnExit shows the launcher again when the launched process exits.
	// Disable for commands that hand off to an already-running instance and
	// exit immediately (e.g. steam when the client is already up).
	RestoreOnExit *bool `yaml:"restoreOnExit"`
}

var defaultFonts = []string{
	"/usr/share/fonts/google-noto/NotoSans-Bold.ttf",
	"/usr/share/fonts/google-noto-vf/NotoSans[wght].ttf",
	"/usr/share/fonts/dejavu-sans-fonts/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/app/share/fonts/biscuittin/NotoSans-Bold.ttf", // bundled in the Flatpak
}

func BoolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "biscuittin.yaml"
	}
	return filepath.Join(dir, "biscuittin", "config.yaml")
}

// Load reads the YAML config at path. If the file does not exist, a default
// is written there first and startup continues from that file.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := writeDefault(path); err != nil {
			return nil, fmt.Errorf("write default config %s: %w", path, err)
		}
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	ApplyDefaults(cfg)

	if len(cfg.Entries) == 0 {
		return nil, fmt.Errorf("%s: no entries defined", path)
	}

	for i, e := range cfg.Entries {
		if len(e.Command) == 0 {
			return nil, fmt.Errorf("%s: entry %d (%q) has no command", path, i, e.Title)
		}
	}

	return cfg, nil
}

func writeDefault(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, defaultYAML); err != nil {
		return err
	}
	zap.L().Named("config").Info("wrote default", zap.String("path", path))
	return nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.Title == "" {
		cfg.Title = "BiscuitTin on {hostname}"
	}
	cfg.Title = strings.ReplaceAll(cfg.Title, "{hostname}", shortHostname())
	if cfg.TileColor == "" {
		cfg.TileColor = "#3d5a80"
	}
	if cfg.Background == "" {
		cfg.Background = "#14161f"
	}
	if cfg.Accent == "" {
		cfg.Accent = "#f2b134"
	}
	if cfg.RestoreHoldSeconds <= 0 {
		cfg.RestoreHoldSeconds = 1.0
	}
	if cfg.CacheDir == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			cfg.CacheDir = filepath.Join(dir, "biscuittin")
		} else {
			cfg.CacheDir = filepath.Join(os.TempDir(), "biscuittin")
		}
	}
	cfg.CacheDir = ExpandPath(cfg.CacheDir)
	cfg.Backgrounds.Dir = ExpandPath(cfg.Backgrounds.Dir)
	if cfg.LogDir == "" {
		cfg.LogDir = cfg.CacheDir
	}
	cfg.LogDir = ExpandPath(cfg.LogDir)
	if cfg.IconProviders == nil {
		cfg.IconProviders = []string{"thesvg"}
	}
	if cfg.Backgrounds.Providers == nil {
		cfg.Backgrounds.Providers = []string{"wikimedia", "picsum"}
	}
	cfg.Backgrounds.Interval = ClampInterval(cfg.Backgrounds.Interval)
	if cfg.Backgrounds.Dim <= 0 || cfg.Backgrounds.Dim > 1 {
		cfg.Backgrounds.Dim = 0.45
	}
	if cfg.RSS.Interval <= 0 {
		cfg.RSS.Interval = 15 * 60
	}
	cfg.RSS.Interval = ClampInterval(cfg.RSS.Interval)
	if cfg.RSS.RotateSeconds <= 0 {
		cfg.RSS.RotateSeconds = 60
	}
	cfg.RSS.RotateSeconds = ClampInterval(cfg.RSS.RotateSeconds)
	cfg.RSS.Feeds = cleanRSSFeeds(cfg.RSS.Feeds)

	if cfg.Font == "" {
		for _, f := range defaultFonts {
			if _, err := os.Stat(f); err == nil {
				cfg.Font = f
				break
			}
		}
	}

	for i := range cfg.Entries {
		cfg.Entries[i].Icon = ExpandPath(cfg.Entries[i].Icon)
	}
}

// shortHostname is the host name up to the first dot, e.g. "queen".
func shortHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "localhost"
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

func ExpandPath(p string) string {
	if p == "" {
		return p
	}
	p = os.ExpandEnv(p)
	if len(p) > 1 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	return p
}

// OliveTinConfig points BiscuitTin at an OliveTin server whose actions can be
// pinned as tiles from More > OliveTin actions.
type OliveTinConfig struct {
	URL string `yaml:"url"`
}

// HomeAssistantConfig enables More > Home Assistant automations. The token is
// a long-lived access token (HA profile > Security); prefer tokenFile so the
// secret stays out of the config file.
type HomeAssistantConfig struct {
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	TokenFile string `yaml:"tokenFile"`
}

// WebControlConfig configures the web remote (More > Settings > Web control).
type WebControlConfig struct {
	Port int `yaml:"port"`
}

// RSSConfig lists feeds whose article summaries appear on the idle home screen.
type RSSConfig struct {
	Feeds []RSSFeed `yaml:"feeds"`
	// Interval is seconds between feed refreshes, 60 to 86400. Default 900.
	Interval int `yaml:"interval"`
	// RotateSeconds is how long each article stays on screen, 60 to 86400.
	// Default 60. The on-screen setting takes precedence.
	RotateSeconds int `yaml:"rotateSeconds"`
}

// RSSFeed is one RSS or Atom URL. A feed may be written as a URL string or as
// {url, name}.
type RSSFeed struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func (f *RSSFeed) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		f.URL = strings.TrimSpace(value.Value)
		return nil
	}
	type raw RSSFeed
	var r raw
	if err := value.Decode(&r); err != nil {
		return err
	}
	*f = RSSFeed(r)
	return nil
}

func cleanRSSFeeds(in []RSSFeed) []RSSFeed {
	var out []RSSFeed
	for _, f := range in {
		f.URL = strings.TrimSpace(f.URL)
		f.Name = strings.TrimSpace(f.Name)
		if f.URL == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Background image intervals, in seconds.
const (
	minBackgroundInterval = 60
	maxBackgroundInterval = 24 * 60 * 60
)

// ClampInterval keeps a background interval within the allowed range.
func ClampInterval(seconds int) int {
	if seconds < minBackgroundInterval {
		return minBackgroundInterval
	}
	if seconds > maxBackgroundInterval {
		return maxBackgroundInterval
	}
	return seconds
}
