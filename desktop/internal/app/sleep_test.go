package app

import (
	"slices"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/settings"
)

func stubHostCommands(t *testing.T) *[][]string {
	t.Helper()
	orig := runHostCommand
	t.Cleanup(func() { runHostCommand = orig })
	var ran [][]string
	runHostCommand = func(argv []string) error {
		ran = append(ran, argv)
		return nil
	}
	return &ran
}

func TestStepSleep(t *testing.T) {
	if got := stepSleep(0, 1); got != 30 {
		t.Fatalf("Off + 1 = %d", got)
	}
	if got := stepSleep(120, 1); got != 120 {
		t.Fatalf("120 + 1 should clamp, got %d", got)
	}
	if got := stepSleep(0, -1); got != 0 {
		t.Fatalf("Off - 1 should clamp, got %d", got)
	}
	if got := stepSleep(120, 0); got != 0 {
		t.Fatalf("A on 120 should cycle to Off, got %d", got)
	}
}

func TestSleepLabel(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	now := time.Unix(1000, 0)
	if got := u.sleepLabel(now); got != "Off" {
		t.Fatal(got)
	}
	u.sleep.set(30, now)
	if got := u.sleepLabel(now.Add(90 * time.Second)); got != "30 min (29 left)" {
		t.Fatal(got)
	}
}

func TestSleepCountdownAndRun(t *testing.T) {
	ran := stubHostCommands(t)
	s := testSession([]config.Entry{{Title: "A", Command: []string{"a"}}})
	now := time.Unix(1000, 0)
	s.u.sleep.set(30, now)

	s.checkSleep(now.Add(28 * time.Minute))
	if s.u.screen == screenSleep {
		t.Fatal("countdown too early")
	}
	s.checkSleep(now.Add(29*time.Minute + 15*time.Second))
	if s.u.screen != screenSleep {
		t.Fatalf("screen = %d, want the countdown", s.u.screen)
	}
	if got := s.u.sleepTitle(now.Add(29*time.Minute + 15*time.Second)); got != "Suspending in 45 s" {
		t.Fatal(got)
	}
	if len(*ran) != 0 {
		t.Fatal("nothing should run before the end")
	}
	s.checkSleep(now.Add(30 * time.Minute))
	if len(*ran) != 1 || !slices.Equal((*ran)[0], []string{"systemctl", "suspend"}) {
		t.Fatalf("ran %v", *ran)
	}
	if s.u.sleep.active() {
		t.Fatal("timer should be cleared once it has run")
	}
}

func TestSleepPowerOff(t *testing.T) {
	ran := stubHostCommands(t)
	s := testSession([]config.Entry{{Title: "A", Command: []string{"a"}}})
	s.u.settings.SleepPowerOff = true
	now := time.Now()
	s.u.sleep.set(30, now)
	s.checkSleep(now.Add(time.Hour))
	if len(*ran) != 1 || !slices.Equal((*ran)[0], []string{"systemctl", "poweroff"}) {
		t.Fatalf("ran %v", *ran)
	}
}

func TestSleepCancelWithBack(t *testing.T) {
	ran := stubHostCommands(t)
	s := testSession([]config.Entry{{Title: "A", Command: []string{"a"}}})
	now := time.Now()
	s.u.sleep.set(30, now)
	s.checkSleep(now.Add(29*time.Minute + 30*time.Second))
	s.u.handleMenu(actionBack)
	if s.u.sleep.active() || s.u.screen != screenHome {
		t.Fatalf("Back should cancel: active=%v screen=%d", s.u.sleep.active(), s.u.screen)
	}
	s.checkSleep(now.Add(time.Hour))
	if len(*ran) != 0 {
		t.Fatalf("cancelled timer ran %v", *ran)
	}
	if got := menuTitles(s.u.sleepMenu()); !slices.Equal(got, []string{"Cancel timer", "Suspend now"}) {
		t.Fatal(got)
	}
}
