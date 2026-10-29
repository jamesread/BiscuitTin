package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/tray"
)

func TestApplyTrayQuit(t *testing.T) {
	s := testSession([]config.Entry{{Title: "Sleep", Command: []string{"sleep", "1"}}})
	s.applyTray(tray.EventQuit)
	if !s.u.quit {
		t.Fatal("tray Exit should request quit")
	}
}
