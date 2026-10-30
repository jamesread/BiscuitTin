// Package x11win finds a launched app's top-level window and asks the window
// manager to bring it to the front, via EWMH (_NET_CLIENT_LIST,
// _NET_ACTIVE_WINDOW).
package x11win

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// ErrNotFound is returned when no window belongs to the app.
var ErrNotFound = errors.New("no window found")

// Window is a top-level client window as the window manager lists it.
type Window struct {
	ID    xproto.Window
	PID   int    // _NET_WM_PID, 0 if unset
	Class string // WM_CLASS, instance and class joined by a space
}

// Switch activates the newest window whose process is in one of sessions
// (launch.Start gives each app its own session, led by the launched pid),
// or failing that whose WM_CLASS matches class.
func Switch(sessions []int, class string) error {
	x, err := xgb.NewConn()
	if err != nil {
		return fmt.Errorf("x11: %w", err)
	}
	defer x.Close()
	c := &client{x: x, root: xproto.Setup(x).DefaultScreen(x).Root}
	wins, err := c.clients()
	if err != nil {
		return err
	}
	w, ok := Match(wins, sessions, class, sessionOf)
	if !ok {
		return ErrNotFound
	}
	return c.activate(w)
}

// Match picks the window to switch to. Windows are in _NET_CLIENT_LIST
// order, oldest first, so the newest match wins.
func Match(wins []Window, sessions []int, class string, sessionOf func(pid int) (int, bool)) (xproto.Window, bool) {
	want := map[int]bool{}
	for _, s := range sessions {
		want[s] = true
	}
	for i := len(wins) - 1; i >= 0; i-- {
		if sid, ok := sessionOf(wins[i].PID); ok && want[sid] {
			return wins[i].ID, true
		}
	}
	if class == "" {
		return 0, false
	}
	for i := len(wins) - 1; i >= 0; i-- {
		if classMatches(wins[i].Class, class) {
			return wins[i].ID, true
		}
	}
	return 0, false
}

func classMatches(wmClass, want string) bool {
	for part := range strings.FieldsSeq(wmClass) {
		if strings.EqualFold(part, want) {
			return true
		}
	}
	return false
}

// sessionOf reads the session id from /proc/<pid>/stat. Processes in another
// pid namespace (Flatpak apps) are not visible, so these fall back to class.
func sessionOf(pid int) (int, bool) {
	if pid <= 0 {
		return 0, false
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	return ParseSession(string(raw))
}

// ParseSession returns the session field of a /proc/<pid>/stat line. The
// command name is in parentheses and may itself contain spaces or
// parentheses, so fields are counted from the last ")".
func ParseSession(stat string) (int, bool) {
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, false
	}
	// state ppid pgrp session ...
	f := strings.Fields(stat[end+1:])
	if len(f) < 4 {
		return 0, false
	}
	sid, err := strconv.Atoi(f[3])
	if err != nil {
		return 0, false
	}
	return sid, true
}

type client struct {
	x    *xgb.Conn
	root xproto.Window
}

func (c *client) atom(name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(c.x, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

func (c *client) property(w xproto.Window, name string) (*xproto.GetPropertyReply, error) {
	a, err := c.atom(name)
	if err != nil {
		return nil, err
	}
	return xproto.GetProperty(c.x, false, w, a, xproto.GetPropertyTypeAny, 0, 1<<16).Reply()
}

func (c *client) clients() ([]Window, error) {
	reply, err := c.property(c.root, "_NET_CLIENT_LIST")
	if err != nil {
		return nil, err
	}
	if reply.Format != 32 {
		return nil, errors.New("window manager has no _NET_CLIENT_LIST")
	}
	var wins []Window
	for _, v := range cardinals(reply.Value) {
		id := xproto.Window(v)
		wins = append(wins, Window{ID: id, PID: c.pid(id), Class: c.class(id)})
	}
	return wins, nil
}

func (c *client) pid(w xproto.Window) int {
	reply, err := c.property(w, "_NET_WM_PID")
	if err != nil || reply.Format != 32 {
		return 0
	}
	vals := cardinals(reply.Value)
	if len(vals) == 0 {
		return 0
	}
	return int(vals[0])
}

func (c *client) class(w xproto.Window) string {
	reply, err := c.property(w, "WM_CLASS")
	if err != nil || reply.Format != 8 {
		return ""
	}
	return strings.Join(strings.FieldsFunc(string(reply.Value), func(r rune) bool { return r == 0 }), " ")
}

func cardinals(b []byte) []uint32 {
	out := make([]uint32, 0, len(b)/4)
	for i := 0; i+4 <= len(b); i += 4 {
		out = append(out, xgb.Get32(b[i:]))
	}
	return out
}

// activate sends _NET_ACTIVE_WINDOW with source indication 2 (a pager or
// taskbar acting for the user), which window managers honour rather than
// treating as focus stealing.
func (c *client) activate(w xproto.Window) error {
	a, err := c.atom("_NET_ACTIVE_WINDOW")
	if err != nil {
		return err
	}
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: w,
		Type:   a,
		Data:   xproto.ClientMessageDataUnionData32New([]uint32{2, xproto.TimeCurrentTime, 0, 0, 0}),
	}
	mask := uint32(xproto.EventMaskSubstructureRedirect | xproto.EventMaskSubstructureNotify)
	return xproto.SendEventChecked(c.x, false, c.root, mask, string(ev.Bytes())).Check()
}
