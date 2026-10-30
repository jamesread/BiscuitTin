package x11win

import (
	"os"
	"testing"
)

func TestParseSession(t *testing.T) {
	for stat, want := range map[string]int{
		"1234 (sleep) S 1 1234 1234 0 -1 4194560":         1234,
		"77 (Web Content (x)) S 70 70 65 0 -1 4194560 12": 65,
	} {
		got, ok := ParseSession(stat)
		if !ok || got != want {
			t.Errorf("ParseSession(%q) = %d, %v; want %d", stat, got, ok, want)
		}
	}
	for _, bad := range []string{"", "1 (x) S 1", "no parens here"} {
		if _, ok := ParseSession(bad); ok {
			t.Errorf("ParseSession(%q) should fail", bad)
		}
	}
}

func TestParseSessionSelf(t *testing.T) {
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Skip(err)
	}
	sid, ok := ParseSession(string(raw))
	if !ok || sid <= 0 {
		t.Fatalf("own session = %d, %v", sid, ok)
	}
}

func TestMatch(t *testing.T) {
	sessions := map[int]int{10: 100, 11: 100, 20: 200}
	sessionOf := func(pid int) (int, bool) { s, ok := sessions[pid]; return s, ok }
	wins := []Window{
		{ID: 1, PID: 10, Class: "jellyfin-desktop Jellyfin"},
		{ID: 2, PID: 20, Class: "retroarch RetroArch"},
		{ID: 3, PID: 11, Class: "jellyfin-desktop Jellyfin"},
		{ID: 4, PID: 0, Class: "steamwebhelper steam"},
	}
	cases := []struct {
		name     string
		sessions []int
		class    string
		want     uint32
		ok       bool
	}{
		{"newest window in session", []int{100}, "", 3, true},
		{"session beats class", []int{200}, "steam", 2, true},
		{"class fallback, any case", []int{999}, "STEAM", 4, true},
		{"nothing", []int{999}, "", 0, false},
		{"unknown class", nil, "kodi", 0, false},
	}
	for _, c := range cases {
		got, ok := Match(wins, c.sessions, c.class, sessionOf)
		if ok != c.ok || uint32(got) != c.want {
			t.Errorf("%s: got %d, %v; want %d, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
