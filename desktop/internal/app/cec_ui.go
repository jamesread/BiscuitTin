package app

import (
	"os/exec"
	"slices"

	"github.com/jamesread/biscuittin/internal/cec"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/launch"
)

var cecActions = map[cec.Key]action{
	cec.KeyUp:     actionUp,
	cec.KeyDown:   actionDown,
	cec.KeyLeft:   actionLeft,
	cec.KeyRight:  actionRight,
	cec.KeySelect: actionActivate,
	cec.KeyBack:   actionBack,
}

// startCEC runs cec-client when cec.enabled is set. Scripted runs never
// do, so screenshots don't depend on the TV.
func (s *session) startCEC() {
	if !s.cfg.CEC.Enabled || s.sc != nil {
		return
	}
	argv := s.cfg.CEC.Command
	s.cec = cec.Start(func() *exec.Cmd { return launch.Host(argv) })
	s.u.cec = s.cec
}

func (s *session) stopCEC() {
	s.cec.Stop()
	s.cec = nil
	s.u.cec = nil
}

// restartCECIfChanged follows a config reload.
func (s *session) restartCECIfChanged(old, cfg *config.Config) {
	if old.CEC.Enabled == cfg.CEC.Enabled && slices.Equal(old.CEC.Command, cfg.CEC.Command) {
		return
	}
	s.stopCEC()
	s.startCEC()
}

// cecAction is the next remote press, as an action. Presses count as the
// keyboard for control help, as CEC remotes have keys rather than glyphs.
func (s *session) cecAction() action {
	if s.cec == nil {
		return actionNone
	}
	p, ok := tryRecv(s.cec.Presses)
	if !ok {
		return actionNone
	}
	s.in.noteRecent("CEC " + p.Name)
	s.in.primary, s.in.usedInput = deviceKeyboard, true
	return cecActions[p.Key]
}

// drainCEC drops presses while an app is in front, so they don't fire when
// the launcher comes back.
func (s *session) drainCEC() {
	if s.cec == nil {
		return
	}
	for {
		if _, ok := tryRecv(s.cec.Presses); !ok {
			return
		}
	}
}
