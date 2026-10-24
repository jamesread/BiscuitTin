package app

import (
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/launch"
	"github.com/jamesread/biscuittin/internal/settings"
	"go.uber.org/zap"
)

type screen int

const (
	screenHome screen = iota
	screenMore
	screenSettings
	screenIntegrations
	screenGamepads
	screenDiagnostics
	screenActionList
	screenSystem
	screenConfirm
	screenRunning
)

// menuItem is one row of a list menu. It is a link (target, or open for
// screens that need setup), a radio group, or a stepped value whose adjust
// gets -1/+1 from left/right (clamped) and 0 from A (cycles round).
type menuItem struct {
	open   func()
	value  func() string
	adjust func(dir int)
	radio  *radioGroup
	title  string
	target screen
}

func (u *ui) moreMenu() []menuItem {
	var items []menuItem
	if len(u.sources) > 0 {
		items = append(items, menuItem{
			title: "Integrations",
			open:  func() { u.openMenu(screenIntegrations) },
			value: func() string { return fmt.Sprintf("%d pinned", len(u.settings.PinnedActions)) },
		})
	}
	return append(items, []menuItem{
		{title: "Settings", open: func() { u.openMenu(screenSettings) }},
		{title: "Gamepad info", target: screenGamepads},
		{title: "Diagnostics", target: screenDiagnostics},
		{title: "System", open: func() { u.openMenu(screenSystem) }},
		{title: "Back", target: screenHome},
	}...)
}

func (u *ui) systemMenu() []menuItem {
	return []menuItem{
		{title: "Exit", open: func() { u.quit = true }},
		{title: "Reboot", open: func() { u.openConfirm("Reboot this machine?", "Reboot", u.rebootHost) }},
		{title: "Power off", open: func() { u.openConfirm("Power off this machine?", "Power off", u.poweroffHost) }},
		{title: "Back", open: func() { u.openMenu(screenMore) }},
	}
}

func (u *ui) openConfirm(title, actionTitle string, do func()) {
	u.openConfirmFrom(screenSystem, title, actionTitle, do)
}

func (u *ui) openConfirmFrom(back screen, title, actionTitle string, do func()) {
	u.confirmTitle = title
	u.confirmLabel = actionTitle
	u.confirmDo = do
	u.confirmBack = back
	u.screen = screenConfirm
	u.menuSel = 0 // Cancel
}

func (u *ui) promptExit() {
	u.openConfirmFrom(screenHome, "Exit BiscuitTin?", "Exit", func() { u.quit = true })
}

func (u *ui) leaveConfirm() {
	if u.confirmBack == screenHome {
		u.screen = screenHome
		return
	}
	u.openMenu(u.confirmBack)
}

func (u *ui) confirmMenu() []menuItem {
	return []menuItem{
		{title: "Cancel", open: u.leaveConfirm},
		{title: u.confirmLabel, open: u.confirmDo},
	}
}

func (u *ui) openRunning(title string, kill, again func()) {
	u.runningTitle = title + " is running"
	u.runningKill = kill
	u.runningAgain = again
	u.screen = screenRunning
	u.menuSel = 0
}

func (u *ui) runningMenu() []menuItem {
	return []menuItem{
		{title: "Cancel", open: func() { u.screen = screenHome }},
		{title: "Start another instance", open: u.runningAgain},
		{title: "Stop", open: u.runningKill},
	}
}

// runHostCommand starts a host command (via flatpak-spawn in the sandbox).
// Tests replace this.
var runHostCommand = func(argv []string) error {
	return launch.Host(argv).Run()
}

func (u *ui) rebootHost() {
	u.runSystem([]string{"systemctl", "reboot"}, "Could not reboot")
}

func (u *ui) poweroffHost() {
	u.runSystem([]string{"systemctl", "poweroff"}, "Could not power off")
}

func (u *ui) runSystem(argv []string, fail string) {
	lg().Info("system command", zap.Strings("argv", argv))
	if err := runHostCommand(argv); err != nil {
		lg().Warn("system command failed", zap.Strings("argv", argv), zap.Error(err))
		u.setToast(fail+": "+err.Error(), colorBad, 6)
		u.openMenu(screenSystem)
	}
}

