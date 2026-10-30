package app

import (
	"time"

	"go.uber.org/zap"
)

// keepAwakeEvery is the most often a gamepad press resets the X idle timer;
// screensaver and DPMS timeouts are minutes, so this is plenty.
const keepAwakeEvery = 30 * time.Second

// resetter is x11win.ScreenSaver; tests use a fake.
type resetter interface {
	Reset() error
}

// keepAwake stops the screensaver and DPMS blanking the screen while a
// gamepad is in use, in the launcher or in a launched app.
type keepAwake struct {
	r       resetter
	last    time.Time
	enabled bool
}

// padUsed reports gamepad activity. It resets the X idle timer at most once
// per keepAwakeEvery.
func (k *keepAwake) padUsed(now time.Time) {
	if !k.enabled || k.r == nil || now.Sub(k.last) < keepAwakeEvery {
		return
	}
	k.last = now
	if err := k.r.Reset(); err != nil {
		lg().Info("keep awake unavailable", zap.Error(err))
		return
	}
	lg().Debug("reset screensaver for gamepad activity")
}
