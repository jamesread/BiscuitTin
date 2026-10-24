package app

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/internal/installationinfo"
)

type diagRow struct {
	key, value string
}

func (u *ui) diagnostics() []diagRow {
	b := installationinfo.Build
	host, _ := os.Hostname()

	gamepads := int(countGamepads())

	m := rl.GetCurrentMonitor()
	display := fmt.Sprintf("%d x %d @ %d Hz", rl.GetScreenWidth(), rl.GetScreenHeight(), rl.GetMonitorRefreshRate(m))

	return []diagRow{
		{"Version", b.Version},
		{"Commit", b.Commit},
		{"Build date", b.Date},
		{"Go", runtime.Version() + "  " + runtime.GOOS + "/" + runtime.GOARCH},
		{"Host", host},
		{"Display", display},
		{"Gamepads", fmt.Sprintf("%d detected", gamepads)},
		{"Primary input", u.primaryInputLabel()},
		{"Uptime", time.Since(u.startedAt).Round(time.Second).String()},
		{"Config", u.configPath},
		{"Settings", u.settings.File},
		{"Cache", u.cfg.CacheDir},
		{"OliveTin", orNone(u.cfg.OliveTin.URL)},
		{"Home Assistant", orNone(u.cfg.HomeAssistant.URL)},
		{"Web control", webControlLabel(u.webURL)},
		{"Debug", debugSocketLabel(u.debugSocket)},
		{"RSS", u.rss.Status()},
	}
}

// debugSnapshot is published to the local debug socket each frame.
func countGamepads() int32 {
	n := int32(0)
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) {
			n++
		}
	}
	return n
}

func snapshotMonitors() []*pb.Monitor {
	n := rl.GetMonitorCount()
	monitors := make([]*pb.Monitor, 0, n)
	for i := range n {
		monitors = append(monitors, &pb.Monitor{
			Index:  int32(i),
			Width:  int32(rl.GetMonitorWidth(i)),
			Height: int32(rl.GetMonitorHeight(i)),
			Name:   rl.GetMonitorName(i),
		})
	}
	return monitors
}

func (u *ui) selectedTileKey() string {
	if u.selected < 0 {
		return ""
	}
	if u.selected >= len(u.tiles) {
		return ""
	}
	return u.tiles[u.selected].key
}

func (u *ui) toastMessage() string {
	if u.toast == "" {
		return ""
	}
	if rl.GetTime() >= u.toastUntil {
		return ""
	}
	return u.toast
}

func (u *ui) debugSnapshot(hidden bool) *pb.GetDiagnosticsResponse {
	b := installationinfo.Build
	host, _ := os.Hostname()
	m := rl.GetCurrentMonitor()
	return &pb.GetDiagnosticsResponse{
		Version:      b.Version,
		Commit:       b.Commit,
		BuildDate:    b.Date,
		GoRuntime:    runtime.Version() + "  " + runtime.GOOS + "/" + runtime.GOARCH,
		Host:         host,
		Pid:          int32(os.Getpid()),
		Uptime:       time.Since(u.startedAt).Round(time.Second).String(),
		ConfigPath:   u.configPath,
		SettingsPath: u.settings.File,
		CacheDir:     u.cfg.CacheDir,
		SocketPath:   u.debugSocket,
		Window: &pb.Window{
			Width:      int32(rl.GetScreenWidth()),
			Height:     int32(rl.GetScreenHeight()),
			Fullscreen: rl.IsWindowState(rl.FlagBorderlessWindowedMode),
			Hidden:     hidden,
		},
		Display: &pb.Display{
			CurrentIndex: int32(m),
			RefreshHz:    int32(rl.GetMonitorRefreshRate(m)),
			Monitors:     snapshotMonitors(),
		},
		Screen:        screenNames[u.screen],
		SelectedTile:  u.selectedTileKey(),
		Message:       u.toastMessage(),
		GamepadCount:  countGamepads(),
		PrimaryInput:  u.primaryInputLabel(),
		Olivetin:      u.cfg.OliveTin.URL,
		HomeAssistant: u.cfg.HomeAssistant.URL,
		WebControl:    webControlLabel(u.webURL),
	}
}

func (u *ui) primaryInputLabel() string {
	if u.input == nil || u.input.primary == deviceKeyboard {
		return "Keyboard / remote"
	}
	layout := "Xbox-style buttons"
	if detectLayout(u.input.padName()) == layoutPlayStation {
		layout = "PlayStation buttons"
	}
	return "Gamepad: " + u.input.padName() + "  (" + layout + ")"
}

func webControlLabel(url string) string {
	if url == "" {
		return "off"
	}
	return url
}

func debugSocketLabel(path string) string {
	if path == "" {
		return "off"
	}
	return path
}

func orNone(s string) string {
	if s == "" {
		return "not configured"
	}
	return s
}

func (u *ui) drawDiagnostics(w, h, s, margin float32) {
	u.text("Diagnostics", margin, 190*s, 52*s, rl.ColorAlpha(rl.RayWhite, 0.85))

	rows := u.diagnostics()
	rowH := 46 * s
	size := 30 * s
	top := 270 * s
	avail := h - top - 80*s
	need := float32(len(rows))*rowH + 40*s
	if need > avail && len(rows) > 0 {
		rowH = (avail - 40*s) / float32(len(rows))
		size = rowH * 30 / 46
	}
	panel := rl.Rectangle{X: margin, Y: top, Width: w - 2*margin, Height: float32(len(rows))*rowH + 40*s}
	rl.DrawRectangleRounded(panel, 0.06, 10, u.fa(rl.ColorAlpha(rl.White, 0.06)))

	keyX := panel.X + 32*s
	keyCol := float32(math.Min(float64(280*s), float64(panel.Width*0.38)))
	valX := keyX + keyCol
	valW := panel.X + panel.Width - 32*s - valX
	y := panel.Y + 20*s
	for _, r := range rows {
		u.text(u.fitText(r.key, size, keyCol-16*s), keyX, y+(rowH-size)/2, size, rl.ColorAlpha(rl.RayWhite, 0.55))
		u.text(u.fitText(r.value, size, valW), valX, y+(rowH-size)/2, size, rl.RayWhite)
		y += rowH
	}
}
