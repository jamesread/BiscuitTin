package app

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/webcontrol"
	"go.uber.org/zap"
	"rsc.io/qr"
)

var screenNames = map[screen]string{
	screenHome:         "home",
	screenMore:         "more",
	screenSettings:     "settings",
	screenIntegrations: "integrations",
	screenActionList:   "actions",
	screenGamepads:     "gamepads",
	screenDiagnostics:  "diagnostics",
	screenSystem:       "system",
	screenConfirm:      "confirm",
	screenRunning:      "running",
}

// syncWebControl starts or stops the web server to match the setting, and
// returns it with the URL to show (empty when off).
func startWebControl(cfg *config.Config, bridge *webcontrol.Bridge, u *ui) (*webcontrol.Server, string) {
	web, err := webcontrol.Start(webcontrol.Port(cfg.WebControl), bridge)
	if err != nil {
		lg().Error("web control could not start", zap.Error(err))
		u.setToast("Web control could not start: "+err.Error(), colorBad, 8)
		u.settings.WebControl = false
		u.settingsChanged = true
		return nil, ""
	}
	return web, web.URL()
}

func stopWebControl(web *webcontrol.Server) (*webcontrol.Server, string) {
	if web != nil {
		web.Stop()
	}
	return nil, ""
}

func syncWebControl(web *webcontrol.Server, enabled bool, cfg *config.Config, bridge *webcontrol.Bridge, u *ui) (*webcontrol.Server, string) {
	if !enabled {
		return stopWebControl(web)
	}
	if web == nil {
		return startWebControl(cfg, bridge, u)
	}
	return web, web.URL()
}

func drainRemote(b *webcontrol.Bridge) {
	for {
		select {
		case <-b.Commands:
		default:
			return
		}
	}
}

var buttonActions = map[pb.Button]action{
	pb.Button_BUTTON_UP:     actionUp,
	pb.Button_BUTTON_DOWN:   actionDown,
	pb.Button_BUTTON_LEFT:   actionLeft,
	pb.Button_BUTTON_RIGHT:  actionRight,
	pb.Button_BUTTON_SELECT: actionActivate,
	pb.Button_BUTTON_BACK:   actionBack,
}

// nextRemoteAction turns one queued web remote command into an action.
// remoteTap is true for a tile tapped on the phone.
func (u *ui) applyRemoteTile(key string) bool {
	for i, t := range u.tiles {
		if t.key == key {
			u.screen = screenHome
			u.selected = i
			return true
		}
	}
	return false
}

func handleRemote(c webcontrol.Command, apply func(string) bool) (action, bool) {
	if c.TileKey == "" {
		return buttonActions[c.Button], false
	}
	if apply(c.TileKey) {
		return actionActivate, true
	}
	return actionNone, false
}

func (u *ui) nextRemoteAction(b *webcontrol.Bridge) (a action, remoteTap bool) {
	select {
	case c := <-b.Commands:
		return handleRemote(c, u.applyRemoteTile)
	default:
		return actionNone, false
	}
}

// remoteState is the snapshot served by RemoteService.GetState.
func (u *ui) remoteState(hidden bool) *pb.GetStateResponse {
	s := &pb.GetStateResponse{
		Title:  u.cfg.Title,
		Screen: screenNames[u.screen],
		Hidden: hidden,
	}
	if u.toast != "" && rl.GetTime() < u.toastUntil {
		s.Message = u.toast
	}
	for i, t := range u.tiles {
		s.Tiles = append(s.Tiles, &pb.Tile{
			Key:      t.key,
			Title:    t.title,
			Subtitle: t.subtitle,
			Selected: i == u.selected,
			Running:  u.tileRunning(t),
		})
	}
	return s
}

// drawQR draws a QR code for the web remote's URL in the bottom right of the
// home screen, dark modules on white with the standard 4-module quiet zone.
// qrSide is the QR code's width at 1080p.
const qrSide = 220

func (u *ui) qrVisible() bool {
	return u.webURL != "" && u.settings.ShowQR
}

func (u *ui) ensureQR() bool {
	if u.qrURL == u.webURL {
		return true
	}
	code, err := qr.Encode(u.webURL, qr.M)
	if err != nil {
		lg().Warn("QR code", zap.Error(err))
		return false
	}
	u.qr, u.qrURL = code, u.webURL
	return true
}

func (u *ui) paintQRRow(x0, y0, cell float32, quiet, y int, dark rl.Color) {
	for x := range u.qr.Size {
		if u.qr.Black(x, y) {
			rl.DrawRectangleRec(rl.Rectangle{
				X: x0 + float32(x+quiet)*cell, Y: y0 + float32(y+quiet)*cell,
				Width: cell + 0.5, Height: cell + 0.5, // overlap to avoid hairline gaps
			}, dark)
		}
	}
}

func (u *ui) paintQRModules(x0, y0, cell float32, quiet int) {
	dark := u.fa(rl.Black)
	for y := range u.qr.Size {
		u.paintQRRow(x0, y0, cell, quiet, y, dark)
	}
}

func (u *ui) paintQRLabels(x0, y0, side, s float32) {
	label := "Remote control"
	size := 26 * s
	m := u.measure(label, size)
	u.text(label, x0+(side-m.X)/2, y0-size-14*s, size, rl.RayWhite)
	addr := strings.TrimSuffix(strings.TrimPrefix(u.webURL, "http://"), "/")
	as := 22 * s
	am := u.measure(addr, as)
	u.text(addr, x0+(side-am.X)/2, y0+side+12*s, as, rl.ColorAlpha(rl.RayWhite, 0.85))
}

func (u *ui) paintQR(w, h, s, margin float32) {
	const quiet = 4
	side := qrSide * s
	cell := side / float32(u.qr.Size+2*quiet)
	x0 := w - margin - side
	y0 := (h - side) / 2
	rl.DrawRectangleRounded(rl.Rectangle{X: x0, Y: y0, Width: side, Height: side}, 0.06, 6, u.fa(rl.White))
	u.paintQRModules(x0, y0, cell, quiet)
	u.paintQRLabels(x0, y0, side, s)
}

func (u *ui) drawQR(w, h, s, margin float32) {
	if !u.qrVisible() {
		return
	}
	if !u.ensureQR() {
		return
	}
	u.paintQR(w, h, s, margin)
}
