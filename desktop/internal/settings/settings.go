// Package settings stores the choices made in the on-screen menus, separately
// from the config file.
package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/fsutil"
	"github.com/jamesread/biscuittin/internal/integrations"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Intervals are the steps offered in the More menu, in seconds.
var Intervals = []int{60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400}

// Settings are the values changed from the on-screen menus. They are stored
// separately from the hand-written config (which they override), so the
// config file is never rewritten by the app.
type Settings struct {
	File string `yaml:"-"` // where the settings are saved
	// FocusBorder and FocusFill colour the selected tile, menu row or
	// action ("#rrggbb"). Empty follows the config's accent.
	FocusBorder string `yaml:"focusBorder,omitempty"`
	FocusFill   string `yaml:"focusFill,omitempty"`
	// AccentText colours headings, focused values and other highlighted
	// text ("#rrggbb"). Empty follows the config's accent.
	AccentText string `yaml:"accentText,omitempty"`
	// WebCode is the web remote's pairing code, made when web control is
	// first switched on.
	WebCode string `yaml:"webCode,omitempty"`
	// PinnedActions are pinned from More (OliveTin, Home Assistant, ...) and
	// shown as tiles.
	PinnedActions []integrations.PinnedAction `yaml:"pinnedActions,omitempty"`
	// LegacyOliveTinActions is the pre-Home Assistant name for PinnedActions,
	// read once and migrated.
	LegacyOliveTinActions []integrations.PinnedAction `yaml:"olivetinActions,omitempty"`
	// TileOrder is the home screen order, by tile ID ("entry/<title>" or a
	// pinned action's key); HiddenTiles are left off the home screen.
	TileOrder   []string `yaml:"tileOrder,omitempty"`
	HiddenTiles []string `yaml:"hiddenTiles,omitempty"`
	// RecentFirst puts the most recently launched tiles first on the home
	// screen; LastUsed is when each tile (by tile ID) was last launched, in
	// unix seconds.
	LastUsed    map[string]int64 `yaml:"lastUsed,omitempty"`
	RecentFirst bool             `yaml:"recentFirst"`
	// QRPlace is none, menu (home tiles) or idle. Default none.
	QRPlace            QRPlace `yaml:"qrPlace"`
	BackgroundInterval int     `yaml:"backgroundInterval"`
	// ScreenOffAfter blanks the screen after this many idle seconds; 0 is
	// never (the default).
	ScreenOffAfter int `yaml:"screenOffAfter"`
	// InterfaceSize is a percentage of the normal size: 85, 100, 115 or
	// 130. Unset is 100.
	InterfaceSize int `yaml:"interfaceSize,omitempty"`
	// RSSInterval is how long each headline stays on the idle screen.
	RSSInterval        int  `yaml:"rssInterval"`
	BackgroundsEnabled bool `yaml:"backgroundsEnabled"`
	// RSSEnabled shows idle headlines when feeds are configured. Default on.
	RSSEnabled bool `yaml:"rssEnabled"`
	// HideAttribution hides the background image credit, separately from
	// the title; HideHelp hides the control hints in the footer.
	HideAttribution bool `yaml:"hideAttribution"`
	HideHelp        bool `yaml:"hideHelp"`
	// SleepPowerOff makes the Sleep timer power off instead of suspend.
	SleepPowerOff bool `yaml:"sleepPowerOff"`
	// MuteMenuSounds silences the move, select and back clicks. Default
	// off.
	MuteMenuSounds bool `yaml:"muteMenuSounds"`
	// ShowImageTitle shows the picture's title above its credit. Default
	// off.
	ShowImageTitle bool `yaml:"showImageTitle"`
	// WebControl runs the web remote on 0.0.0.0.
	WebControl bool `yaml:"webControl"`
}

// QRPlace is where the web remote QR is drawn.
type QRPlace int

const (
	QRNone QRPlace = iota
	QRMenu
	QRIdle
)

// QRPlaceLabels is the Settings radio order: None, Main menu, Idle screen.
var QRPlaceLabels = []string{"None", "Main menu", "Idle screen"}

var qrPlaceNames = []string{"none", "menu", "idle"}

func settingsPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "biscuittin-settings.yaml")
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "biscuittin", "settings.yaml")
}

func defaultSettings(cfg *config.Config) *Settings {
	return &Settings{
		BackgroundsEnabled: config.BoolOr(cfg.Backgrounds.Enabled, true),
		RSSEnabled:         true,
		BackgroundInterval: cfg.Backgrounds.Interval,
		RSSInterval:        cfg.RSS.RotateSeconds,
		File:               settingsPath(),
	}
}

