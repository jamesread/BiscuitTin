package app

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"go.uber.org/zap"
)

type action int

const (
	actionNone action = iota
	actionUp
	actionDown
	actionLeft
	actionRight
	actionActivate
	actionBack
	actionExit
	actionQuit
)

const (
	maxGamepads   = 4
	stickDeadzone = 0.5
	repeatDelay   = 0.40
	repeatRate    = 0.12
)

// input turns keyboards, remote controls (which present as keyboards) and
// gamepads into one stream of directional actions with key-repeat on hold.
type input struct {
	held      action
	nextFire  float64
	guideHeld float64
	queued    []action

	// active is true for a frame with any new key or gamepad button press, or
	// a stick moved, including inputs that map to no action. It wakes the UI.
	// Buttons that are merely still held don't count, so the press that sent
	// the UI idle (Back) doesn't immediately wake it again.
	active bool

	// primary is the device used most recently, which decides whether control
	// help shows gamepad buttons or keys. pad is the gamepad last used.
	primary   inputDevice
	pad       int32
	usedInput bool

	// padOverride stands in for the gamepad's name in scripted runs, so the
	// hints don't depend on what is plugged in.
	padOverride string
}

type inputDevice int

const (
	deviceKeyboard inputDevice = iota
	deviceGamepad
)

// startupPadWindow is how long after start a connected gamepad makes the
// gamepad primary, before anything has been pressed. GLFW can take a few
// frames to report joysticks.
const startupPadWindow = 3.0

const activityDeadzone = 0.3

// gamepadActive reports a new button press or a moved stick, and on which
// gamepad.
func gamepadActive() (bool, int32) {
	for g := int32(0); g < maxGamepads; g++ {
		if !rl.IsGamepadAvailable(g) {
			continue
		}
		for b := int32(rl.GamepadButtonLeftFaceUp); b <= rl.GamepadButtonRightThumb; b++ {
			if rl.IsGamepadButtonPressed(g, b) {
				return true, g
			}
		}
		for _, axis := range []int32{rl.GamepadAxisLeftX, rl.GamepadAxisLeftY, rl.GamepadAxisRightX, rl.GamepadAxisRightY} {
			if v := rl.GetGamepadAxisMovement(g, axis); v > activityDeadzone || v < -activityDeadzone {
				return true, g
			}
		}
	}
	return false, 0
}

func firstGamepad() (int32, bool) {
	for g := int32(0); g < maxGamepads; g++ {
		if rl.IsGamepadAvailable(g) {
			return g, true
		}
	}
	return 0, false
}

// padName is the name of the gamepad hints should be drawn for.
func (in *input) padName() string {
	if in.padOverride != "" {
		return in.padOverride
	}
	if rl.IsGamepadAvailable(in.pad) {
		return rl.GetGamepadName(in.pad)
	}
	if g, ok := firstGamepad(); ok {
		return rl.GetGamepadName(g)
	}
	return ""
}

// keyAction maps a key press to an action. Remotes and xdotool often press
// and release within a single frame, which IsKeyPressed misses, so key
// presses are read from raylib's press queue instead.
func keyAction(k int32) action {
	switch k {
	case rl.KeyUp, rl.KeyW:
		return actionUp
	case rl.KeyDown, rl.KeyS:
		return actionDown
	case rl.KeyLeft, rl.KeyA:
		return actionLeft
	case rl.KeyRight, rl.KeyD:
		return actionRight
	case rl.KeyEnter, rl.KeyKpEnter, rl.KeySpace:
		return actionActivate
	case rl.KeyEscape:
		return actionExit
	case rl.KeyBackspace, rl.KeyBack:
		return actionBack
	case rl.KeyQ:
		if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
			return actionQuit
		}
	}
	return actionNone
}

