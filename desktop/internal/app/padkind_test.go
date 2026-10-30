package app

import "testing"

// Device blocks as /proc/bus/input/devices shows them: an Xbox pad over
// Bluetooth, and a foot switch and keyboard receiver that GLFW also reports
// as gamepads because they have absolute axes.
const testInputDevices = `I: Bus=0003 Vendor=3553 Product=b001 Version=0110
N: Name="PCsensor FootSwitch Keyboard"
B: EV=12001b
B: KEY=733eff 0 0 483ffef17aff32d bfc4444600000000 1 130c730b17c007 ffbf7bfad9415fff febeffdfffefffff fffffffffffffffe
B: ABS=100000000

I: Bus=0003 Vendor=3553 Product=b001 Version=0110
N: Name="PCsensor FootSwitch"
B: EV=9
B: ABS=10000000000

I: Bus=0003 Vendor=3554 Product=fc03 Version=0110
N: Name="2.4G Receiver"
B: EV=120013
B: KEY=1000000000007 ff9f207ac14057ff febeffdfffefffff fffffffffffffffe

I: Bus=0005 Vendor=045e Product=0b13 Version=0523
N: Name="Xbox Wireless Controller"
B: EV=20001b
B: KEY=7cdb000000000000 0 0 0 0
B: ABS=20003003f
`

func TestParsePadNames(t *testing.T) {
	got := parsePadNames(testInputDevices)
	if len(got) != 1 || !got["Xbox Wireless Controller"] {
		t.Errorf("parsePadNames = %v; want only the Xbox controller", got)
	}
}

func TestHasPadButton(t *testing.T) {
	cases := map[string]bool{
		"":                      false,
		"1 0 0 0 0":             false, // BTN_MISC (0x100) only
		"1 0 0 0 0 0":           false, // KEY_* at 0x140
		"100000000 0 0 0 0":     true,  // BTN_TRIGGER (0x120), a joystick
		"1000000000000 0 0 0 0": true,  // BTN_SOUTH (0x130)
		"not-hex 0 0 0 0":       false,
	}
	for keys, want := range cases {
		if got := hasPadButton(keys); got != want {
			t.Errorf("hasPadButton(%q) = %v; want %v", keys, got, want)
		}
	}
}

func TestIsPadTrustsGLFWWithoutDeviceList(t *testing.T) {
	if !isPad(nil, "Anything") {
		t.Error("isPad(nil) should trust GLFW")
	}
	if isPad(map[string]bool{"Xbox Wireless Controller": true}, "PCsensor FootSwitch") {
		t.Error("foot switch counted as a gamepad")
	}
}
