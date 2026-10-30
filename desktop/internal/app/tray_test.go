package app

import (
	"os"
	"syscall"
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

func TestStopSignalQuits(t *testing.T) {
	s := testSession([]config.Entry{{Title: "Sleep", Command: []string{"sleep", "1"}}})
	s.drainStopSignals()
	if s.u.quit {
		t.Fatal("no signal should not quit")
	}
	s.stopSignals = make(chan os.Signal, 1)
	s.stopSignals <- syscall.SIGTERM
	s.drainStopSignals()
	if !s.u.quit {
		t.Fatal("SIGTERM should request quit")
	}
}