func (in *input) directionHeld() action {
	switch {
	case rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW):
		return actionUp
	case rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS):
		return actionDown
	case rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA):
		return actionLeft
	case rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD):
		return actionRight
	}

	for g := int32(0); g < maxGamepads; g++ {
		if !rl.IsGamepadAvailable(g) {
			continue
		}
		x := rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftX)
		y := rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftY)
		switch {
		case rl.IsGamepadButtonDown(g, rl.GamepadButtonLeftFaceUp) || y < -stickDeadzone:
			return actionUp
		case rl.IsGamepadButtonDown(g, rl.GamepadButtonLeftFaceDown) || y > stickDeadzone:
			return actionDown
		case rl.IsGamepadButtonDown(g, rl.GamepadButtonLeftFaceLeft) || x < -stickDeadzone:
			return actionLeft
		case rl.IsGamepadButtonDown(g, rl.GamepadButtonLeftFaceRight) || x > stickDeadzone:
			return actionRight
		}
	}

	return actionNone
}

func gamepadActivatePressed() bool {
	for g := int32(0); g < maxGamepads; g++ {
		if rl.IsGamepadAvailable(g) &&
			(rl.IsGamepadButtonPressed(g, rl.GamepadButtonRightFaceDown) ||
				rl.IsGamepadButtonPressed(g, rl.GamepadButtonMiddleRight)) {
			return true
		}
	}
	return false
}

func gamepadBackPressed() bool {
	for g := int32(0); g < maxGamepads; g++ {
		if rl.IsGamepadAvailable(g) && rl.IsGamepadButtonPressed(g, rl.GamepadButtonRightFaceRight) {
			return true
		}
	}
	return false
}

// poll returns at most one action per frame.
func (in *input) poll() action {
	before := in.primary
	defer func() {
		if in.primary != before {
			lg().Debug("primary input changed", zap.Int("device", int(in.primary)), zap.Int32("pad", in.pad), zap.String("padName", in.padName()))
		}
	}()
	padUsed, pad := gamepadActive()
	in.active = padUsed
	if padUsed {
		in.primary, in.pad, in.usedInput = deviceGamepad, pad, true
	} else if !in.usedInput && rl.GetTime() < startupPadWindow {
		if g, ok := firstGamepad(); ok {
			in.primary, in.pad = deviceGamepad, g
		}
	}
	for k := rl.GetKeyPressed(); k != 0; k = rl.GetKeyPressed() {
		in.active = true
		in.primary, in.usedInput = deviceKeyboard, true
		if a := keyAction(k); a != actionNone {
			in.queued = append(in.queued, a)
		}
	}

	if gamepadActivatePressed() {
		in.queued = append(in.queued, actionActivate)
	}
	if gamepadBackPressed() {
		in.queued = append(in.queued, actionBack)
	}

	now := rl.GetTime()
	dir := in.directionHeld()

	if len(in.queued) > 0 {
		a := in.queued[0]
		in.queued = in.queued[1:]
		// A queued direction counts as the initial press of any hold.
		if a == dir {
			in.held = dir
			in.nextFire = now + repeatDelay
		}
		return a
	}

	if dir == actionNone {
		in.held = actionNone
		return actionNone
	}

	if dir != in.held {
		in.held = dir
		in.nextFire = now + repeatDelay
		return dir
	}

	if now >= in.nextFire {
		in.nextFire = now + repeatRate
		return dir
	}

	return actionNone
}

// guideHeldFor reports whether the Guide/Home button on any gamepad has been
// held continuously for the given number of seconds. Gamepads are read even
// when the window is hidden or unfocused, so this is how a hidden launcher is
// brought back.
func (in *input) guideHeldFor(seconds float64) bool {
	down := false
	for g := int32(0); g < maxGamepads; g++ {
		if rl.IsGamepadAvailable(g) && rl.IsGamepadButtonDown(g, rl.GamepadButtonMiddle) {
			down = true
			break
		}
	}

	if !down {
		in.guideHeld = 0
		return false
	}

	now := rl.GetTime()
	if in.guideHeld == 0 {
		in.guideHeld = now
	}
	if now-in.guideHeld >= seconds {
		in.guideHeld = 0
		return true
	}
	return false
}
