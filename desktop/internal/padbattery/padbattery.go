// Package padbattery reads wireless gamepad batteries from sysfs. Drivers
// for Xbox, PlayStation and other pads (xpad, xone, hid-sony,
// hid-playstation, hid-nintendo) register a power supply with scope
// "Device", which keeps laptop and UPS batteries out.
package padbattery

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Root is where the kernel lists power supplies.
const Root = "/sys/class/power_supply"

// Battery is one controller's charge.
type Battery struct {
	Name     string // the controller's input device name, or the supply's
	Supply   string // the power supply's directory name
	Level    string // capacity_level, e.g. "Normal", "Low", "Critical"
	Status   string // e.g. "Discharging", "Charging", "Full"
	Capacity int    // percent, or -1 when the driver only reports Level
}

// Charging is true while plugged in, so low-battery warnings can wait.
func (b Battery) Charging() bool {
	return b.Status == "Charging" || b.Status == "Full"
}

// Read lists the device batteries under root, sorted by name.
func Read(root string) []Battery {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Battery
	for _, d := range dirs {
		if b, ok := readOne(filepath.Join(root, d.Name())); ok {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readOne(dir string) (Battery, bool) {
	if attr(dir, "scope") != "Device" || attr(dir, "type") != "Battery" {
		return Battery{}, false
	}
	b := Battery{
		Supply:   filepath.Base(dir),
		Level:    attr(dir, "capacity_level"),
		Status:   attr(dir, "status"),
		Capacity: -1,
	}
	if n, err := strconv.Atoi(attr(dir, "capacity")); err == nil {
		b.Capacity = n
	}
	b.Name = inputName(dir)
	if b.Name == "" {
		b.Name = b.Supply
	}
	return b, true
}

func attr(dir, name string) string {
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// inputName is the name of the input device beside the supply on the same
// HID device: device/input/input*/name.
func inputName(dir string) string {
	names, _ := filepath.Glob(filepath.Join(dir, "device", "input", "input*", "name"))
	for _, n := range names {
		if raw, err := os.ReadFile(n); err == nil {
			if s := strings.TrimSpace(string(raw)); s != "" {
				return s
			}
		}
	}
	return ""
}
