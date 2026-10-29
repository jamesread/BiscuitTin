// Package tray shows a tray icon when the desktop has a tray: StatusNotifier
// first, then the XEmbed system tray (polybar, tint2, and similar).
// Startup without a tray is normal and is only logged.
package tray

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

const (
	watcherName = "org.kde.StatusNotifierWatcher"
	watcherPath = dbus.ObjectPath("/StatusNotifierWatcher")
	itemIface   = "org.kde.StatusNotifierItem"
	itemPath    = dbus.ObjectPath("/StatusNotifierItem")
	menuIface   = "com.canonical.dbusmenu"
	menuPath    = dbus.ObjectPath("/StatusNotifierMenu")
)

// Event is a user action on the tray icon.
type Event int

const (
	EventShow Event = iota
	EventQuit
)

// Host is the tray icon for this process. Start always returns a Host;
// Events stays silent when the tray is unused.
type Host struct {
	Events <-chan Event
	send   chan Event
	conn   *dbus.Conn
	xe     *xembed
}

// Start tries to show a tray icon. A missing session bus or tray host is
// logged and ignored.
func Start() *Host {
	h := newHost()
	proto, err := h.serve()
	if err != nil {
		h.Stop()
		lg().Info("tray unused", zap.Error(err))
		return h
	}
	lg().Info("tray icon ready", zap.String("protocol", proto))
	return h
}

// Stop removes the tray icon. It is safe on an unused Host and on nil.
func (h *Host) Stop() {
	if h == nil {
		return
	}
	h.stopSNI()
	h.stopXEmbed()
}

func (h *Host) stopSNI() {
	if h.conn == nil {
		return
	}
	_ = h.conn.Close()
	h.conn = nil
}

func (h *Host) stopXEmbed() {
	if h.xe == nil {
		return
	}
	h.xe.close()
	h.xe = nil
}

func newHost() *Host {
	ch := make(chan Event, 4)
	return &Host{Events: ch, send: ch}
}

func (h *Host) emit(ev Event) {
	select {
	case h.send <- ev:
	default:
	}
}

func (h *Host) serve() (string, error) {
	sniErr := h.serveSNI()
	if sniErr == nil {
		return "statusnotifier", nil
	}
	lg().Debug("status notifier unused", zap.Error(sniErr))
	if err := h.serveXEmbed(); err != nil {
		return "", errors.Join(sniErr, err)
	}
	return "xembed", nil
}

func (h *Host) serveSNI() error {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(context.Background()))
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}
	h.conn = conn
	if err := h.attach(conn); err != nil {
		h.stopSNI()
		return err
	}
	return nil
}

func (h *Host) attach(conn *dbus.Conn) error {
	if err := ensureHost(conn); err != nil {
		return err
	}
	if err := h.export(conn); err != nil {
		return err
	}
	return h.register(conn)
}

func (h *Host) export(conn *dbus.Conn) error {
	pix, err := itemPixmaps()
	if err != nil {
		return err
	}
	if err := exportItem(conn, &sni{emit: h.emit}, pix); err != nil {
		return err
	}
	return exportMenu(conn, &menu{emit: h.emit})
}

func (h *Host) register(conn *dbus.Conn) error {
	name := itemServiceName()
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("claim tray name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("tray name %s already taken", name)
	}
	return registerItem(conn, name)
}

func registerItem(conn *dbus.Conn, name string) error {
	obj := conn.Object(watcherName, watcherPath)
	call := obj.CallWithContext(context.Background(), watcherName+".RegisterStatusNotifierItem", 0, name)
	if call.Err != nil {
		return fmt.Errorf("register tray item: %w", call.Err)
	}
	return nil
}

func ensureHost(conn *dbus.Conn) error {
	if !nameOwned(conn, watcherName) {
		return errors.New("no status notifier watcher")
	}
	if !hostRegistered(conn) {
		return errors.New("no status notifier host")
	}
	return nil
}

func nameOwned(conn *dbus.Conn, name string) bool {
	var owned bool
	err := conn.BusObject().CallWithContext(context.Background(), "org.freedesktop.DBus.NameHasOwner", 0, name).Store(&owned)
	return err == nil && owned
}

func hostRegistered(conn *dbus.Conn) bool {
	obj := conn.Object(watcherName, watcherPath)
	var v dbus.Variant
	err := obj.CallWithContext(context.Background(), "org.freedesktop.DBus.Properties.Get", 0, watcherName, "IsStatusNotifierHostRegistered").Store(&v)
	if err != nil {
		return false
	}
	registered, _ := v.Value().(bool)
	return registered
}

func itemServiceName() string {
	return fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
}

func lg() *zap.Logger { return zap.L().Named("tray") }
