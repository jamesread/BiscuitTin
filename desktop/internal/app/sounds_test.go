package app

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestSoundForAction(t *testing.T) {
	s := uiSounds{
		move:    rl.Sound{FrameCount: 1},
		confirm: rl.Sound{FrameCount: 2},
		back:    rl.Sound{FrameCount: 3},
	}
	if soundForAction(actionDown, s).FrameCount != 1 {
		t.Fatal("move")
	}
	if soundForAction(actionActivate, s).FrameCount != 2 {
		t.Fatal("confirm")
	}
	if soundForAction(actionBack, s).FrameCount != 3 {
		t.Fatal("back")
	}
	if soundForAction(actionNone, s).FrameCount != 0 {
		t.Fatal("silence")
	}
}

func TestSoundFilesEmbedded(t *testing.T) {
	for _, name := range []string{"move.wav", "confirm.wav", "back.wav"} {
		raw, err := soundWAVs.ReadFile("assets/sounds/" + name)
		if err != nil || len(raw) < 100 {
			t.Fatalf("%s: %v len=%d", name, err, len(raw))
		}
	}
}