// integrationsMenu lists each configured integration's pin list.
func (u *ui) integrationsMenu() []menuItem {
	var items []menuItem
	for _, src := range u.sources {
		items = append(items, menuItem{
			title: src.Title(),
			open:  func() { u.openActionList(src.ID()) },
			value: func() string { return fmt.Sprintf("%d pinned", u.pinnedCount(src.ID())) },
		})
	}
	return append(items, menuItem{title: "Back", open: func() { u.openMenu(screenMore) }})
}

func intervalAdjust(dst *int, changed func()) func(int) {
	return func(dir int) {
		next := settings.StepInterval(*dst, dir)
		if dir == 0 {
			next = settings.StepInterval(*dst, 1)
			if next == *dst {
				next = settings.Intervals[0]
			}
		}
		*dst = next
		changed()
	}
}

func (u *ui) settingsMenu() []menuItem {
	st := u.settings
	changed := func() { u.settingsChanged = true }
	return []menuItem{
		{
			title: "Background images",
			radio: boolRadio("On", "Off", func() bool { return st.BackgroundsEnabled }, func(v bool) { st.BackgroundsEnabled = v; changed() }),
		},
		{
			title:  "Next image every",
			value:  func() string { return settings.IntervalLabel(st.BackgroundInterval) },
			adjust: intervalAdjust(&st.BackgroundInterval, changed),
		},
		{
			title:  "Next headline every",
			value:  func() string { return settings.IntervalLabel(st.RSSInterval) },
			adjust: intervalAdjust(&st.RSSInterval, changed),
		},
		{
			title: "Image attribution",
			radio: boolRadio("Shown", "Hidden", func() bool { return !st.HideAttribution }, func(v bool) { st.HideAttribution = !v; changed() }),
		},
		{
			title: "Control help",
			radio: boolRadio("Shown", "Hidden", func() bool { return !st.HideHelp }, func(v bool) { st.HideHelp = !v; changed() }),
		},
		{
			title: "Web control",
			radio: boolRadio("On", "Off", func() bool { return st.WebControl }, func(v bool) { st.WebControl = v; changed() }),
		},
		{
			title: "QR code on home screen",
			radio: boolRadio("Shown", "Hidden", func() bool { return st.ShowQR }, func(v bool) { st.ShowQR = v; changed() }),
		},
		{title: "Back", open: func() { u.openMenu(screenMore) }},
	}
}

// menuFor returns the items and title of a list-menu screen.
func (u *ui) menuFor(sc screen) (string, []menuItem, screen) {
	if title, items, parent, ok := u.listMenu(sc); ok {
		return title, items, parent
	}
	return "More", u.moreMenu(), screenHome
}

func (u *ui) listMenu(sc screen) (string, []menuItem, screen, bool) {
	if sc == screenSettings {
		return "Settings", u.settingsMenu(), screenMore, true
	}
	if sc == screenIntegrations {
		return "Integrations", u.integrationsMenu(), screenMore, true
	}
	if sc == screenSystem {
		return "System", u.systemMenu(), screenMore, true
	}
	return u.promptMenu(sc)
}

func (u *ui) promptMenu(sc screen) (string, []menuItem, screen, bool) {
	if sc == screenConfirm {
		return u.confirmTitle, u.confirmMenu(), u.confirmBack, true
	}
	if sc == screenRunning {
		return u.runningTitle, u.runningMenu(), screenHome, true
	}
	return "", nil, 0, false
}

func (u *ui) openMenu(sc screen) {
	u.screen = sc
	u.menuSel = 0
}

func (u *ui) openMore() { u.openMenu(screenMore) }

var gamepadButtonNames = map[int32]string{
	rl.GamepadButtonLeftFaceUp:     "Up",
	rl.GamepadButtonLeftFaceRight:  "Right",
	rl.GamepadButtonLeftFaceDown:   "Down",
	rl.GamepadButtonLeftFaceLeft:   "Left",
	rl.GamepadButtonRightFaceUp:    "Y",
	rl.GamepadButtonRightFaceRight: "B",
	rl.GamepadButtonRightFaceDown:  "A",
	rl.GamepadButtonRightFaceLeft:  "X",
	rl.GamepadButtonLeftTrigger1:   "LB",
	rl.GamepadButtonLeftTrigger2:   "LT",
	rl.GamepadButtonRightTrigger1:  "RB",
	rl.GamepadButtonRightTrigger2:  "RT",
	rl.GamepadButtonMiddleLeft:     "Back",
	rl.GamepadButtonMiddle:         "Guide",
	rl.GamepadButtonMiddleRight:    "Start",
	rl.GamepadButtonLeftThumb:      "LS",
	rl.GamepadButtonRightThumb:     "RS",
}

