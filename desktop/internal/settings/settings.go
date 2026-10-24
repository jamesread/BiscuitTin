// Package settings stores the choices made in the on-screen menus, separately
// from the config file.
package settings

import (
	"fmt"
	"os"
	"path/filepath"

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
	BackgroundsEnabled bool `yaml:"backgroundsEnabled"`
	BackgroundInterval int  `yaml:"backgroundInterval"`
	// RSSInterval is how long each headline stays on the idle screen.
	RSSInterval int `yaml:"rssInterval"`

	// HideAttribution hides the background image credit; HideHelp hides the
	// control hints in the footer.
	HideAttribution bool `yaml:"hideAttribution"`
	HideHelp        bool `yaml:"hideHelp"`

	// WebControl runs the web remote on 0.0.0.0; ShowQR draws its address as
	// a QR code on the home screen.
	WebControl bool `yaml:"webControl"`
	ShowQR     bool `yaml:"showQR"`

	// PinnedActions are pinned from More (OliveTin, Home Assistant, ...) and
	// shown as tiles.
	PinnedActions []integrations.PinnedAction `yaml:"pinnedActions,omitempty"`

	// LegacyOliveTinActions is the pre-Home Assistant name for PinnedActions,
	// read once and migrated.
	LegacyOliveTinActions []integrations.PinnedAction `yaml:"olivetinActions,omitempty"`

	File string `yaml:"-"` // where the settings are saved
}

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

// Load starts from the config's defaults and applies any saved
// settings on top.
func Load(cfg *config.Config) *Settings {
	s := &Settings{
		BackgroundsEnabled: config.BoolOr(cfg.Backgrounds.Enabled, true),
		BackgroundInterval: cfg.Backgrounds.Interval,
		RSSInterval:        cfg.RSS.RotateSeconds,
		File:               settingsPath(),
	}

	if raw, err := os.ReadFile(s.File); err == nil {
		if err := yaml.Unmarshal(raw, s); err != nil {
			zap.L().Named("settings").Warn("ignoring unreadable settings", zap.String("path", s.File), zap.Error(err))
		}
	}

	s.BackgroundInterval = config.ClampInterval(s.BackgroundInterval)
	if s.RSSInterval <= 0 {
		s.RSSInterval = cfg.RSS.RotateSeconds
	}
	s.RSSInterval = config.ClampInterval(s.RSSInterval)
	if len(s.LegacyOliveTinActions) > 0 {
		for _, a := range s.LegacyOliveTinActions {
			a.Source = "olivetin"
			s.PinnedActions = append(s.PinnedActions, a)
		}
		s.LegacyOliveTinActions = nil
	}
	return s
}

func (s *Settings) Save() error {
	raw, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.File), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.File, raw)
}

// StepInterval moves to the previous (dir < 0) or next (dir > 0) menu step,
// snapping values that are between steps (e.g. set by hand in config).
func StepInterval(current, dir int) int {
	if dir > 0 {
		for _, v := range Intervals {
			if v > current {
				return v
			}
		}
		return Intervals[len(Intervals)-1]
	}
	for i := len(Intervals) - 1; i >= 0; i-- {
		if Intervals[i] < current {
			return Intervals[i]
		}
	}
	return Intervals[0]
}

func IntervalLabel(seconds int) string {
	switch {
	case seconds >= 86400 && seconds%86400 == 0:
		return plural(seconds/86400, "day")
	case seconds >= 3600 && seconds%3600 == 0:
		return plural(seconds/3600, "hour")
	case seconds >= 60 && seconds%60 == 0:
		return plural(seconds/60, "minute")
	}
	return plural(seconds, "second")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
