package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/padbattery"
	"go.uber.org/zap"
)

const (
	batteryEvery = time.Minute
	batteryLow   = 20 // percent; warn below this
	batteryReArm = 25 // percent; warn again after charging above this
)

// readBatteries is padbattery.Read on sysfs; tests replace it.
var readBatteries = func() []padbattery.Battery { return padbattery.Read(padbattery.Root) }

// batteryWatch polls controller batteries and warns once per controller
// when one runs low.
type batteryWatch struct {
	next    time.Time
	warned  map[string]bool // by supply
	pending bool            // a read is in flight
}

// pollBatteries starts a background read every batteryEvery. Scripted runs
// (screenshots) skip it.
func (s *session) pollBatteries(now time.Time) {
	w := &s.batteries
	if s.sc != nil || w.pending || now.Before(w.next) {
		return
	}
	w.next = now.Add(batteryEvery)
	w.pending = true
	go func() { s.batteryResults <- readBatteries() }()
}

func (s *session) drainBatteries() {
	bs, ok := tryRecv(s.batteryResults)
	if !ok {
		return
	}
	s.batteries.pending = false
	s.u.batteries = bs
	for _, msg := range s.batteries.warnings(bs) {
		lg().Info("controller battery low", zap.String("message", msg))
		s.toastOrKeep(msg)
	}
}

// warnings returns a message for each battery that has just gone low, and
// re-arms ones that have been charged.
func (w *batteryWatch) warnings(bs []padbattery.Battery) []string {
	if w.warned == nil {
		w.warned = map[string]bool{}
	}
	var out []string
	for _, b := range bs {
		switch {
		case b.Charging() || b.Capacity > batteryReArm:
			delete(w.warned, b.Supply)
		case batteryIsLow(b) && !w.warned[b.Supply]:
			w.warned[b.Supply] = true
			out = append(out, b.Name+" battery low"+percentSuffix(b))
		}
	}
	return out
}

func batteryIsLow(b padbattery.Battery) bool {
	if b.Charging() {
		return false
	}
	if b.Capacity >= 0 {
		return b.Capacity < batteryLow
	}
	return b.Level == "Low" || b.Level == "Critical"
}

func percentSuffix(b padbattery.Battery) string {
	if b.Capacity < 0 {
		return ""
	}
	return fmt.Sprintf(" (%d%%)", b.Capacity)
}

// batteryLabel is the Input devices diagnostics line for one battery.
func batteryLabel(b padbattery.Battery) string {
	level := b.Level
	if b.Capacity >= 0 {
		level = fmt.Sprintf("%d%%", b.Capacity)
	}
	if b.Status != "" {
		level += " (" + b.Status + ")"
	}
	return "Battery " + level
}

// batteryFor finds the battery of the pad with this name. raylib's names
// and the kernel's can differ, so either may contain the other.
func (u *ui) batteryFor(padName string) (padbattery.Battery, bool) {
	p := strings.ToLower(padName)
	for _, b := range u.batteries {
		n := strings.ToLower(b.Name)
		if p != "" && (strings.Contains(p, n) || strings.Contains(n, p)) {
			return b, true
		}
	}
	return padbattery.Battery{}, false
}

// unmatchedBatteries are batteries no connected pad's name matches.
func (u *ui) unmatchedBatteries(pads []gamepadReading) []padbattery.Battery {
	var out []padbattery.Battery
	for _, b := range u.batteries {
		matched := false
		for _, p := range pads {
			if mb, ok := u.batteryFor(p.name); ok && mb.Supply == b.Supply {
				matched = true
			}
		}
		if !matched {
			out = append(out, b)
		}
	}
	return out
}

// toastOrKeep shows a message now, or keeps it for when the launcher comes
// back if an app is in front.
func (s *session) toastOrKeep(msg string) {
	if s.hidden {
		s.keptToast = msg
		return
	}
	s.u.setToast(msg, colorBad, 8)
}
