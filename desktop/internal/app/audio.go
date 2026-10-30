package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// audioEnv is what the audio page reads from the process environment and the
// file system, kept apart from raylib so it can be tested.
type audioEnv struct {
	getenv func(string) string
	stat   func(string) (os.FileInfo, error)
	uid    int
}

func osAudioEnv() audioEnv {
	return audioEnv{getenv: os.Getenv, stat: os.Stat, uid: os.Getuid()}
}

// soundServer names the sound server sockets found in the runtime dir.
// PipeWire's PulseAudio shim also creates pulse/native, so both are listed.
func (e audioEnv) soundServer() string {
	if v := e.getenv("PULSE_SERVER"); v != "" {
		return "PULSE_SERVER=" + v
	}
	dir := e.getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "Unknown (XDG_RUNTIME_DIR not set)"
	}
	found := e.serverSockets(dir)
	if len(found) == 0 {
		return "None found in " + dir
	}
	return strings.Join(found, " + ")
}

var serverSockets = []struct{ name, path string }{
	{"PipeWire", "pipewire-0"},
	{"PulseAudio socket", filepath.Join("pulse", "native")},
}

func (e audioEnv) serverSockets(dir string) []string {
	var found []string
	for _, s := range serverSockets {
		if e.exists(filepath.Join(dir, s.path)) {
			found = append(found, s.name)
		}
	}
	return found
}

func (e audioEnv) exists(path string) bool {
	_, err := e.stat(path)
	return err == nil
}

// runtimeDir shows XDG_RUNTIME_DIR, and says so when another user owns it:
// sound servers then refuse the connection and nothing plays.
func (e audioEnv) runtimeDir() string {
	dir := e.getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "Not set"
	}
	fi, err := e.stat(dir)
	if err != nil {
		return dir + "  (missing)"
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if ok && int(st.Uid) != e.uid {
		return fmt.Sprintf("%s  (owned by uid %d, not this user, uid %d)", dir, st.Uid, e.uid)
	}
	return dir
}

func (s uiSounds) loadedCount() int {
	n := 0
	for _, snd := range []rl.Sound{s.move, s.confirm, s.back} {
		if snd.FrameCount != 0 {
			n++
		}
	}
	return n
}

func audioDeviceLabel(ready bool) string {
	if ready {
		return "Ready"
	}
	return "Not available (no output device opened)"
}

func menuSoundsLabel(loaded int, muted bool) string {
	label := fmt.Sprintf("%d of 3 loaded", loaded)
	if muted {
		label += ", off in Settings"
	}
	return label
}

func testSoundLabel(playing bool) string {
	if playing {
		return "Playing"
	}
	return "Press A or Enter to play"
}

func (u *ui) audioDiagnostics() []diagRow {
	env := osAudioEnv()
	return []diagRow{
		{"Audio device", audioDeviceLabel(rl.IsAudioDeviceReady())},
		{"Sound server", env.soundServer()},
		{"Runtime dir", env.runtimeDir()},
		{"Master volume", fmt.Sprintf("%.0f%%", rl.GetMasterVolume()*100)},
		{"Menu sounds", menuSoundsLabel(u.sounds.loadedCount(), u.menuSoundsMuted())},
		{"Test sound", testSoundLabel(u.sounds.confirm.FrameCount != 0 && rl.IsSoundPlaying(u.sounds.confirm))},
	}
}

// handleAudio leaves on Back. A needs no handling: the confirm tick that
// every A press plays is the test sound.
func (u *ui) handleAudio(a action) {
	switch a {
	case actionBack, actionExit:
		u.leaveInfoScreen(a)
	}
}

func audioHints() []hint {
	return []hint{
		{pad: []glyphRole{glyphConfirm}, keys: []string{"Enter"}, label: "Test sound"},
		backHint(),
	}
}
