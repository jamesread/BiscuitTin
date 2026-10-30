package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/launch"
	"go.uber.org/zap"
)

// volumeShowSeconds is how long the volume overlay stays after a change.
const volumeShowSeconds = 2

// volumeResult is the level read back after a volume change.
type volumeResult struct {
	err    error
	level  int
	muted  bool
	parsed bool
}

// runVolumeCommand runs a volume command and returns its output. Tests
// replace this.
var runVolumeCommand = func(argv []string) (string, error) {
	out, err := launch.Host(argv).Output()
	return string(out), err
}

// isVolume is true for the actions handled by changeVolume, which work on
// every screen and never reach the menus.
func isVolume(a action) bool {
	return a == actionVolumeUp || a == actionVolumeDown
}

// changeVolume runs the up or down command, then reads the level back for
// the overlay, off the main thread.
func (s *session) changeVolume(a action) {
	argv := s.cfg.Volume.UpCommand
	if a == actionVolumeDown {
		argv = s.cfg.Volume.DownCommand
	}
	get := s.cfg.Volume.GetCommand
	s.u.volumeUntil = rl.GetTime() + volumeShowSeconds
	go func() {
		s.volumeResults <- readVolumeAfter(argv, get)
	}()
}

func readVolumeAfter(change, get []string) volumeResult {
	if len(change) > 0 {
		if _, err := runVolumeCommand(change); err != nil {
			return volumeResult{err: err}
		}
	}
	if len(get) == 0 {
		return volumeResult{}
	}
	out, err := runVolumeCommand(get)
	if err != nil {
		return volumeResult{err: err}
	}
	level, muted, ok := parseVolume(out)
	return volumeResult{level: level, muted: muted, parsed: ok}
}

func (s *session) drainVolume() {
	r, ok := tryRecv(s.volumeResults)
	if !ok {
		return
	}
	if r.err != nil {
		lg().Warn("volume command failed", zap.Error(r.err))
		s.u.volumeUntil = 0
		s.u.setToast("Volume: "+r.err.Error(), colorBad, 4)
		return
	}
	s.u.volumeLevel, s.u.volumeMuted, s.u.volumeKnown = r.level, r.muted, r.parsed
	s.u.volumeUntil = rl.GetTime() + volumeShowSeconds
}

// parseVolume reads wpctl's "Volume: 0.45" or "Volume: 0.45 [MUTED]" as a
// percentage. A bare number ("45%" or "0.45") is accepted too, for other
// getCommands.
func parseVolume(out string) (int, bool, bool) {
	muted := strings.Contains(strings.ToUpper(out), "MUTED")
	for f := range strings.FieldsSeq(out) {
		if strings.HasSuffix(f, "%") {
			if n, err := strconv.Atoi(strings.TrimSuffix(f, "%")); err == nil {
				return n, muted, true
			}
			continue
		}
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			return int(math.Round(v * 100)), muted, true
		}
	}
	return 0, muted, false
}

func volumeLabel(level int, muted, known bool) string {
	switch {
	case !known:
		return "Volume"
	case muted:
		return fmt.Sprintf("Volume %d%% (muted)", level)
	}
	return fmt.Sprintf("Volume %d%%", level)
}

// drawVolume is the overlay shown for a couple of seconds after a change:
// a label and a bar, top centre, over every screen.
func (u *ui) drawVolume(w, s float32) {
	if rl.GetTime() >= u.volumeUntil {
		return
	}
	size := 34 * s
	label := volumeLabel(u.volumeLevel, u.volumeMuted, u.volumeKnown)
	boxW := float32(math.Max(float64(u.measure(label, size).X+64*s), float64(420*s)))
	box := rl.Rectangle{X: (w - boxW) / 2, Y: 40 * s, Width: boxW, Height: 120 * s}
	rl.DrawRectangleRounded(box, 0.3, 10, rl.ColorAlpha(rl.Black, 0.75))
	ts := u.measure(label, size)
	u.text(label, box.X+(box.Width-ts.X)/2, box.Y+18*s, size, rl.RayWhite)
	bar := rl.Rectangle{X: box.X + 32*s, Y: box.Y + 76*s, Width: box.Width - 64*s, Height: 16 * s}
	rl.DrawRectangleRounded(bar, 1, 8, rl.ColorAlpha(rl.White, 0.2))
	if !u.volumeKnown {
		return
	}
	fill := bar
	fill.Width = bar.Width * float32(min(max(u.volumeLevel, 0), 100)) / 100
	c := u.accentText()
	if u.volumeMuted {
		c = rl.ColorAlpha(rl.White, 0.4)
	}
	if fill.Width > 0 {
		rl.DrawRectangleRounded(fill, 1, 8, c)
	}
}
