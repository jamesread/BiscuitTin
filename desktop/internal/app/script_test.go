package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steps")
	src := "# comment\n\nclock 20:15\ninput xbox\npress right\nsleep 0.5\nrss seed/rss/article.json\nshot out/home.png\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := loadScript(path)
	if err != nil {
		t.Fatal(err)
	}
	var cmds []string
	for _, st := range sc.steps {
		cmds = append(cmds, st.cmd+" "+st.arg)
	}
	if got := strings.Join(cmds, "|"); got != "clock 20:15|input xbox|press right|sleep 0.5|rss seed/rss/article.json|shot out/home.png" {
		t.Errorf("steps = %s", got)
	}
}

func TestLoadScriptRejectsBadSteps(t *testing.T) {
	for _, line := range []string{"press jump", "clock 8pm", "input wiimote", "sleep soon", "shot", "rss", "dance"} {
		path := filepath.Join(t.TempDir(), "steps")
		if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadScript(path); err == nil || !strings.Contains(err.Error(), ":1:") {
			t.Errorf("%q: err = %v; want an error naming line 1", line, err)
		}
	}
}

func TestParseSize(t *testing.T) {
	if p, err := ParseSize("1920x1080"); err != nil || p != (image.Point{X: 1920, Y: 1080}) {
		t.Errorf("parseSize = %v, %v", p, err)
	}
	for _, bad := range []string{"", "1920", "0x10", "axb"} {
		if _, err := ParseSize(bad); err == nil {
			t.Errorf("parseSize(%q) should fail", bad)
		}
	}
}
