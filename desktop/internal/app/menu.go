package app

import (
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/integrations"
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
// screens that need setup), a radio group, a stepped value whose adjust
// gets -1/+1 from left/right (clamped) and 0 from A (cycles round), or a
// heading that is drawn as a group title and is not selectable.
type menuItem struct {
	open     func()
	value    func() string
	adjust   func(dir int)
	radio    *radioGroup
	title    string
	target   screen
	disabled bool
	heading  bool
}

func menuHeading(title string) menuItem {
	return menuItem{title: title, heading: true}
}

func (item menuItem) selectable() bool {
	return !item.heading
}

func (u *ui) moreMenu() []menuItem {
	return []menuItem{
		{
			title: "Integrations",
			open:  func() { u.openMenu(screenIntegrations) },
			value: func() string { return fmt.Sprintf("%d pinned", len(u.settings.PinnedActions)) },
		},
		{title: "Settings", open: func() { u.openMenu(screenSettings) }},
		{title: "Gamepad info", target: screenGamepads},
		{title: "Diagnostics", target: screenDiagnostics},
		{title: "System", open: func() { u.openMenu(screenSystem) }},
		{title: "Back", target: screenHome},
	}
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

// integrationsMenu lists every known integration. Unconfigured ones stay
// focusable but disabled, with a value that says they need YAML.
func (u *ui) integrationsMenu() []menuItem {
	var items []menuItem
	for _, e := range integrations.Catalog() {
		items = append(items, u.integrationItem(e))
	}
	return append(items, menuItem{title: "Back", open: func() { u.openMenu(screenMore) }})
}

func (u *ui) integrationItem(e integrations.CatalogEntry) menuItem {
	if u.sourceByID(e.ID) != nil {
		return menuItem{
			title: e.Title,
			open:  func() { u.openActionList(e.ID) },
			value: func() string { return fmt.Sprintf("%d pinned", u.pinnedCount(e.ID)) },
		}
	}
	return menuItem{
		title:    e.Title,
		disabled: true,
		value:    func() string { return "Needs YAML config" },
	}
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
	items := u.settingsBackgroundItems(st, changed)
	items = append(items, u.settingsRSSItems(st, changed)...)
	items = append(items, u.settingsDisplayItems(st, changed)...)
	items = append(items, u.settingsWebItems(st, changed)...)
	return append(items, menuItem{title: "Back", open: func() { u.openMenu(screenMore) }})
}

func (u *ui) settingsBackgroundItems(st *settings.Settings, changed func()) []menuItem {
	return []menuItem{
		menuHeading("Backgrounds"),
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
			title: "Image attribution",
			radio: boolRadio("Shown", "Hidden", func() bool { return !st.HideAttribution }, func(v bool) { st.HideAttribution = !v; changed() }),
		},
	}
}

func (u *ui) settingsRSSItems(st *settings.Settings, changed func()) []menuItem {
	return []menuItem{
		menuHeading("RSS"),
		u.rssHeadlinesItem(st, changed),
		{
			title:  "Next headline every",
			value:  func() string { return settings.IntervalLabel(st.RSSInterval) },
			adjust: intervalAdjust(&st.RSSInterval, changed),
		},
	}
}

func (u *ui) settingsDisplayItems(st *settings.Settings, changed func()) []menuItem {
	return []menuItem{
		menuHeading("Display"),
		{
			title: "Control help",
			radio: boolRadio("Shown", "Hidden", func() bool { return !st.HideHelp }, func(v bool) { st.HideHelp = !v; changed() }),
		},
	}
}

func (u *ui) settingsWebItems(st *settings.Settings, changed func()) []menuItem {
	return []menuItem{
		menuHeading("Web control"),
		{
			title: "Web control",
			radio: boolRadio("On", "Off", func() bool { return st.WebControl }, func(v bool) { st.WebControl = v; changed() }),
		},
		u.qrPlaceItem(st, changed),
		u.webAddressItem(),
	}
}

func (u *ui) qrPlaceItem(st *settings.Settings, changed func()) menuItem {
	if !st.WebControl {
		return menuItem{
			title:    "QR code on home screen",
			disabled: true,
			value:    func() string { return "Needs web control" },
		}
	}
	return menuItem{
		title: "QR code on home screen",
		radio: choiceRadio(settings.QRPlaceLabels, func() int { return int(st.QRPlace) }, func(i int) { st.QRPlace = settings.QRPlace(i); changed() }),
	}
}

func (u *ui) rssHeadlinesItem(st *settings.Settings, changed func()) menuItem {
	if !u.rssConfigured() {
		return menuItem{
			title:    "RSS headlines",
			disabled: true,
			value:    func() string { return "Needs YAML config" },
		}
	}
	return menuItem{
		title: "RSS headlines",
		radio: boolRadio("On", "Off", func() bool { return st.RSSEnabled }, func(v bool) { st.RSSEnabled = v; changed() }),
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
	if !listMenus[sc] {
		return
	}
	_, items, _ := u.menuFor(sc)
	u.menuSel = firstSelectable(items)
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
	u.menuSel = clampSelectable(items, u.menuSel)
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
		u.menuSel = nextSelectable(items, u.menuSel, menuNavDir(a))
	case actionLeft, actionRight:
		adjustMenuItem(item, a)
	case actionActivate:
		u.activateMenuItem(item)
	}
}

func menuNavDir(a action) int {
	if a == actionUp {
		return -1
	}
	return 1
}

func firstSelectable(items []menuItem) int {
	for i, it := range items {
		if it.selectable() {
			return i
		}
	}
	return 0
}

func nextSelectable(items []menuItem, from, dir int) int {
	for i := from + dir; i >= 0 && i < len(items); i += dir {
		if items[i].selectable() {
			return i
		}
	}
	return from
}

func clampSelectable(items []menuItem, sel int) int {
	if len(items) == 0 {
		return 0
	}
	sel = clampInt(sel, len(items)-1)
	if items[sel].selectable() {
		return sel
	}
	return firstSelectable(items)
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
	if item.disabled {
		return
	}
	u.activateEnabled(item)
}

func (u *ui) activateEnabled(item menuItem) {
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
		u.drawListMenu(title, items, w, h, s, margin)
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

func (u *ui) drawListMenu(title string, items []menuItem, w, h, s, margin float32) {
	u.text(u.fitText(title, 52*s, w-2*margin), margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))
	itemW := w - 2*margin
	top, bottom := 270*s, h-120*s
	y := menuListOrigin(items, u.menuSel, top, bottom, s)
	u.drawMenuRows(items, margin, y, itemW, top, bottom, s)
}

func menuRowHeight(item menuItem, s float32) float32 {
	if item.heading {
		return 40 * s
	}
	return 74 * s
}

func menuRowGap(item menuItem, s float32) float32 {
	if item.heading {
		return 6 * s
	}
	return 14 * s
}

func menuListOrigin(items []menuItem, sel int, top, bottom, s float32) float32 {
	if sel < 0 || sel >= len(items) {
		return top
	}
	y := top
	for i, item := range items {
		if i == sel {
			return menuAlignRow(y, y+menuRowHeight(item, s), top, bottom)
		}
		y += menuRowHeight(item, s) + menuRowGap(item, s)
	}
	return top
}

func menuAlignRow(rowTop, rowBot, top, bottom float32) float32 {
	origin := top
	if rowBot > bottom {
		origin -= rowBot - bottom
	}
	drawnTop := rowTop - (top - origin)
	if drawnTop < top {
		origin += top - drawnTop
	}
	return origin
}

func (u *ui) drawMenuRows(items []menuItem, x, y, itemW, top, bottom, s float32) {
	rl.BeginScissorMode(int32(x-8*s), int32(top), int32(itemW+16*s), int32(bottom-top))
	for i, item := range items {
		ih := menuRowHeight(item, s)
		u.drawMenuRow(item, i == u.menuSel, x, y, itemW, ih, s)
		y += ih + menuRowGap(item, s)
	}
	rl.EndScissorMode()
}

func (u *ui) drawMenuRow(item menuItem, selected bool, x, y, itemW, itemH, s float32) {
	if item.heading {
		u.drawMenuHeading(item.title, x, y, itemH, s)
		return
	}
	u.drawMenuItem(item, selected, x, y, itemW, itemH, s)
}

func (u *ui) drawMenuHeading(title string, x, y, itemH, s float32) {
	size := 28 * s
	u.text(title, x+4*s, y+(itemH-size)/2, size, rl.ColorAlpha(u.accent, 0.95))
}

func menuItemColors(accent rl.Color, selected, disabled bool) (fill, textColor rl.Color) {
	fill, textColor = selectedMenuColors(accent, selected)
	if disabled {
		textColor = rl.ColorAlpha(textColor, 0.5)
	}
	return fill, textColor
}

func selectedMenuColors(accent rl.Color, selected bool) (fill, textColor rl.Color) {
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
	fill, textColor := menuItemColors(u.accent, selected, item.disabled)
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
	u.text(v, r.X+r.Width-36*s-m.X, r.Y+(itemH-36*s)/2, 36*s, menuValueColor(u.accent, selected && !item.disabled))
}

func (u *ui) drawGamepads(w, s, margin float32) {
	u.text("Gamepad info", margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))
	y := 280 * s
	pads := u.gamepadReadings()
	for _, p := range pads {
		y = u.drawGamepadCard(p, margin, y, w, s)
	}
	if len(pads) == 0 {
		u.text("No gamepads detected", margin, y, 40*s, rl.ColorAlpha(rl.RayWhite, 0.65))
	}
}

