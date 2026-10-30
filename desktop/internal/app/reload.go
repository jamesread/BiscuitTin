package app

import (
	"image/color"
	"os"
	"slices"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/backgrounds"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/icons"
	"github.com/jamesread/biscuittin/internal/integrations"
	"github.com/jamesread/biscuittin/internal/rss"
	"go.uber.org/zap"
)

// configCheckSeconds is how often the config file is checked for changes.
const configCheckSeconds = 2

// configStamp identifies one version of the config file on disk.
type configStamp struct {
	mod  time.Time
	size int64
}

func statConfig(path string) (configStamp, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return configStamp{}, false
	}
	return configStamp{mod: fi.ModTime(), size: fi.Size()}, true
}

// watchConfig reloads the config when the file changes, or when More >
// System > Reload config asks.
func (s *session) watchConfig() {
	if s.u.reloadRequested {
		s.u.reloadRequested = false
		s.reloadConfig(true)
		return
	}
	now := rl.GetTime()
	if now < s.configCheckAt {
		return
	}
	s.configCheckAt = now + configCheckSeconds
	stamp, ok := statConfig(s.configPath)
	if !ok || stamp == s.configStamp {
		return
	}
	s.reloadConfig(false)
}

// reloadConfig reads the config again and applies it. A file that is
// missing or does not parse keeps the running config. asked is true for
// Reload config, which goes home and always confirms.
func (s *session) reloadConfig(asked bool) {
	stamp, ok := statConfig(s.configPath)
	if !ok {
		s.u.setToast("Config not found: "+s.configPath, colorBad, 6)
		return
	}
	s.configStamp = stamp
	cfg, err := config.Load(s.configPath)
	if err != nil {
		lg().Warn("config reload failed", zap.String("path", s.configPath), zap.Error(err))
		s.u.setToast("Config not reloaded: "+err.Error(), colorBad, 8)
		return
	}
	s.applyConfig(cfg)
	lg().Info("config reloaded", zap.String("path", s.configPath), zap.Int("entries", len(cfg.Entries)))
	if asked {
		s.u.screen = screenHome
	}
	s.u.setToast("Config reloaded", colorOK, 3)
}

// applyConfig swaps in a new config. Window, font and web remote port
// changes need a restart; everything else follows straight away.
func (s *session) applyConfig(cfg *config.Config) {
	old := s.cfg
	s.cfg = cfg
	s.u.remapPIDs(old, cfg)
	s.waitingFor = remapEntry(old, cfg, s.waitingFor)
	s.applySessionOptions(cfg)
	s.u.applyConfig(cfg)
	s.restartFetchers(cfg)
	s.restartCECIfChanged(old, cfg)
	s.iconRequested = map[string]bool{}
	s.goIcon(func() { icons.Resolve(cfg, entryKey, s.iconResults) })
	s.requestActionIcons()
	s.checkInstalled(cfg)
}

func (s *session) applySessionOptions(cfg *config.Config) {
	s.hideOnLaunch = config.BoolOr(cfg.HideOnLaunch, true)
	s.awake.enabled = config.BoolOr(cfg.KeepAwake, true)
	s.idleFade = 20
	if cfg.IdleFadeSeconds != nil {
		s.idleFade = *cfg.IdleFadeSeconds
	}
}

func (s *session) restartFetchers(cfg *config.Config) {
	s.bg.Stop()
	s.bg = backgrounds.NewFetcher(cfg)
	s.bgPending = false
	s.bgNext = 0
	s.u.rss.Stop()
	s.u.rss = rss.NewFetcher(cfg)
}

// applyConfig is the ui half of a reload: colours, sources and tiles.
func (u *ui) applyConfig(cfg *config.Config) {
	u.cfg = cfg
	u.applyConfigColors(cfg)
	u.sources = integrations.NewSources(cfg)
	u.lists = map[string]*uiActionList{}
	if u.screen == screenActionList {
		u.openMenu(screenIntegrations)
	}
	u.unloadEntryIcons()
	for i, e := range cfg.Entries {
		if e.Icon != "" {
			u.setIcon(entryKey(i), e.Icon)
		}
	}
	u.rebuildTiles()
}

func (u *ui) applyConfigColors(cfg *config.Config) {
	u.bg = contrast.ParseHex(cfg.Background, color.RGBA{20, 22, 31, 255})
	u.accent = contrast.ParseHex(cfg.Accent, color.RGBA{242, 177, 52, 255})
	u.tileColor = contrast.AccessibleTileColor(contrast.ParseHex(cfg.TileColor, color.RGBA{61, 90, 128, 255}))
}

// unloadEntryIcons drops entry icons, which are keyed by position and so
// may belong to a different entry after a reload.
func (u *ui) unloadEntryIcons() {
	for _, t := range u.tiles {
		if t.kind != tileEntry {
			continue
		}
		if tex := u.icons[t.key]; tex.ID != 0 {
			rl.UnloadTexture(tex)
		}
		delete(u.icons, t.key)
	}
}

// remapPIDs keeps running apps attached to their entry, matched by title,
// when entries are added, removed or reordered.
func (u *ui) remapPIDs(old, cfg *config.Config) {
	pids := make([][]int, len(cfg.Entries))
	for i, p := range u.pids {
		if j := remapEntry(old, cfg, i); j >= 0 {
			pids[j] = append(pids[j], p...)
		}
	}
	u.pids = pids
}

// remapEntry is the index in cfg of old's entry i, by title, or -1.
func remapEntry(old, cfg *config.Config, i int) int {
	if i < 0 || i >= len(old.Entries) {
		return -1
	}
	for j, e := range cfg.Entries {
		if e.Title == old.Entries[i].Title {
			return j
		}
	}
	return -1
}

// entryForPID is the entry a launched process belongs to, or -1 once its
// entry has been removed from the config.
func (u *ui) entryForPID(pid int) int {
	for i, pids := range u.pids {
		if slices.Contains(pids, pid) {
			return i
		}
	}
	return -1
}
