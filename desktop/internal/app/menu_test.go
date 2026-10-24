package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/jamesread/biscuittin/internal/settings"
)

func menuTitles(items []menuItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.title
	}
	return out
}

func TestMoreMenuIncludesSystem(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	got := menuTitles(u.moreMenu())
	if !slices.Contains(got, "System") {
		t.Fatalf("more menu missing System: %v", got)
	}
	if got[len(got)-1] != "Back" || got[len(got)-2] != "System" {
		t.Fatalf("System should be last before Back: %v", got)
	}
}

func TestSystemMenuItems(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	got := menuTitles(u.systemMenu())
	want := []string{"Exit", "Reboot", "Power off", "Back"}
	if !slices.Equal(got, want) {
		t.Fatalf("system menu = %v; want %v", got, want)
	}
}

func TestSystemExitSetsQuit(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	u.openMenu(screenSystem)
	u.handleMenu(actionActivate) // Exit is first
	if !u.quit {
		t.Fatal("Exit should request quit")
	}
}

func TestRebootAndPoweroffNeedConfirm(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	u.openMenu(screenSystem)
	u.menuSel = 1
	u.handleMenu(actionActivate)
	if u.screen != screenConfirm || u.menuSel != 0 || u.confirmTitle != "Reboot this machine?" {
		t.Fatalf("reboot confirm: screen=%d sel=%d title=%q", u.screen, u.menuSel, u.confirmTitle)
	}
	if titles := menuTitles(u.confirmMenu()); !slices.Equal(titles, []string{"Cancel", "Reboot"}) {
		t.Fatalf("reboot confirm items = %v", titles)
	}

	u.openMenu(screenSystem)
	u.menuSel = 2
	u.handleMenu(actionActivate)
	if u.screen != screenConfirm || u.menuSel != 0 || u.confirmTitle != "Power off this machine?" {
		t.Fatalf("poweroff confirm: screen=%d sel=%d title=%q", u.screen, u.menuSel, u.confirmTitle)
	}
	if titles := menuTitles(u.confirmMenu()); !slices.Equal(titles, []string{"Cancel", "Power off"}) {
		t.Fatalf("poweroff confirm items = %v", titles)
	}
}

func TestHomeEscPromptsExit(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	u.promptExit()
	if u.screen != screenConfirm || u.confirmTitle != "Exit biscuittin?" || u.menuSel != 0 {
		t.Fatalf("exit prompt: screen=%d title=%q sel=%d", u.screen, u.confirmTitle, u.menuSel)
	}
	if titles := menuTitles(u.confirmMenu()); !slices.Equal(titles, []string{"Cancel", "Exit"}) {
		t.Fatalf("exit confirm items = %v", titles)
	}

	u.handleMenu(actionActivate) // Cancel
	if u.screen != screenHome || u.quit {
		t.Fatalf("cancel should return home, screen=%d quit=%v", u.screen, u.quit)
	}

	u.promptExit()
	u.menuSel = 1
	u.handleMenu(actionActivate)
	if !u.quit {
		t.Fatal("confirming Exit should request quit")
	}
}

func TestConfirmEscFromHomeReturnsHome(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	u.promptExit()
	u.handleMenu(actionExit)
	if u.screen != screenHome || u.quit {
		t.Fatalf("Esc on exit prompt: screen=%d quit=%v", u.screen, u.quit)
	}
}

func TestConfirmCancelReturnsToSystem(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	var ran bool
	u.openConfirm("Reboot this machine?", "Reboot", func() { ran = true })
	u.handleMenu(actionActivate) // Cancel
	if u.screen != screenSystem || ran {
		t.Fatalf("cancel: screen=%d ran=%v", u.screen, ran)
	}
}

func TestConfirmRunsHostCommand(t *testing.T) {
	orig := runHostCommand
	t.Cleanup(func() { runHostCommand = orig })
	var got []string
	runHostCommand = func(argv []string) error {
		got = append([]string(nil), argv...)
		return nil
	}

	u := &ui{settings: &settings.Settings{}}
	u.openMenu(screenSystem)
	u.menuSel = 1
	u.handleMenu(actionActivate)
	u.menuSel = 1
	u.handleMenu(actionActivate)
	if !slices.Equal(got, []string{"systemctl", "reboot"}) {
		t.Fatalf("reboot ran %v", got)
	}

	got = nil
	u.openMenu(screenSystem)
	u.menuSel = 2
	u.handleMenu(actionActivate)
	u.menuSel = 1
	u.handleMenu(actionActivate)
	if !slices.Equal(got, []string{"systemctl", "poweroff"}) {
		t.Fatalf("poweroff ran %v", got)
	}
}

func TestConfirmCommandFailureShowsSystem(t *testing.T) {
	orig := runHostCommand
	t.Cleanup(func() { runHostCommand = orig })
	runHostCommand = func([]string) error { return errors.New("denied") }
	u := &ui{settings: &settings.Settings{}}
	u.openConfirm("Reboot this machine?", "Reboot", u.rebootHost)
	u.menuSel = 1
	u.handleMenu(actionActivate)
	if u.screen != screenSystem {
		t.Fatalf("failed command should return to System, screen=%d", u.screen)
	}
	if u.toast == "" {
		t.Fatal("failed command should toast")
	}
}
