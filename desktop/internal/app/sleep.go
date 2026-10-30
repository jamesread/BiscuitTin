package app

import (
	"fmt"
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/settings"
)

// sleepSteps are the Sleep timer choices in minutes; 0 is Off.
var sleepSteps = []int{0, 30, 60, 90, 120}

// sleepWarning is how long before the timer ends the countdown appears.
const sleepWarning = 60 * time.Second

// sleepTimer is More > System > Sleep timer: after the chosen time the
// machine suspends or powers off, with a cancellable countdown first. It
// lasts for this session only.
type sleepTimer struct {
	at       time.Time
	minutes  int  // the chosen step, for the menu
	prompted bool // the countdown screen has been opened
}

func (t *sleepTimer) set(minutes int, now time.Time) {
	*t = sleepTimer{minutes: minutes}
	if minutes > 0 {
		t.at = now.Add(time.Duration(minutes) * time.Minute)
	}
}

func (t *sleepTimer) active() bool { return !t.at.IsZero() }

func (t *sleepTimer) left(now time.Time) time.Duration {
	return max(t.at.Sub(now), 0)
}

// stepSleep moves to the next or previous step; dir 0 (A) cycles round.
func stepSleep(minutes, dir int) int {
	i := 0
	for j, m := range sleepSteps {
		if m == minutes {
			i = j
		}
	}
	if dir == 0 {
		return sleepSteps[(i+1)%len(sleepSteps)]
	}
	return sleepSteps[clampInt(i+dir, len(sleepSteps)-1)]
}

// minutesLeft rounds up, so the label never says 0 while time remains.
func minutesLeft(d time.Duration) int {
	return int(math.Ceil(d.Minutes()))
}

func (u *ui) sleepLabel(now time.Time) string {
	if !u.sleep.active() {
		return "Off"
	}
	return fmt.Sprintf("%d min (%d left)", u.sleep.minutes, minutesLeft(u.sleep.left(now)))
}

func (u *ui) sleepItems(st *settings.Settings, changed func()) []menuItem {
	return []menuItem{
		{
			title: "Sleep timer",
			value: func() string { return u.sleepLabel(time.Now()) },
			adjust: func(dir int) {
				u.sleep.set(stepSleep(u.sleep.minutes, dir), time.Now())
			},
		},
		{
			title: "When it ends",
			radio: boolRadio("Suspend", "Power off", func() bool { return !st.SleepPowerOff }, func(v bool) { st.SleepPowerOff = !v; changed() }),
		},
	}
}

func (u *ui) sleepVerb() string {
	if u.settings != nil && u.settings.SleepPowerOff {
		return "Power off"
	}
	return "Suspend"
}

// sleepTitle is the countdown screen's title, worked out each frame.
func (u *ui) sleepTitle(now time.Time) string {
	verb := "Suspending"
	if u.sleepVerb() == "Power off" {
		verb = "Powering off"
	}
	return fmt.Sprintf("%s in %d s", verb, int(math.Ceil(u.sleep.left(now).Seconds())))
}

func (u *ui) sleepMenu() []menuItem {
	return []menuItem{
		{title: "Cancel timer", open: u.cancelSleep},
		{title: u.sleepVerb() + " now", open: u.sleepNow},
	}
}

func (u *ui) cancelSleep() {
	u.sleep.set(0, time.Time{})
	u.screen = screenHome
	u.setToast("Sleep timer cancelled", colorOK, 3)
}

func (u *ui) sleepNow() {
	u.sleep.set(0, time.Time{})
	if u.sleepVerb() == "Power off" {
		u.poweroffHost()
		return
	}
	u.suspendHost()
}

// checkSleep opens the countdown a minute before the timer ends, showing
// the launcher if an app is in front, and acts when it runs out.
func (s *session) checkSleep(now time.Time) {
	t := &s.u.sleep
	if !t.active() {
		return
	}
	if !now.Before(t.at) {
		s.u.sleepNow()
		return
	}
	if t.prompted || t.left(now) > sleepWarning {
		return
	}
	t.prompted = true
	if s.screenOff {
		s.wakeScreen()
	}
	s.hideAt = 0
	if s.hidden {
		s.show()
	}
	s.u.openMenu(screenSleep)
}

// drawSleepIndicator draws "Sleep 42m" in the header, right-aligned to
// right, and returns the width used.
func (u *ui) drawSleepIndicator(right, y, size, s float32) float32 {
	if !u.sleep.active() {
		return 0
	}
	label := fmt.Sprintf("Sleep %dm", minutesLeft(u.sleep.left(time.Now())))
	m := u.measure(label, size)
	u.text(label, right-m.X, y, size, rl.ColorAlpha(rl.RayWhite, 0.7))
	return m.X + 32*s
}
