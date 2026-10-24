package launch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestHostRunsOnHost(t *testing.T) {
	flatpakInfo = filepath.Join(t.TempDir(), "absent")
	cmd := Host([]string{"systemctl", "reboot"})
	if !slices.Equal(cmd.Args, []string{"systemctl", "reboot"}) {
		t.Errorf("args %q", cmd.Args)
	}
}

func TestCommandOnHost(t *testing.T) {
	flatpakInfo = filepath.Join(t.TempDir(), "absent")
	cmd := command([]string{"steam", "-gamepadui"}, "/tmp", map[string]string{"B": "2", "A": "1"})
	if !slices.Equal(cmd.Args, []string{"steam", "-gamepadui"}) || cmd.Dir != "/tmp" {
		t.Errorf("args %q, dir %q", cmd.Args, cmd.Dir)
	}
	if env := cmd.Env[len(cmd.Env)-2:]; !slices.Equal(env, []string{"A=1", "B=2"}) {
		t.Errorf("entry env should follow the inherited environment, sorted: %q", env)
	}
}

func TestCommandInFlatpakRunsOnHost(t *testing.T) {
	flatpakInfo = filepath.Join(t.TempDir(), ".flatpak-info")
	if err := os.WriteFile(flatpakInfo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := command([]string{"steam", "-gamepadui"}, "/tmp", map[string]string{"B": "2", "A": "1"})
	want := []string{"flatpak-spawn", "--host", "--directory=/tmp", "--env=A=1", "--env=B=2", "steam", "-gamepadui"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q; want %q", cmd.Args, want)
	}
	if cmd.Dir != "" || cmd.Env != nil {
		t.Error("the sandbox's directory and environment should not be used")
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Steam Big Picture": "steam-big-picture", "!!": "entry", "Kodi": "kodi"} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestAliveAndKill(t *testing.T) {
	if Alive(0) || Alive(-1) {
		t.Fatal("non-positive pids are not alive")
	}
	if !Alive(os.Getpid()) {
		t.Fatal("this process should be alive")
	}
	cmd := exec.CommandContext(context.Background(), "sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if !Alive(pid) {
		t.Fatalf("started pid %d should be alive", pid)
	}
	if err := Kill(pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("kill did not stop sleep")
	}
	if Alive(pid) {
		t.Fatalf("pid %d should be dead after kill", pid)
	}
}
