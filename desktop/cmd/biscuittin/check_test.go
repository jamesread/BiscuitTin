package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheck(t *testing.T) {
	old := installed
	t.Cleanup(func() { installed = old })
	installed = func(cmd []string) (bool, bool) { return cmd[0] != "gone" && cmd[0] != "flatpak", true }

	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(good, []byte("entries:\n  - title: A\n    command: [a]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badYAML := "colums: 3\nentries:\n  - title: A\n    command: [gone]\n  - title: J\n    command: [flatpak, run, org.example.J]\n"
	if err := os.WriteFile(bad, []byte(badYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if code := runCheck([]string{"-config", good}, &out, &out); code != 0 || !strings.Contains(out.String(), "OK, 1 entries") {
		t.Fatalf("good: code %d, %q", code, out.String())
	}
	out.Reset()
	code := runCheck([]string{"-config", bad}, &out, &out)
	got := out.String()
	for _, want := range []string{"field colums not found", "entries[0] (A): command not found: gone", "Flatpak org.example.J is not installed"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if code != 1 {
		t.Fatalf("bad: code %d", code)
	}
}