// handleMenu processes an action on a non-home screen.
var listMenus = map[screen]bool{
	screenMore:         true,
	screenSettings:     true,
	screenIntegrations: true,
	screenSystem:       true,
	screenConfirm:      true,
	screenRunning:      true,
}

func (u *ui) handleMenu(a action) {
	if listMenus[u.screen] {
		u.handleListMenu(a)
		return
	}
	if u.screen == screenActionList {
		u.handleActionList(a)
		return
	}
	// On gamepads, B is a button worth inspecting, so only keyboard/remote
	// Back and the A button leave. Diagnostics uses the same keys.
	u.leaveInfoScreen(a)
}

func (u *ui) leaveInfoScreen(a action) {
	switch a {
	case actionBack, actionExit, actionActivate:
		u.screen = screenMore
	}
}

func (u *ui) handleListMenu(a action) {
	_, items, parent := u.menuFor(u.screen)
	u.menuSel = clampInt(u.menuSel, len(items)-1)
	switch a {
	case actionBack, actionExit:
		u.leaveMenu(parent)
	default:
		u.applyListNav(a, items)
	}
}

func (u *ui) applyListNav(a action, items []menuItem) {
	item := items[u.menuSel]
	switch a {
	case actionUp, actionDown:
		u.moveMenuSel(a, len(items))
	case actionLeft, actionRight:
		adjustMenuItem(item, a)
	case actionActivate:
		u.activateMenuItem(item)
	}
}

func (u *ui) moveMenuSel(a action, n int) {
	if a == actionUp {
		u.menuSel = max(u.menuSel-1, 0)
		return
	}
	u.menuSel = min(u.menuSel+1, n-1)
}

func adjustMenuItem(item menuItem, a action) {
	dir := 1
	if a == actionLeft {
		dir = -1
	}
	if item.radio != nil {
		item.radio.step(dir)
		return
	}
	if item.adjust != nil {
		item.adjust(dir)
	}
}

func (u *ui) activateMenuItem(item menuItem) {
	if item.radio != nil {
		item.radio.cycle()
		return
	}
	if item.adjust != nil {
		item.adjust(0)
		return
	}
	if item.open != nil {
		item.open()
		return
	}
	u.screen = item.target
}

func (u *ui) leaveMenu(parent screen) {
	if parent == screenHome {
		u.screen = screenHome
		return
	}
	u.openMenu(parent)
}

func (u *ui) drawMenu(w, h, s, margin float32) {
	if listMenus[u.screen] {
		title, items, _ := u.menuFor(u.screen)
		u.drawListMenu(title, items, w, s, margin)
		return
	}
	u.drawInfoScreen(w, h, s, margin)
}

func (u *ui) drawInfoScreen(w, h, s, margin float32) {
	if u.screen == screenGamepads {
		u.drawGamepads(w, s, margin)
		return
	}
	if u.screen == screenDiagnostics {
		u.drawDiagnostics(w, h, s, margin)
		return
	}
	u.drawActionList(w, h, s, margin)
}

func (u *ui) drawListMenu(title string, items []menuItem, w, s, margin float32) {
	u.text(u.fitText(title, 52*s, w-2*margin), margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))
	itemW := w - 2*margin
	itemH := float32(74) * s
	y := 290 * s
	for i, item := range items {
		u.drawMenuItem(item, i == u.menuSel, margin, y, itemW, itemH, s)
		y += itemH + 14*s
	}
}

func menuItemColors(accent rl.Color, selected bool) (fill, textColor rl.Color) {
	if selected {
		return rl.ColorAlpha(accent, 0.22), rl.RayWhite
	}
	return rl.ColorAlpha(rl.Black, 0.3), rl.ColorAlpha(rl.RayWhite, 0.7)
}

func menuValueColor(accent rl.Color, selected bool) rl.Color {
	if selected {
		return accent
	}
	return rl.ColorAlpha(rl.RayWhite, 0.6)
}