// gamepadReading is what the Gamepad info screen shows for one pad.
type gamepadReading struct {
	index int32
	name  string
	axes  int32
	// left x/y, right x/y, left trigger, right trigger
	sticks [6]float32
	held   []string
}

// gamepadReadings returns the connected pads, or the scripted ones when a
// script has added any (Xvfb has no gamepads).
func (u *ui) gamepadReadings() []gamepadReading {
	if u.scriptPads != nil {
		return u.scriptPads
	}
	var pads []gamepadReading
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) {
			pads = append(pads, readGamepad(g))
		}
	}
	return pads
}

func readGamepad(g int32) gamepadReading {
	return gamepadReading{
		index: g,
		name:  rl.GetGamepadName(g),
		axes:  rl.GetGamepadAxisCount(g),
		sticks: [6]float32{
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftX),
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftY),
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightX),
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightY),
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftTrigger),
			rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightTrigger),
		},
		held: heldGamepadButtons(g),
	}
}

// scriptGamepad is a pad at rest, as raylib reports one: sticks centred and
// triggers released (-1).
func scriptGamepad(index int32, name string) gamepadReading {
	return gamepadReading{index: index, name: name, axes: 6, sticks: [6]float32{0, 0, 0, 0, -1, -1}}
}

