package tray

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestStartWhenTrayMissing(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/no/such/biscuittin-tray-bus")
	t.Setenv("DISPLAY", "")
	h := Start()
	defer h.Stop()
	select {
	case ev := <-h.Events:
		t.Fatalf("unused tray sent %v", ev)
	default:
	}
}

func TestPixmapARGB(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 40})
	p := rgbaPixmap(img)
	if p.Width != 1 || p.Height != 1 {
		t.Fatalf("size %dx%d", p.Width, p.Height)
	}
	want := []byte{40, 10, 20, 30}
	if !bytes.Equal(p.Pixels, want) {
		t.Fatalf("pixels %v; want %v", p.Pixels, want)
	}
}

func TestItemPixmaps(t *testing.T) {
	p, err := itemPixmaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 {
		t.Fatalf("len=%d", len(p))
	}
	if p[0].Width != iconSize || p[0].Height != iconSize {
		t.Fatalf("size %dx%d", p[0].Width, p[0].Height)
	}
	if len(p[0].Pixels) != iconSize*iconSize*4 {
		t.Fatalf("pixels %d", len(p[0].Pixels))
	}
}

func TestMenuClick(t *testing.T) {
	h := newHost()
	m := &menu{emit: h.emit}
	if err := m.Event(menuShow, "clicked", dbus.MakeVariant(""), 0); err != nil {
		t.Fatal(err)
	}
	if ev := recvEvent(t, h); ev != EventShow {
		t.Fatalf("show got %v", ev)
	}
	if err := m.Event(menuExit, "clicked", dbus.MakeVariant(""), 0); err != nil {
		t.Fatal(err)
	}
	if ev := recvEvent(t, h); ev != EventQuit {
		t.Fatalf("quit got %v", ev)
	}
}

func TestActivateShows(t *testing.T) {
	h := newHost()
	item := &sni{emit: h.emit}
	if err := item.Activate(0, 0); err != nil {
		t.Fatal(err)
	}
	if ev := recvEvent(t, h); ev != EventShow {
		t.Fatalf("got %v", ev)
	}
}

func TestItemServiceName(t *testing.T) {
	name := itemServiceName()
	if !strings.HasPrefix(name, "org.kde.StatusNotifierItem-") {
		t.Fatalf("name %q", name)
	}
}

func recvEvent(t *testing.T, h *Host) Event {
	t.Helper()
	select {
	case ev := <-h.Events:
		return ev
	default:
		t.Fatal("no event")
		return 0
	}
}
