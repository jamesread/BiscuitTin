package app

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/launch"
	"github.com/jamesread/biscuittin/internal/settings"
	"go.uber.org/zap"
)

// runScreenCommand runs a screen off or on command. Tests replace this.
var runScreenCommand = func(argv []string) error {
	return launch.Host(argv).Run()
}

func startScreenCommand(argv []string) {
	if len(argv) == 0 {
		return
	}
	go func() {
		if err := runScreenCommand(argv); err != nil {
			lg().Warn("screen command failed", zap.Strings("argv", argv), zap.Error(err))
		}
	}()
}

// applyScreenOff blanks the screen once nothing has been pressed for the
// Screen off after setting, and wakes it on the next press, which does
// nothing else.
func (s *session) applyScreenOff(a action) action {
	if s.screenOff {
		if a == actionNone {
			return a
		}
		s.wakeScreen()
		return actionNone
	}
	if s.screenOffDue() {
		s.blankScreen()
	}
	return a
}

func (s *session) screenOffDue() bool {
	after := s.settings.ScreenOffAfter
	if after <= 0 || s.sc != nil || s.blocksIdle() {
		return false
	}
	return rl.GetTime()-s.lastInput >= float64(after)
}

func (s *session) blankScreen() {
	s.screenOff = true
	lg().Info("screen off")
	startScreenCommand(s.cfg.ScreenOff.OffCommand)
}

func (s *session) wakeScreen() {
	s.screenOff = false
	s.lastInput = rl.GetTime()
	lg().Info("screen on")
	startScreenCommand(s.cfg.ScreenOff.OnCommand)
}

// drawScreenOff is the frame while the screen is off: black, and slow, as
// only input needs polling.
func drawScreenOff() {
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	rl.EndDrawing()
	rl.WaitTime(0.05)
}

func screenOffLabel(seconds int) string {
	if seconds <= 0 {
		return "Never"
	}
	return settings.IntervalLabel(seconds)
}

func (u *ui) screenOffItem(st *settings.Settings, changed func()) menuItem {
	return menuItem{
		title: "Screen off after",
		value: func() string { return screenOffLabel(st.ScreenOffAfter) },
		adjust: func(dir int) {
			st.ScreenOffAfter = settings.StepScreenOff(st.ScreenOffAfter, dir)
			changed()
		},
	}
}
