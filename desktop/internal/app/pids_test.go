package app

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/launch"
	"github.com/jamesread/biscuittin/internal/settings"
)

func TestRemovePID(t *testing.T) {
	got := removePID([]int{1, 2, 3}, 2)
	if !slices.Equal(got, []int{1, 3}) {
		t.Fatalf("removePID = %v", got)
	}
	if got := removePID([]int{7}, 7); len(got) != 0 {
		t.Fatalf("remove last = %v", got)
	}
}

func TestHasLivePID(t *testing.T) {
	if hasLivePID(nil) || hasLivePID([]int{0, -3}) {
		t.Fatal("empty or invalid pids should not be live")
	}
	if !hasLivePID([]int{os.Getpid()}) {
		t.Fatal("current pid should be live")
	}
}

func startSleep(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

func testSession(entries []config.Entry) *session {
	cfg := &config.Config{Entries: entries}
	u := &ui{
		cfg:      cfg,
		settings: &settings.Settings{},
		pids:     make([][]int, len(entries)),
		tiles:    []tile{{kind: tileEntry, entry: 0, title: entries[0].Title}},
	}
	return &session{
		cfg:          cfg,
		u:            u,
		started:      map[int]time.Time{},
		exits:        make(chan launch.Exit, 4),
		hideOnLaunch: false,
	}
}

func TestLaunchEntryPromptsWhenPIDAlive(t *testing.T) {
	cmd := startSleep(t)
	s := testSession([]config.Entry{{Title: "Sleep", Command: []string{"sleep", "30"}}})
	s.u.pids[0] = []int{cmd.Process.Pid}

	s.launchEntry(s.u.tiles[0])
	if s.u.screen != screenRunning {
		t.Fatalf("screen=%d; want running prompt", s.u.screen)
	}
	if s.u.runningTitle != "Sleep is running" {
		t.Fatalf("title %q", s.u.runningTitle)
	}
	got := menuTitles(s.u.runningMenu())
	want := []string{"Cancel", "Start another instance", "Stop"}
	if !slices.Equal(got, want) {
		t.Fatalf("menu = %v; want %v", got, want)
	}
	if s.u.tileRunning(s.u.tiles[0]) != true {
		t.Fatal("tile should show running")
	}
}

func TestRunningMenuCancel(t *testing.T) {
	s := testSession([]config.Entry{{Title: "Sleep"}})
	s.u.openRunning("Sleep", func() {}, func() {})
	s.u.handleMenu(actionActivate)
	if s.u.screen != screenHome {
		t.Fatalf("cancel should return home, screen=%d", s.u.screen)
	}
}

func TestKillEntryStopsProcess(t *testing.T) {
	cmd := startSleep(t)
	s := testSession([]config.Entry{{Title: "Sleep"}})
	s.u.pids[0] = []int{cmd.Process.Pid}
	s.u.screen = screenRunning
	s.killEntry(0)
	if s.u.screen != screenHome {
		t.Fatalf("screen=%d", s.u.screen)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("killEntry did not stop sleep")
	}
	if launch.Alive(cmd.Process.Pid) {
		t.Fatal("pid still alive")
	}
}

func TestLaunchEntryStartsWhenPIDDead(t *testing.T) {
	dir := t.TempDir()
	s := testSession([]config.Entry{{Title: "True", Command: []string{"true"}}})
	s.cfg.LogDir = dir
	s.u.pids[0] = []int{999999}

	s.launchEntry(s.u.tiles[0])
	if s.u.screen == screenRunning {
		t.Fatal("dead pid should not prompt")
	}
	if len(s.u.pids[0]) == 0 {
		t.Fatal("start should record a pid")
	}
}