func applySaved(s *Settings) {
	raw, err := os.ReadFile(s.File)
	if err != nil {
		return
	}
	if err := yaml.Unmarshal(raw, s); err != nil {
		zap.L().Named("settings").Warn("ignoring unreadable settings", zap.String("path", s.File), zap.Error(err))
		return
	}
	applyLegacyQR(s, raw)
}

func applyLegacyQR(s *Settings, raw []byte) {
	var old struct {
		QRPlace *QRPlace `yaml:"qrPlace"`
		ShowQR  *bool    `yaml:"showQR"`
	}
	_ = yaml.Unmarshal(raw, &old)
	if old.QRPlace != nil {
		return
	}
	if old.ShowQR != nil && *old.ShowQR {
		s.QRPlace = QRMenu
	}
}

func clampIntervals(s *Settings, cfg *config.Config) {
	s.BackgroundInterval = config.ClampInterval(s.BackgroundInterval)
	if s.RSSInterval <= 0 {
		s.RSSInterval = cfg.RSS.RotateSeconds
	}
	s.RSSInterval = config.ClampInterval(s.RSSInterval)
}

func migrateLegacyPins(s *Settings) {
	if len(s.LegacyOliveTinActions) == 0 {
		return
	}
	for _, a := range s.LegacyOliveTinActions {
		a.Source = "olivetin"
		s.PinnedActions = append(s.PinnedActions, a)
	}
	s.LegacyOliveTinActions = nil
}

// Load starts from the config's defaults and applies any saved
// settings on top.
func Load(cfg *config.Config) *Settings {
	s := defaultSettings(cfg)
	applySaved(s)
	clampIntervals(s, cfg)
	migrateLegacyPins(s)
	normalizeQR(s)
	return s
}

func normalizeQR(s *Settings) {
	if s.QRPlace < QRNone || s.QRPlace > QRIdle {
		s.QRPlace = QRNone
	}
}

func (p QRPlace) MarshalYAML() (any, error) {
	if p >= QRNone && p <= QRIdle {
		return qrPlaceNames[p], nil
	}
	return qrPlaceNames[QRNone], nil
}

func (p *QRPlace) UnmarshalYAML(n *yaml.Node) error {
	var name string
	if err := n.Decode(&name); err == nil {
		*p = qrPlaceNamed(name)
		return nil
	}
	var i int
	if err := n.Decode(&i); err != nil {
		return err
	}
	*p = QRPlace(i)
	return nil
}

func qrPlaceNamed(name string) QRPlace {
	switch name {
	case "menu":
		return QRMenu
	case "idle":
		return QRIdle
	}
	return QRNone
}

func (s *Settings) Save() error {
	raw, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.File), 0o750); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.File, raw)
}

// StepInterval moves to the previous (dir < 0) or next (dir > 0) menu step,
// snapping values that are between steps (e.g. set by hand in config).
func nextInterval(current int) int {
	for _, v := range Intervals {
		if v > current {
			return v
		}
	}
	return Intervals[len(Intervals)-1]
}

func prevInterval(current int) int {
	for _, Interval := range slices.Backward(Intervals) {
		if Interval < current {
			return Interval
		}
	}
	return Intervals[0]
}

func StepInterval(current, dir int) int {
	if dir > 0 {
		return nextInterval(current)
	}
	return prevInterval(current)
}

func intervalUnit(seconds, unit int, name string) (string, bool) {
	if seconds < unit {
		return "", false
	}
	if seconds%unit != 0 {
		return "", false
	}
	return plural(seconds/unit, name), true
}

func IntervalLabel(seconds int) string {
	if label, ok := intervalUnit(seconds, 86400, "day"); ok {
		return label
	}
	if label, ok := intervalUnit(seconds, 3600, "hour"); ok {
		return label
	}
	if label, ok := intervalUnit(seconds, 60, "minute"); ok {
		return label
	}
	return plural(seconds, "second")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// ScreenOffSteps are the Screen off after choices, in seconds; 0 is never.
var ScreenOffSteps = []int{0, 300, 600, 900, 1800, 3600, 7200, 14400}

// StepScreenOff moves through ScreenOffSteps like StepInterval, or cycles
// round for dir 0.
func StepScreenOff(current, dir int) int {
	i := slices.Index(ScreenOffSteps, current)
	if i < 0 {
		i = 0
		for i+1 < len(ScreenOffSteps) && ScreenOffSteps[i+1] <= current {
			i++
		}
	}
	if dir == 0 {
		return ScreenOffSteps[(i+1)%len(ScreenOffSteps)]
	}
	return ScreenOffSteps[min(max(i+dir, 0), len(ScreenOffSteps)-1)]
}
