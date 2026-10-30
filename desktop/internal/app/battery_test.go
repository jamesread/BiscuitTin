package app

import (
	"slices"
	"testing"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/padbattery"
)

func pad(capacity int, status string) padbattery.Battery {
	return padbattery.Battery{Name: "DualSense Wireless Controller", Supply: "ps-1", Capacity: capacity, Status: status}
}

func TestBatteryWarnsOnceAndReArms(t *testing.T) {
	w := &batteryWatch{}
	steps := []struct {
		b    padbattery.Battery
		want int
	}{
		{pad(50, "Discharging"), 0},
		{pad(15, "Discharging"), 1}, // went low
		{pad(12, "Discharging"), 0}, // already warned
		{pad(12, "Charging"), 0},    // plugged in: re-armed, no warning
		{pad(18, "Discharging"), 1}, // unplugged while still low
		{pad(30, "Discharging"), 0}, // charged above the re-arm level
		{pad(19, "Discharging"), 1},
	}
	for i, st := range steps {
		if got := w.warnings([]padbattery.Battery{st.b}); len(got) != st.want {
			t.Fatalf("step %d (%+v): warnings %v", i, st.b, got)
		}
	}
}

func TestBatteryLevelOnly(t *testing.T) {
	b := padbattery.Battery{Name: "Xbox", Supply: "x", Capacity: -1, Level: "Critical", Status: "Discharging"}
	got := (&batteryWatch{}).warnings([]padbattery.Battery{b})
	if !slices.Equal(got, []string{"Xbox battery low"}) {
		t.Fatal(got)
	}
	if batteryLabel(b) != "Battery Critical (Discharging)" || batteryLabel(pad(62, "Discharging")) != "Battery 62% (Discharging)" {
		t.Fatal(batteryLabel(b))
	}
}

func TestBatteryForMatchesNames(t *testing.T) {
	u := &ui{batteries: []padbattery.Battery{pad(50, ""), {Name: "Other", Supply: "o"}}}
	if b, ok := u.batteryFor("PS5 Controller DualSense Wireless Controller"); !ok || b.Supply != "ps-1" {
		t.Fatalf("match = %+v %v", b, ok)
	}
	if _, ok := u.batteryFor(""); ok {
		t.Fatal("empty name should not match")
	}
	un := u.unmatchedBatteries([]gamepadReading{{name: "DualSense Wireless Controller"}})
	if len(un) != 1 || un[0].Supply != "o" {
		t.Fatalf("unmatched = %+v", un)
	}
}

func TestLowBatteryKeptWhileHidden(t *testing.T) {
	s := testSession([]config.Entry{{Title: "A", Command: []string{"a"}}})
	s.batteryResults = make(chan []padbattery.Battery, 1)
	s.hidden = true
	s.batteryResults <- []padbattery.Battery{pad(10, "Discharging")}
	s.drainBatteries()
	if s.u.toast != "" || s.keptToast != "DualSense Wireless Controller battery low (10%)" {
		t.Fatalf("toast %q kept %q", s.u.toast, s.keptToast)
	}
}

func TestPollBatteriesSkippedInScripts(t *testing.T) {
	orig := readBatteries
	t.Cleanup(func() { readBatteries = orig })
	readBatteries = func() []padbattery.Battery { t.Error("scripted runs should not read"); return nil }
	s := testSession([]config.Entry{{Title: "A", Command: []string{"a"}}})
	s.sc = &script{}
	s.pollBatteries(time.Now())
}
