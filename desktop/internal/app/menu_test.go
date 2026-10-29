package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/integrations"
	"github.com/jamesread/biscuittin/internal/settings"
)

type stubSource struct{ id, title string }

func (s stubSource) ID() string    { return s.id }
func (s stubSource) Title() string { return s.title }
func (s stubSource) Where() string { return "" }
func (stubSource) List() ([]integrations.ListedAction, error) {
	return nil, nil
}
func (stubSource) Run(integrations.PinnedAction, chan<- integrations.Event) {}

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
	if got[0] != "Integrations" {
		t.Fatalf("Integrations should be first: %v", got)
	}
	if !slices.Contains(got, "System") {
		t.Fatalf("more menu missing System: %v", got)
	}
	if got[len(got)-1] != "Back" || got[len(got)-2] != "System" {
		t.Fatalf("System should be last before Back: %v", got)
	}
}

func TestIntegrationsMenuUnconfigured(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	items := u.integrationsMenu()
	want := []string{"OliveTin actions", "Home Assistant automations", "Back"}
	if !slices.Equal(menuTitles(items), want) {
		t.Fatalf("titles = %v; want %v", menuTitles(items), want)
	}
	if !items[0].disabled || items[0].value() != "Needs YAML config" {
		t.Fatalf("OliveTin: disabled=%v value=%q", items[0].disabled, items[0].value())
	}
	if !items[1].disabled || items[1].value() != "Needs YAML config" {
		t.Fatalf("Home Assistant: disabled=%v value=%q", items[1].disabled, items[1].value())
	}
}

func TestIntegrationsDisabledActivateStays(t *testing.T) {
	u := &ui{settings: &settings.Settings{}}
	u.openMenu(screenIntegrations)
	u.handleMenu(actionActivate)
	if u.screen != screenIntegrations {
		t.Fatalf("disabled item opened screen %d", u.screen)
	}
}

func menuItemByTitle(items []menuItem, title string) (menuItem, bool) {
	for _, it := range items {
		if it.title == title {
			return it, true
		}
	}
	return menuItem{}, false
}

func TestSettingsGroupsAndAddress(t *testing.T) {
	u := &ui{
		cfg:        &config.Config{},
		settings:   &settings.Settings{},
		listenAddr: "192.0.2.10:8137",
	}
	items := u.settingsMenu()
	got := menuTitles(items)
	want := []string{
		"Backgrounds", "Background images", "Next image every", "Image attribution",
		"RSS", "RSS headlines", "Next headline every",
		"Display", "Control help",
		"Web control", "Web control", "QR code on home screen", "Address",
		"Back",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("settings = %v; want %v", got, want)
	}
	if !items[0].heading || items[0].selectable() {
		t.Fatal("group title should be a heading")
	}
	addr, ok := menuItemByTitle(items, "Address")
	if !ok || !addr.disabled || addr.value() != "192.0.2.10:8137" {
		t.Fatalf("address ok=%v disabled=%v value=%q", ok, addr.disabled, addr.value())
	}
}

func TestOpenSettingsSkipsHeading(t *testing.T) {
	u := &ui{cfg: &config.Config{}, settings: &settings.Settings{}}
	u.openMenu(screenSettings)
	items := u.settingsMenu()
	if items[u.menuSel].heading || items[u.menuSel].title != "Background images" {
		t.Fatalf("sel=%d title=%q", u.menuSel, items[u.menuSel].title)
	}
}

func TestNextSelectableSkipsHeading(t *testing.T) {
	items := []menuItem{menuHeading("G"), {title: "A"}, menuHeading("H"), {title: "B"}}
	if i := firstSelectable(items); i != 1 {
		t.Fatalf("first = %d", i)
	}
	if i := nextSelectable(items, 1, 1); i != 3 {
		t.Fatalf("down = %d", i)
	}
	if i := nextSelectable(items, 1, -1); i != 1 {
		t.Fatalf("up past heading = %d", i)
	}
}

func TestMenuAlignRowKeepsSelectionOnScreen(t *testing.T) {
	if origin := menuAlignRow(900, 974, 270, 960); origin != 256 {
		t.Fatalf("scroll up origin = %v", origin)
	}
	if origin := menuAlignRow(200, 274, 270, 960); origin != 340 {
		t.Fatalf("keep top origin = %v", origin)
	}
}

func TestSettingsQRNeedsWebControl(t *testing.T) {
	u := &ui{settings: &settings.Settings{QRPlace: settings.QRIdle}}
	item, ok := menuItemByTitle(u.settingsMenu(), "QR code on home screen")
	if !ok {
		t.Fatal("settings missing QR code")
	}
	if !item.disabled || item.value() != "Needs web control" {
		t.Fatalf("disabled=%v value=%q", item.disabled, item.value())
	}
	u.settings.WebControl = true
	item, ok = menuItemByTitle(u.settingsMenu(), "QR code on home screen")
	if !ok || item.disabled || item.radio == nil {
		t.Fatalf("web on should show QR radio, ok=%v disabled=%v radio=%v", ok, item.disabled, item.radio != nil)
	}
}

func TestSettingsRSSUnconfigured(t *testing.T) {
	u := &ui{cfg: &config.Config{}, settings: &settings.Settings{RSSEnabled: true}}
	item, ok := menuItemByTitle(u.settingsMenu(), "RSS headlines")
	if !ok {
		t.Fatal("settings missing RSS headlines")
	}
	if !item.disabled || item.value() != "Needs YAML config" {
		t.Fatalf("disabled=%v value=%q", item.disabled, item.value())
	}
}

func TestSettingsRSSConfiguredToggle(t *testing.T) {
	u := &ui{
		cfg:      &config.Config{RSS: config.RSSConfig{Feeds: []config.RSSFeed{{URL: "https://example.com/rss.xml"}}}},
		settings: &settings.Settings{RSSEnabled: true},
	}
	item, ok := menuItemByTitle(u.settingsMenu(), "RSS headlines")
	if !ok || item.disabled || item.radio == nil {
		t.Fatalf("configured RSS should be a toggle, ok=%v disabled=%v radio=%v", ok, item.disabled, item.radio != nil)
	}
}

func TestIntegrationsConfiguredOpensList(t *testing.T) {
	u := &ui{
		settings: &settings.Settings{},
		sources:  []integrations.Source{stubSource{id: "olivetin", title: "OliveTin actions"}},
		lists:    map[string]*uiActionList{},
	}
	items := u.integrationsMenu()
	if items[0].disabled {
		t.Fatal("configured OliveTin should be enabled")
	}
	u.openMenu(screenIntegrations)
	u.handleMenu(actionActivate)
	if u.screen != screenActionList || u.listSource != "olivetin" {
		t.Fatalf("screen=%d source=%s", u.screen, u.listSource)
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
	if u.screen != screenConfirm || u.confirmTitle != "Exit BiscuitTin?" || u.menuSel != 0 {
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
