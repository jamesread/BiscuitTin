package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeAudioEnv(vars map[string]string, uid int) audioEnv {
	return audioEnv{getenv: func(k string) string { return vars[k] }, stat: os.Stat, uid: uid}
}

func TestSoundServer(t *testing.T) {
	dir := t.TempDir()
	env := fakeAudioEnv(map[string]string{"XDG_RUNTIME_DIR": dir}, os.Getuid())
	if got := env.soundServer(); got != "None found in "+dir {
		t.Errorf("empty dir: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "pipewire-0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pulse"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pulse", "native"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := env.soundServer(); got != "PipeWire + PulseAudio socket" {
		t.Errorf("both sockets: %q", got)
	}
	env = fakeAudioEnv(map[string]string{"PULSE_SERVER": "tcp:192.0.2.1"}, 0)
	if got := env.soundServer(); got != "PULSE_SERVER=tcp:192.0.2.1" {
		t.Errorf("PULSE_SERVER: %q", got)
	}
	if got := fakeAudioEnv(nil, 0).soundServer(); !strings.HasPrefix(got, "Unknown") {
		t.Errorf("no runtime dir: %q", got)
	}
}

// A runtime dir owned by someone else is what silenced sound when BiscuitTin
// was started over ssh as root with runuser.
func TestRuntimeDirFlagsOtherOwner(t *testing.T) {
	dir := t.TempDir()
	if got := fakeAudioEnv(map[string]string{"XDG_RUNTIME_DIR": dir}, os.Getuid()).runtimeDir(); got != dir {
		t.Errorf("own dir: %q", got)
	}
	got := fakeAudioEnv(map[string]string{"XDG_RUNTIME_DIR": dir}, os.Getuid()+1).runtimeDir()
	if !strings.Contains(got, "not this user") {
		t.Errorf("other owner not flagged: %q", got)
	}
	if got := fakeAudioEnv(map[string]string{"XDG_RUNTIME_DIR": filepath.Join(dir, "gone")}, 0).runtimeDir(); !strings.HasSuffix(got, "(missing)") {
		t.Errorf("missing dir: %q", got)
	}
	if got := fakeAudioEnv(nil, 0).runtimeDir(); got != "Not set" {
		t.Errorf("unset: %q", got)
	}
}

func TestHandleAudioLeavesOnBackOnly(t *testing.T) {
	u := &ui{screen: screenAudio}
	u.handleAudio(actionActivate)
	if u.screen != screenAudio {
		t.Fatal("A should play the test sound, not leave")
	}
	u.handleAudio(actionBack)
	if u.screen != screenMore {
		t.Fatalf("Back left to %v; want More", u.screen)
	}
}

func TestMoreMenuHasAudioDiagnostics(t *testing.T) {
	u := &ui{}
	for _, it := range u.moreMenu() {
		if it.title == "Audio diagnostics" && it.target == screenAudio {
			return
		}
	}
	t.Fatal("More menu has no Audio diagnostics item")
}
