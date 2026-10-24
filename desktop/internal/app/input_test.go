package app

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestKeyActionEscIsExit(t *testing.T) {
	if keyAction(rl.KeyEscape) != actionExit {
		t.Fatal("Esc should be actionExit")
	}
	if keyAction(rl.KeyBackspace) != actionBack {
		t.Fatal("Backspace should stay actionBack")
	}
	if keyAction(rl.KeyBack) != actionBack {
		t.Fatal("KEY_BACK should stay actionBack")
	}
}
