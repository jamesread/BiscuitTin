package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/settings"
)

// The QR code advertises the web remote, so it is only drawn while the
// server is actually running, whatever the QR setting says.
func TestWebListenLabel(t *testing.T) {
	u := &ui{listenAddr: "192.0.2.10:8137"}
	if u.webListenLabel() != "192.0.2.10:8137" {
		t.Fatalf("override = %q", u.webListenLabel())
	}
	u.listenAddr = ""
	u.webURL = "http://192.0.2.8:9000/"
	if u.webListenLabel() != "192.0.2.8:9000" {
		t.Fatalf("running = %q", u.webListenLabel())
	}
}

func TestQRNeedsRunningWebControl(t *testing.T) {
	u := &ui{settings: &settings.Settings{QRPlace: settings.QRMenu}}
	if u.qrVisible() {
		t.Fatal("QR shown with web control off")
	}
	u.webURL = "http://192.0.2.10:8137/"
	if !u.qrVisible() {
		t.Fatal("QR hidden with web control on and menu QR")
	}
	u.settings.QRPlace = settings.QRNone
	if u.qrVisible() {
		t.Fatal("QR shown with place none")
	}
}

func TestQRIdleOnlyWhenIdle(t *testing.T) {
	u := &ui{
		settings:  &settings.Settings{QRPlace: settings.QRIdle},
		webURL:    "http://192.0.2.10:8137/",
		bodyAlpha: 1,
	}
	if u.qrVisible() {
		t.Fatal("idle QR shown on the main menu")
	}
	u.idle = true
	if !u.qrVisible() {
		t.Fatal("idle QR hidden while idle")
	}
	if u.qrReservesGrid() {
		t.Fatal("idle QR should not shrink the tile grid")
	}
}

func TestQRMenuReservesGrid(t *testing.T) {
	u := &ui{settings: &settings.Settings{QRPlace: settings.QRMenu}, webURL: "http://192.0.2.10:8137/"}
	if !u.qrReservesGrid() {
		t.Fatal("menu QR should reserve a column")
	}
}

func TestQROriginIdleBottomRight(t *testing.T) {
	x0, y0 := qrOrigin(settings.QRIdle, 1920, 1080, 90, 220)
	if x0 != 1920-90-220 || y0 != 1080-90-220 {
		t.Fatalf("idle origin %v,%v", x0, y0)
	}
	_, mid := qrOrigin(settings.QRMenu, 1920, 1080, 90, 220)
	if mid != (1080-220)/2 {
		t.Fatalf("menu y %v", mid)
	}
}

func TestWebCodeMadeWhenWebControlOn(t *testing.T) {
	u := &ui{settings: &settings.Settings{}, webURL: "http://192.0.2.10:8137/"}
	u.ensureWebCode()
	if u.settings.WebCode != "" {
		t.Fatal("code made with web control off")
	}
	u.settings.WebControl = true
	u.ensureWebCode()
	code := u.settings.WebCode
	if len(code) != 8 || !u.settingsChanged {
		t.Fatalf("code = %q changed = %v", code, u.settingsChanged)
	}
	if got := u.qrTarget(); got != "http://192.0.2.10:8137/#code="+code {
		t.Fatalf("QR target = %q", got)
	}
	if titles := menuTitles(u.webCodeItems()); len(titles) != 2 || titles[1] != "New pairing code" {
		t.Fatalf("code items = %v", titles)
	}
	u.renewWebCode()
	if u.settings.WebCode == code || u.screen != screenSettings {
		t.Fatalf("renew: code %q screen %d", u.settings.WebCode, u.screen)
	}
}
