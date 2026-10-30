package app

import (
	"errors"
	"slices"
	"testing"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/internal/webcontrol"
)

func TestParseVolume(t *testing.T) {
	cases := []struct {
		in           string
		level        int
		muted, known bool
	}{
		{"Volume: 0.45\n", 45, false, true},
		{"Volume: 1.00 [MUTED]\n", 100, true, true},
		{"72%", 72, false, true},
		{"no level here", 0, false, false},
	}
	for _, c := range cases {
		level, muted, known := parseVolume(c.in)
		if level != c.level || muted != c.muted || known != c.known {
			t.Errorf("parseVolume(%q) = %d, %v, %v", c.in, level, muted, known)
		}
	}
}

func TestReadVolumeAfter(t *testing.T) {
	var ran [][]string
	old := runVolumeCommand
	t.Cleanup(func() { runVolumeCommand = old })
	runVolumeCommand = func(argv []string) (string, error) {
		ran = append(ran, argv)
		return "Volume: 0.30", nil
	}
	r := readVolumeAfter([]string{"up"}, []string{"get"})
	if r.err != nil || r.level != 30 || !r.parsed {
		t.Fatalf("result = %+v", r)
	}
	if len(ran) != 2 || !slices.Equal(ran[0], []string{"up"}) || !slices.Equal(ran[1], []string{"get"}) {
		t.Fatalf("ran %v", ran)
	}

	runVolumeCommand = func([]string) (string, error) { return "", errors.New("no wpctl") }
	if r := readVolumeAfter([]string{"up"}, []string{"get"}); r.err == nil {
		t.Fatal("a failing command should report its error")
	}
}

func TestVolumeActions(t *testing.T) {
	if !isVolume(actionVolumeUp) || !isVolume(actionVolumeDown) || isVolume(actionUp) {
		t.Fatal("isVolume")
	}
	if buttonActions[pb.Button_BUTTON_VOLUME_UP] != actionVolumeUp || buttonActions[pb.Button_BUTTON_VOLUME_DOWN] != actionVolumeDown {
		t.Fatal("web remote volume buttons are not mapped")
	}
	if soundForAction(actionVolumeUp, uiSounds{}).FrameCount != 0 {
		t.Fatal("volume should not click")
	}
}

func TestVolumeLabel(t *testing.T) {
	if got := volumeLabel(40, true, true); got != "Volume 40% (muted)" {
		t.Fatal(got)
	}
	if got := volumeLabel(0, false, false); got != "Volume" {
		t.Fatal(got)
	}
}

func TestDrainRemoteKeepsVolumeWhileHidden(t *testing.T) {
	b := webcontrol.NewBridge()
	for _, btn := range []pb.Button{pb.Button_BUTTON_SELECT, pb.Button_BUTTON_VOLUME_UP, pb.Button_BUTTON_BACK} {
		b.Commands <- webcontrol.Command{Button: btn}
	}
	var got []action
	drainRemote(b, func(a action) { got = append(got, a) })
	if !slices.Equal(got, []action{actionVolumeUp}) || len(b.Commands) != 0 {
		t.Fatalf("got %v, %d left", got, len(b.Commands))
	}
}
