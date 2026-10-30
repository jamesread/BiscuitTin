package app

import (
	"math/big"
	"os"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// GLFW reports any input device with absolute axes as a gamepad, including
// foot switches and keyboard receivers. The kernel knows better: a gamepad or
// joystick has buttons in BTN_JOYSTICK..BTN_GAMEPAD's range (0x120-0x13f).
const (
	btnJoystickFirst = 0x120
	btnGamepadLast   = 0x13f
)

// padNamesRefresh is how often, in seconds, the device list is re-read.
const padNamesRefresh = 2.0

const inputDevicesPath = "/proc/bus/input/devices"

// padNames caches which input device names are gamepads or joysticks.
type padNames struct {
	names  map[string]bool // nil when the device list can't be read
	readAt float64
}

func (p *padNames) get(now float64) map[string]bool {
	if p.readAt == 0 || now-p.readAt >= padNamesRefresh {
		p.names = readPadNames(inputDevicesPath)
		p.readAt = now
	}
	return p.names
}

func readPadNames(path string) map[string]bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parsePadNames(string(raw))
}

// parsePadNames returns the names of devices in /proc/bus/input/devices
// format whose KEY bitmap has a joystick or gamepad button.
func parsePadNames(src string) map[string]bool {
	out := map[string]bool{}
	for block := range strings.SplitSeq(src, "\n\n") {
		name, keys := inputDeviceFields(block)
		if name != "" && hasPadButton(keys) {
			out[name] = true
		}
	}
	return out
}

func inputDeviceFields(block string) (name, keys string) {
	for line := range strings.SplitSeq(block, "\n") {
		if v, ok := strings.CutPrefix(line, "N: Name="); ok {
			name = strings.Trim(v, `"`)
		}
		if v, ok := strings.CutPrefix(line, "B: KEY="); ok {
			keys = v
		}
	}
	return name, keys
}

// hasPadButton reads a KEY bitmap: hex words, most significant first, each
// 64 bits wide (on 64-bit kernels).
func hasPadButton(keys string) bool {
	words := strings.Fields(keys)
	bits := new(big.Int)
	for _, w := range words {
		v, ok := new(big.Int).SetString(w, 16)
		if !ok {
			return false
		}
		bits.Lsh(bits, 64).Or(bits, v)
	}
	for b := btnJoystickFirst; b <= btnGamepadLast; b++ {
		if bits.Bit(b) == 1 {
			return true
		}
	}
	return false
}

// gamepadCount is how many connected devices are gamepads or joysticks, or
// the scripted pads in a script run.
func (u *ui) gamepadCount() int {
	if u.scriptPads != nil {
		return len(u.scriptPads)
	}
	names := u.padNames.get(rl.GetTime())
	n := 0
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) && isPad(names, rl.GetGamepadName(g)) {
			n++
		}
	}
	return n
}

// isPad trusts GLFW when the kernel's device list couldn't be read.
func isPad(names map[string]bool, name string) bool {
	if names == nil {
		return true
	}
	return names[name]
}