func (p gamepadReading) label() string {
	if p.name == "" {
		return "(unnamed)"
	}
	return p.name
}

func (p gamepadReading) axesLine() string {
	a := p.sticks
	return fmt.Sprintf("Axes: %d     Left stick: %+.2f %+.2f     Right stick: %+.2f %+.2f     Triggers: %+.2f %+.2f",
		p.axes, a[0], a[1], a[2], a[3], a[4], a[5])
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

func (p gamepadReading) heldText() string {
	if len(p.held) == 0 {
		return "Buttons held: none"
	}
	return "Buttons held: " + strings.Join(p.held, "  ")
}

func (u *ui) drawGamepadCard(p gamepadReading, margin, y, w, s float32) float32 {
	cardH := 190 * s
	r := rl.Rectangle{X: margin, Y: y, Width: w - 2*margin, Height: cardH}
	rl.DrawRectangleRounded(r, 0.12, 10, u.fa(rl.ColorAlpha(rl.White, 0.06)))
	u.text(fmt.Sprintf("%d   %s", p.index, p.label()), r.X+32*s, r.Y+24*s, 40*s, rl.RayWhite)
	u.text(u.fitText(p.axesLine(), 28*s, r.Width-64*s), r.X+32*s, r.Y+84*s, 28*s, rl.ColorAlpha(rl.RayWhite, 0.65))
	u.text(p.heldText(), r.X+32*s, r.Y+130*s, 28*s, u.accent)
	return y + cardH + 24*s
}

func clampInt(v, hi int) int {
	return min(max(v, 0), hi)
}
