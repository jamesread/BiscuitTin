package app

import (
	"testing"

	"github.com/jamesread/biscuittin/internal/settings"
)

// The QR code advertises the web remote, so it is only drawn while the
// server is actually running, whatever the QR setting says.
func TestQRNeedsRunningWebControl(t *testing.T) {
	u := &ui{settings: &settings.Settings{ShowQR: true}}
	if u.qrVisible() {
		t.Fatal("QR shown with web control off")
	}
	u.webURL = "http://192.0.2.10:8137/"
	if !u.qrVisible() {
		t.Fatal("QR hidden with web control on and QR shown")
	}
	u.settings.ShowQR = false
	if u.qrVisible() {
		t.Fatal("QR shown with the QR setting off")
	}
}
