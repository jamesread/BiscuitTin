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
)

// menuItem is one row of a list menu. It is a link (target, or open for
// screens that need setup), a radio group, or a stepped value whose adjust
// gets -1/+1 from left/right (clamped) and 0 from A (cycles round).
type menuItem struct {
	title  string
	target screen
	open   func()
	value  func() string
	adjust func(dir int)
	radio  *radioGroup
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
	u.openConfirmFrom(screenHome, "Exit biscuittin?", "Exit", func() { u.quit = true })
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
		src := src
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
	switch sc {
	case screenSettings:
		return "Settings", u.settingsMenu(), screenMore
	case screenIntegrations:
		return "Integrations", u.integrationsMenu(), screenMore
	case screenSystem:
		return "System", u.systemMenu(), screenMore
	case screenConfirm:
		return u.confirmTitle, u.confirmMenu(), u.confirmBack
	default:
		return "More", u.moreMenu(), screenHome
	}
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
func (u *ui) handleMenu(a action) {
	switch u.screen {
	case screenMore, screenSettings, screenIntegrations, screenSystem, screenConfirm:
		_, items, parent := u.menuFor(u.screen)
		u.menuSel = clampInt(u.menuSel, 0, len(items)-1)
		item := items[u.menuSel]
		switch a {
		case actionUp:
			if u.menuSel > 0 {
				u.menuSel--
			}
		case actionDown:
			if u.menuSel < len(items)-1 {
				u.menuSel++
			}
		case actionLeft, actionRight:
			dir := 1
			if a == actionLeft {
				dir = -1
			}
			switch {
			case item.radio != nil:
				item.radio.step(dir)
			case item.adjust != nil:
				item.adjust(dir)
			}
		case actionActivate:
			switch {
			case item.radio != nil:
				item.radio.cycle()
			case item.adjust != nil:
				item.adjust(0)
			case item.open != nil:
				item.open()
			default:
				u.screen = item.target
			}
		case actionBack, actionExit:
			if parent == screenHome {
				u.screen = screenHome
			} else {
				u.openMenu(parent)
			}
		}
	case screenDiagnostics:
		if a == actionBack || a == actionExit || a == actionActivate {
			u.screen = screenMore
		}
	case screenActionList:
		u.handleActionList(a)
	case screenGamepads:
		// B is also a button worth inspecting here, so only keyboard/remote
		// Back and the A button leave this screen.
		if a == actionBack || a == actionExit || a == actionActivate {
			u.screen = screenMore
		}
	}
}

func (u *ui) drawMenu(w, h, s, margin float32) {
	switch u.screen {
	case screenMore, screenSettings, screenIntegrations, screenSystem, screenConfirm:
		title, items, _ := u.menuFor(u.screen)
		u.drawListMenu(title, items, w, s, margin)
	case screenGamepads:
		u.drawGamepads(w, h, s, margin)
	case screenDiagnostics:
		u.drawDiagnostics(w, h, s, margin)
	case screenActionList:
		u.drawActionList(w, h, s, margin)
	}
}

func (u *ui) drawListMenu(title string, items []menuItem, w, s, margin float32) {
	u.text(u.fitText(title, 52*s, w-2*margin), margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))

	itemW := w - 2*margin
	itemH := float32(74) * s
	y := 290 * s
	for i, item := range items {
		r := rl.Rectangle{X: margin, Y: y, Width: itemW, Height: itemH}
		selected := i == u.menuSel
		fill := rl.ColorAlpha(rl.Black, 0.3)
		textColor := rl.ColorAlpha(rl.RayWhite, 0.7)
		if selected {
			fill = rl.ColorAlpha(u.accent, 0.22)
			textColor = rl.RayWhite
		}
		rl.DrawRectangleRounded(r, 0.25, 10, u.fa(fill))
		if selected {
			rl.DrawRectangleRoundedLinesEx(r, 0.25, 10, 4*s, u.fa(u.accent))
		}
		u.surface = &surfaceTint{contrast.RGBOf(fill), float64(fill.A) / 255}
		u.text(item.title, r.X+36*s, r.Y+(itemH-40*s)/2, 40*s, textColor)

		switch {
		case item.radio != nil:
			rh := itemH - 22*s
			u.drawRadioGroup(item.radio, r.X+r.Width-18*s, r.Y+(itemH-rh)/2, rh, s, selected)
		case item.value != nil:
			v := item.value()
			if selected && item.adjust != nil {
				v = "<   " + v + "   >"
			}
			m := u.measure(v, 36*s)
			valColor := rl.ColorAlpha(rl.RayWhite, 0.6)
			if selected {
				valColor = u.accent
			}
			u.text(v, r.X+r.Width-36*s-m.X, r.Y+(itemH-36*s)/2, 36*s, valColor)
		}
		u.surface = nil
		y += itemH + 14*s
	}
}

func (u *ui) drawGamepads(w, h, s, margin float32) {
	u.text("Gamepad info", margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))

	y := 280 * s
	found := 0
	for g := int32(0); g < maxGamepads; g++ {
		if !rl.IsGamepadAvailable(g) {
			continue
		}
		found++

		cardH := 190 * s
		r := rl.Rectangle{X: margin, Y: y, Width: w - 2*margin, Height: cardH}
		rl.DrawRectangleRounded(r, 0.12, 10, u.fa(rl.ColorAlpha(rl.White, 0.06)))

		name := rl.GetGamepadName(g)
		if name == "" {
			name = "(unnamed)"
		}
		u.text(fmt.Sprintf("%d   %s", g, name), r.X+32*s, r.Y+24*s, 40*s, rl.RayWhite)

		axes := rl.GetGamepadAxisCount(g)
		lx := rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftX)
		ly := rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftY)
		rx := rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightX)
		ry := rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightY)
		lt := rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftTrigger)
		rt := rl.GetGamepadAxisMovement(g, rl.GamepadAxisRightTrigger)
		axesLine := fmt.Sprintf("Axes: %d     Left stick: %+.2f %+.2f     Right stick: %+.2f %+.2f     Triggers: %+.2f %+.2f",
			axes, lx, ly, rx, ry, lt, rt)
		u.text(u.fitText(axesLine, 28*s, r.Width-64*s), r.X+32*s, r.Y+84*s, 28*s, rl.ColorAlpha(rl.RayWhite, 0.65))

		var held []string
		for b := int32(rl.GamepadButtonLeftFaceUp); b <= rl.GamepadButtonRightThumb; b++ {
			if rl.IsGamepadButtonDown(g, b) {
				held = append(held, gamepadButtonNames[b])
			}
		}
		heldText := "Buttons held: none"
		if len(held) > 0 {
			heldText = "Buttons held: " + strings.Join(held, "  ")
		}
		u.text(heldText, r.X+32*s, r.Y+130*s, 28*s, u.accent)

		y += cardH + 24*s
	}

	if found == 0 {
		u.text("No gamepads detected", margin, y, 40*s, rl.ColorAlpha(rl.RayWhite, 0.65))
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
