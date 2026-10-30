package x11win

import (
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// ScreenSaver resets the X server's idle timer, which both the screensaver
// and DPMS count from, as a key press would. Gamepads are read by apps
// directly, not through X, so without this the TV can blank mid-game.
type ScreenSaver struct {
	conn   *xgb.Conn
	err    error
	opened sync.Once
}

// Reset reports activity to the X server. The connection is opened on first
// use; if that fails (no X, e.g. Wayland) the error is returned once and
// later calls do nothing.
func (s *ScreenSaver) Reset() error {
	first := false
	s.opened.Do(func() {
		first = true
		s.conn, s.err = xgb.NewConn()
	})
	if s.err != nil {
		if first {
			return s.err
		}
		return nil
	}
	return xproto.ForceScreenSaverChecked(s.conn, xproto.ScreenSaverReset).Check()
}

// Close closes the X connection, if one was opened.
func (s *ScreenSaver) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}
