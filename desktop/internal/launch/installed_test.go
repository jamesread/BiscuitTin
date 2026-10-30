package launch

import (
	"errors"
	"os/exec"
	"slices"
	"testing"
)

func TestProbeArgs(t *testing.T) {
	cases := []struct {
		cmd  []string
		want []string
	}{
		{[]string{"flatpak", "run", "--branch=stable", "org.jellyfin.JellyfinDesktop", "--tv"}, []string{"flatpak", "info", "org.jellyfin.JellyfinDesktop"}},
		{[]string{"retroarch"}, []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", "retroarch"}},
		{[]string{"/opt/kodi/bin/kodi", "--standalone"}, []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", "/opt/kodi/bin/kodi"}},
		{[]string{"flatpak", "run"}, nil},
		{nil, nil},
	}
	for _, c := range cases {
		if got := ProbeArgs(c.cmd); !slices.Equal(got, c.want) {
			t.Errorf("ProbeArgs(%q) = %q; want %q", c.cmd, got, c.want)
		}
	}
}

func TestInstalledReal(t *testing.T) {
	if ok, known := Installed([]string{"sh", "-c", "true"}); !ok || !known {
		t.Fatalf("sh: %v %v", ok, known)
	}
	if ok, known := Installed([]string{"biscuittin-no-such-command"}); ok || !known {
		t.Fatalf("missing command: %v %v", ok, known)
	}
}

func TestInstalledUnknownWhenProbeCannotRun(t *testing.T) {
	old := runProbe
	t.Cleanup(func() { runProbe = old })
	runProbe = func([]string) error { return exec.ErrNotFound }
	if ok, known := Installed([]string{"x"}); ok || known {
		t.Fatalf("got %v %v; want unknown", ok, known)
	}
	runProbe = func([]string) error { return errors.New("boom") }
	if _, known := Installed([]string{"x"}); known {
		t.Fatal("a probe error should be unknown")
	}
}
