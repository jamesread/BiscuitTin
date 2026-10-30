package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jamesread/biscuittin/internal/cec"
	"github.com/jamesread/biscuittin/internal/config"
)

func entries(titles ...string) *config.Config {
	cfg := &config.Config{}
	for _, t := range titles {
		cfg.Entries = append(cfg.Entries, config.Entry{Title: t, Command: []string{"true"}})
	}
	return cfg
}

func TestRemapPIDsFollowsTitles(t *testing.T) {
	old := entries("Kodi", "Steam", "Gone")
	cfg := entries("New", "Steam", "Kodi")
	u := &ui{pids: [][]int{{10}, {20, 21}, {30}}}
	u.remapPIDs(old, cfg)
	want := [][]int{nil, {20, 21}, {10}}
	if !slices.EqualFunc(u.pids, want, slices.Equal) {
		t.Fatalf("pids = %v; want %v", u.pids, want)
	}
	if u.entryForPID(10) != 2 || u.entryForPID(30) != -1 {
		t.Fatalf("entryForPID 10 = %d, 30 = %d", u.entryForPID(10), u.entryForPID(30))
	}
	if remapEntry(old, cfg, -1) != -1 || remapEntry(old, cfg, 2) != -1 || remapEntry(old, cfg, 0) != 2 {
		t.Fatal("remapEntry")
	}
}

func TestConfigStampChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, ok := statConfig(path); ok {
		t.Fatal("missing file has a stamp")
	}
	if err := os.WriteFile(path, []byte("a: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := statConfig(path)
	if err := os.WriteFile(path, []byte("a: 12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := statConfig(path); a == b {
		t.Fatal("stamp should change with the file")
	}
}

func TestReloadKeepsConfigOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("entries: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := entries("Kodi")
	s := &session{cfg: cfg, configPath: path, u: &ui{cfg: cfg}}
	s.reloadConfig(true)
	if s.cfg != cfg || s.u.toast == "" || s.u.toastColor != colorBad {
		t.Fatalf("bad config: cfg replaced=%v toast=%q", s.cfg != cfg, s.u.toast)
	}
}

func TestCECActionNotesPress(t *testing.T) {
	r := &cec.Reader{Presses: make(chan cec.Press, 1)}
	s := &session{cec: r, in: &input{primary: deviceGamepad}}
	if s.cecAction() != actionNone {
		t.Fatal("no press should be no action")
	}
	r.Presses <- cec.Press{Name: "exit", Key: cec.KeyBack}
	if a := s.cecAction(); a != actionBack {
		t.Fatalf("exit = %v; want Back", a)
	}
	if s.in.primary != deviceKeyboard || !slices.Equal(s.in.recent, []string{"CEC exit"}) {
		t.Fatalf("primary = %v recent = %v", s.in.primary, s.in.recent)
	}
}
