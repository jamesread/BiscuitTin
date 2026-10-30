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
	Fullscreen         *bool               `yaml:"fullscreen"`
	HideOnLaunch       *bool               `yaml:"hideOnLaunch"`
	KeepAwake          *bool               `yaml:"keepAwake"`
	IdleFadeSeconds    *float64            `yaml:"idleFadeSeconds"`
	HomeAssistant      HomeAssistantConfig `yaml:"homeassistant"`
	CacheDir           string              `yaml:"cacheDir"`
	Background         string              `yaml:"background"`
	TileColor          string              `yaml:"tileColor"`
	LogDir             string              `yaml:"logDir"`
	Title              string              `yaml:"title"`
	Font               string              `yaml:"font"`
	Accent             string              `yaml:"accent"`
	OliveTin           OliveTinConfig      `yaml:"olivetin"`
	IconProviders      []string            `yaml:"iconProviders"`
	Entries            []Entry             `yaml:"entries"`
	ScreenOff          ScreenOffConfig     `yaml:"screenOff"`
	Volume             VolumeConfig        `yaml:"volume"`
	CEC                CECConfig           `yaml:"cec"`
	Backgrounds        BackgroundConfig    `yaml:"backgrounds"`
	RSS                RSSConfig           `yaml:"rss"`
	WebControl         WebControlConfig    `yaml:"webControl"`
	Columns            int                 `yaml:"columns"`
	RestoreHoldSeconds float64             `yaml:"restoreHoldSeconds"`
}

// BackgroundConfig sets defaults for background images. Enabled and Interval
// can be changed from the More menu; those choices are saved as settings and
// take precedence.
type BackgroundConfig struct {
	Enabled   *bool    `yaml:"enabled"`
	Dir       string   `yaml:"dir"`
	Providers []string `yaml:"providers"`
	Interval  int      `yaml:"interval"`
	Dim       float32  `yaml:"dim"`
}

// Entry is one launchable tile.
type Entry struct {
	Env           map[string]string `yaml:"env"`
	RestoreOnExit *bool             `yaml:"restoreOnExit"`
	Title         string            `yaml:"title"`
	Subtitle      string            `yaml:"subtitle"`
	Workdir       string            `yaml:"workdir"`
	WindowClass   string            `yaml:"windowClass"`
	Icon          string            `yaml:"icon"`
	IconQuery     string            `yaml:"iconQuery"`
	IconStyle     string            `yaml:"iconStyle"`
	Command       []string          `yaml:"command"`
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
	raw, err := readOrWriteDefault(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err = yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	ApplyDefaults(cfg)
	return validateEntries(path, cfg)
}

func readOrWriteDefault(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return raw, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err = writeDefault(path); err != nil {
		return nil, fmt.Errorf("write default config %s: %w", path, err)
	}
	return os.ReadFile(path)
}

func validateEntries(path string, cfg *Config) (*Config, error) {
	if len(cfg.Entries) == 0 {
		return nil, fmt.Errorf("%s: no entries defined", path)
	}
	if err := requireCommands(path, cfg.Entries); err != nil {
		return nil, err
	}
	return cfg, nil
}

func requireCommands(path string, entries []Entry) error {
	for i, e := range entries {
		if len(e.Command) == 0 {
			return fmt.Errorf("%s: entry %d (%q) has no command", path, i, e.Title)
		}
	}
	return nil
}

func writeDefault(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, defaultYAML); err != nil {
		return err
	}
	zap.L().Named("config").Info("wrote default", zap.String("path", path))
	return nil
}

func ApplyDefaults(cfg *Config) {
	applyChromeDefaults(cfg)
	applyPathDefaults(cfg)
	applyProviderDefaults(cfg)
	applyRSSDefaults(cfg)
	applyFontDefault(cfg)
	expandEntryIcons(cfg)
	applyCommandDefaults(cfg)
}

// applyCommandDefaults fills in unset commands. An empty list, as opposed
// to a missing one, stays empty.
func applyCommandDefaults(cfg *Config) {
	if cfg.ScreenOff.OffCommand == nil {
		cfg.ScreenOff.OffCommand = []string{"xset", "dpms", "force", "off"}
	}
	if cfg.ScreenOff.OnCommand == nil {
		cfg.ScreenOff.OnCommand = []string{"xset", "dpms", "force", "on"}
	}
	if cfg.Volume.UpCommand == nil {
		cfg.Volume.UpCommand = []string{"wpctl", "set-volume", "-l", "1.0", "@DEFAULT_AUDIO_SINK@", "5%+"}
	}
	if cfg.Volume.DownCommand == nil {
		cfg.Volume.DownCommand = []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "5%-"}
	}
	if cfg.Volume.GetCommand == nil {
		cfg.Volume.GetCommand = []string{"wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@"}
	}
	if len(cfg.CEC.Command) == 0 {
		cfg.CEC.Command = []string{"cec-client", "-d", "16", "-t", "p", "-o", "BiscuitTin"}
	}
}

