package app

import (
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/launch"
	"go.uber.org/zap"
)

// installedResult is which entries' commands are missing, for one config.
// gen drops results from before a reload.
type installedResult struct {
	missing map[string]bool
	gen     int
}

// entryInstalled is launch.Installed; tests replace it.
var entryInstalled = launch.Installed

// checkInstalled looks for each entry's program on the host in the
// background, so missing ones can be shown as Not installed. Scripted runs
// (screenshots) use fictional entries, so they skip it.
func (s *session) checkInstalled(cfg *config.Config) {
	if s.sc != nil {
		return
	}
	s.installedGen++
	gen := s.installedGen
	entries := cfg.Entries
	go func() {
		s.installedResults <- findMissing(entries, gen)
	}()
}

func findMissing(entries []config.Entry, gen int) installedResult {
	missing := map[string]bool{}
	for i, e := range entries {
		if ok, known := entryInstalled(e.Command); known && !ok {
			lg().Info("not installed", zap.String("title", e.Title), zap.Strings("command", e.Command))
			missing[entryKey(i)] = true
		}
	}
	return installedResult{missing: missing, gen: gen}
}

func (s *session) drainInstalled() {
	r, ok := tryRecv(s.installedResults)
	if !ok || r.gen != s.installedGen {
		return
	}
	s.u.missing = r.missing
}

// tileMissing is true for an entry tile whose command isn't installed.
func (u *ui) tileMissing(t tile) bool {
	return t.kind == tileEntry && u.missing[t.key]
}
