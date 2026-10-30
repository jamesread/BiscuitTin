package padbattery

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	root := t.TempDir()
	// A DualSense, with its input name beside it.
	ps := filepath.Join(root, "ps-controller-battery-aa:bb")
	write(t, filepath.Join(ps, "scope"), "Device")
	write(t, filepath.Join(ps, "type"), "Battery")
	write(t, filepath.Join(ps, "capacity"), "15")
	write(t, filepath.Join(ps, "status"), "Discharging")
	write(t, filepath.Join(ps, "device", "input", "input21", "name"), "DualSense Wireless Controller")
	// A pad that only reports a level, with no input name.
	lvl := filepath.Join(root, "xpad-battery-1")
	write(t, filepath.Join(lvl, "scope"), "Device")
	write(t, filepath.Join(lvl, "type"), "Battery")
	write(t, filepath.Join(lvl, "capacity_level"), "Low")
	// A laptop battery: no scope, so left out.
	bat := filepath.Join(root, "BAT0")
	write(t, filepath.Join(bat, "type"), "Battery")
	write(t, filepath.Join(bat, "capacity"), "95")

	got := Read(root)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Name != "DualSense Wireless Controller" || got[0].Capacity != 15 || got[0].Charging() {
		t.Errorf("dualsense = %+v", got[0])
	}
	if got[1].Name != "xpad-battery-1" || got[1].Capacity != -1 || got[1].Level != "Low" {
		t.Errorf("level-only = %+v", got[1])
	}
}

func TestReadMissingRoot(t *testing.T) {
	if got := Read(filepath.Join(t.TempDir(), "none")); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestReadThisMachine(t *testing.T) {
	for _, b := range Read(Root) {
		if b.Supply == "BAT0" || b.Supply == "AC" {
			t.Fatalf("system supply listed as a controller: %+v", b)
		}
	}
}