func applyChromeDefaults(cfg *Config) {
	applyTitleDefault(cfg)
	setIfEmpty(&cfg.TileColor, "#3d5a80")
	setIfEmpty(&cfg.Background, "#14161f")
	setIfEmpty(&cfg.Accent, "#f2b134")
	if cfg.RestoreHoldSeconds <= 0 {
		cfg.RestoreHoldSeconds = 1.0
	}
}

func applyTitleDefault(cfg *Config) {
	if cfg.Title == "" {
		cfg.Title = "BiscuitTin on {hostname}"
	}
	cfg.Title = strings.ReplaceAll(cfg.Title, "{hostname}", shortHostname())
}

func setIfEmpty(dst *string, def string) {
	if *dst == "" {
		*dst = def
	}
}

func applyPathDefaults(cfg *Config) {
	if cfg.CacheDir == "" {
		cfg.CacheDir = defaultCacheDir()
	}
	cfg.CacheDir = ExpandPath(cfg.CacheDir)
	cfg.Backgrounds.Dir = ExpandPath(cfg.Backgrounds.Dir)
	if cfg.LogDir == "" {
		cfg.LogDir = cfg.CacheDir
	}
	cfg.LogDir = ExpandPath(cfg.LogDir)
}

func defaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "biscuittin")
	}
	return filepath.Join(os.TempDir(), "biscuittin")
}

func applyProviderDefaults(cfg *Config) {
	if cfg.IconProviders == nil {
		cfg.IconProviders = []string{"thesvg"}
	}
	if cfg.Backgrounds.Providers == nil {
		cfg.Backgrounds.Providers = []string{"wikimedia", "picsum"}
	}
	cfg.Backgrounds.Interval = ClampInterval(cfg.Backgrounds.Interval)
	clampBackgroundDim(cfg)
}

func clampBackgroundDim(cfg *Config) {
	if cfg.Backgrounds.Dim <= 0 {
		cfg.Backgrounds.Dim = 0.45
		return
	}
	if cfg.Backgrounds.Dim > 1 {
		cfg.Backgrounds.Dim = 0.45
	}
}

func applyRSSDefaults(cfg *Config) {
	if cfg.RSS.Interval <= 0 {
		cfg.RSS.Interval = 15 * 60
	}
	cfg.RSS.Interval = ClampInterval(cfg.RSS.Interval)
	if cfg.RSS.RotateSeconds <= 0 {
		cfg.RSS.RotateSeconds = 60
	}
	cfg.RSS.RotateSeconds = ClampInterval(cfg.RSS.RotateSeconds)
	cfg.RSS.Feeds = cleanRSSFeeds(cfg.RSS.Feeds)
}

func applyFontDefault(cfg *Config) {
	if cfg.Font != "" {
		return
	}
	for _, f := range defaultFonts {
		if _, err := os.Stat(f); err == nil {
			cfg.Font = f
			return
		}
	}
}

func expandEntryIcons(cfg *Config) {
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
	return expandHome(os.ExpandEnv(p))
}

func expandHome(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
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
// ScreenOffConfig is how the screen is switched off after More > Settings >
// Screen off after, and back on at the next press. The window goes black
// either way; [] runs nothing.
// VolumeConfig is how LB/RB (and - / =) change the volume. getCommand
// prints the level, as wpctl get-volume does ("Volume: 0.45 [MUTED]").
type VolumeConfig struct {
	UpCommand   []string `yaml:"upCommand"`
	DownCommand []string `yaml:"downCommand"`
	GetCommand  []string `yaml:"getCommand"`
}

type ScreenOffConfig struct {
	OffCommand []string `yaml:"offCommand"`
	OnCommand  []string `yaml:"onCommand"`
}

// CECConfig reads TV remote buttons over HDMI-CEC with libCEC's
// cec-client. Kernel CEC drivers already present the remote as a keyboard,
// so they need nothing here.
type CECConfig struct {
	Command []string `yaml:"command"`
	Enabled bool     `yaml:"enabled"`
}

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
