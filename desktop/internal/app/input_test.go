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

func TestKeyNames(t *testing.T) {
	cases := map[int32]string{
		rl.KeyEnter: "Enter", rl.KeyQ: "Q", rl.KeySeven: "7", rl.KeyF5: "F5",
		rl.KeyKp3: "Keypad 3", rl.KeyVolumeUp: "Volume Up", 9999: "Key 9999",
	}
	for k, want := range cases {
		if got := keyName(k); got != want {
			t.Errorf("keyName(%d) = %q; want %q", k, got, want)
		}
	}
}

func TestRecentKeysKeepsNewest(t *testing.T) {
	in := &input{}
	if in.recentText() != "Last keys: none yet" {
		t.Fatalf("empty = %q", in.recentText())
	}
	for i := range recentKeysKept + 2 {
		in.noteRecent(keyName(rl.KeyA + int32(i)))
	}
	if len(in.recent) != recentKeysKept || in.recent[0] != "C" || in.recent[recentKeysKept-1] != "J" {
		t.Fatalf("recent = %v", in.recent)
	}
}
