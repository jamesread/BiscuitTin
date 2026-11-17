package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
)

func TestScreenOffWakesOnAnyPadActivity(t *testing.T) {
	orig := runScreenCommand
	runScreenCommand = func([]string) error { return nil }
	defer func() { runScreenCommand = orig }()

	cfg := &config.Config{}
	cfg.ScreenOff.OnCommand = []string{"on"}
	s := &session{cfg: cfg, in: &input{}, screenOff: true}

	if a := s.applyScreenOff(actionNone); a != actionNone || !s.screenOff {
		t.Fatalf("idle frame woke the screen")
	}

	// An unmapped button (e.g. a trigger) gives no action but is activity.
	s.in.active = true
	if a := s.applyScreenOff(actionNone); a != actionNone || s.screenOff {
		t.Fatalf("pad activity did not wake the screen")
	}
}