func (u *ui) drawMenuItem(item menuItem, selected bool, x, y, itemW, itemH, s float32) {
	r := rl.Rectangle{X: x, Y: y, Width: itemW, Height: itemH}
	fill, textColor := menuItemColors(u.accent, selected)
	rl.DrawRectangleRounded(r, 0.25, 10, u.fa(fill))
	if selected {
		rl.DrawRectangleRoundedLinesEx(r, 0.25, 10, 4*s, u.fa(u.accent))
	}
	u.surface = &surfaceTint{contrast.RGBOf(fill), float64(fill.A) / 255}
	u.text(item.title, r.X+36*s, r.Y+(itemH-40*s)/2, 40*s, textColor)
	u.drawMenuValue(item, selected, r, itemH, s)
	u.surface = nil
}

func (u *ui) drawMenuValue(item menuItem, selected bool, r rl.Rectangle, itemH, s float32) {
	if item.radio != nil {
		rh := itemH - 22*s
		u.drawRadioGroup(item.radio, r.X+r.Width-18*s, r.Y+(itemH-rh)/2, rh, s, selected)
		return
	}
	if item.value != nil {
		u.drawMenuStepped(item, selected, r, itemH, s)
	}
}

func (u *ui) drawMenuStepped(item menuItem, selected bool, r rl.Rectangle, itemH, s float32) {
	v := item.value()
	if selected {
		if item.adjust != nil {
			v = "<   " + v + "   >"
		}
	}
	m := u.measure(v, 36*s)
	u.text(v, r.X+r.Width-36*s-m.X, r.Y+(itemH-36*s)/2, 36*s, menuValueColor(u.accent, selected))
}

func (u *ui) drawGamepads(w, s, margin float32) {
	u.text("Gamepad info", margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))
	y := 280 * s
	found := 0
	for g := range int32(maxGamepads) {
		if !rl.IsGamepadAvailable(g) {
			continue
		}
		found++
		y = u.drawGamepadCard(g, margin, y, w, s)
	}
	if found == 0 {
		u.text("No gamepads detected", margin, y, 40*s, rl.ColorAlpha(rl.RayWhite, 0.65))
	}
}

func gamepadLabel(g int32) string {
	name := rl.GetGamepadName(g)
	if name == "" {
		return "(unnamed)"
	}
	return name
}

func gamepadAxesLine(g int32) string {
	return fmt.Sprintf("Axes: %d     Left stick: %+.2f %+.2f     Right stick: %+.2f %+.2f     Triggers: %+.2f %+.2f",
		rl.GetGamepadAxisCount(g),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftX),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftY),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightX),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightY),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftTrigger),
		rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightTrigger))
}

func heldGamepadButtons(g int32) []string {
	var held []string
	for b := int32(rl.GamepadButtonLeftFaceUp); b <= rl.GamepadButtonRightThumb; b++ {
		if rl.IsGamepadButtonDown(g, b) {
			held = append(held, gamepadButtonNames[b])
		}
	}
	return held
}

func gamepadHeldText(g int32) string {
	held := heldGamepadButtons(g)
	if len(held) == 0 {
		return "Buttons held: none"
	}
	return "Buttons held: " + strings.Join(held, "  ")
}

func (u *ui) drawGamepadCard(g int32, margin, y, w, s float32) float32 {
	cardH := 190 * s
	r := rl.Rectangle{X: margin, Y: y, Width: w - 2*margin, Height: cardH}
	rl.DrawRectangleRounded(r, 0.12, 10, u.fa(rl.ColorAlpha(rl.White, 0.06)))
	u.text(fmt.Sprintf("%d   %s", g, gamepadLabel(g)), r.X+32*s, r.Y+24*s, 40*s, rl.RayWhite)
	u.text(u.fitText(gamepadAxesLine(g), 28*s, r.Width-64*s), r.X+32*s, r.Y+84*s, 28*s, rl.ColorAlpha(rl.RayWhite, 0.65))
	u.text(gamepadHeldText(g), r.X+32*s, r.Y+130*s, 28*s, u.accent)
	return y + cardH + 24*s
}

func clampInt(v, hi int) int {
	return min(max(v, 0), hi)
}
