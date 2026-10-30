package app

import (
	"slices"

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
	actionVolumeUp
	actionVolumeDown
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
	// padOverride stands in for the gamepad's name in scripted runs, so the
	// hints don't depend on what is plugged in.
	padOverride string
	queued      []action
	// recent is the last few key and CEC presses, by name, newest last.
	recent    []string
	held      action
	nextFire  float64
	guideHeld float64
	// primary is the device used most recently, which decides whether control
	// help shows gamepad buttons or keys. pad is the gamepad last used.
	primary   inputDevice
	pad       int32
	usedInput bool
	// active is true for a frame with any new key or gamepad button press, or
	// a stick moved, including inputs that map to no action. It wakes the UI.
	// Buttons that are merely still held don't count, so the press that sent
	// the UI idle (Back) doesn't immediately wake it again.
	active bool
	// padActive is true for a frame with a new gamepad press or a moved
	// stick, which X doesn't see as activity.
	padActive bool
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
	for g := range int32(maxGamepads) {
		if padNewlyUsed(g) {
			return true, g
		}
	}
	return false, 0
}

func padNewlyUsed(g int32) bool {
	if !rl.IsGamepadAvailable(g) {
		return false
	}
	if padButtonPressed(g) {
		return true
	}
	return padStickMoved(g)
}

func padButtonPressed(g int32) bool {
	for b := int32(rl.GamepadButtonLeftFaceUp); b <= rl.GamepadButtonRightThumb; b++ {
		if rl.IsGamepadButtonPressed(g, b) {
			return true
		}
	}
	return false
}

func axisMoved(v float32) bool {
	if v > activityDeadzone {
		return true
	}
	return v < -activityDeadzone
}

func padStickMoved(g int32) bool {
	for _, axis := range []int32{rl.GamepadAxisLeftX, rl.GamepadAxisLeftY, rl.GamepadAxisRightX, rl.GamepadAxisRightY} {
		if axisMoved(rl.GetGamepadAxisMovement(g, axis)) {
			return true
		}
	}
	return false
}

func firstGamepad() (int32, bool) {
	for g := range int32(maxGamepads) {
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
var keyActions = map[int32]action{
	rl.KeyUp:         actionUp,
	rl.KeyW:          actionUp,
	rl.KeyDown:       actionDown,
	rl.KeyS:          actionDown,
	rl.KeyLeft:       actionLeft,
	rl.KeyA:          actionLeft,
	rl.KeyRight:      actionRight,
	rl.KeyD:          actionRight,
	rl.KeyEnter:      actionActivate,
	rl.KeyKpEnter:    actionActivate,
	rl.KeySpace:      actionActivate,
	rl.KeyEscape:     actionExit,
	rl.KeyBackspace:  actionBack,
	rl.KeyBack:       actionBack,
	rl.KeyMinus:      actionVolumeDown,
	rl.KeyKpSubtract: actionVolumeDown,
	rl.KeyEqual:      actionVolumeUp,
	rl.KeyKpAdd:      actionVolumeUp,
}

func keyAction(k int32) action {
	if k == rl.KeyQ && (rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)) {
		return actionQuit
	}
	return keyActions[k]
}

func anyKeyDown(keys ...int32) bool {
	return slices.ContainsFunc(keys, rl.IsKeyDown)
}

func keyDir(keys []int32, a action) action {
	if anyKeyDown(keys...) {
		return a
	}
	return actionNone
}

func keyboardDirection() action {
	dirs := []struct {
		keys []int32
		a    action
	}{
		{[]int32{rl.KeyUp, rl.KeyW}, actionUp},
		{[]int32{rl.KeyDown, rl.KeyS}, actionDown},
		{[]int32{rl.KeyLeft, rl.KeyA}, actionLeft},
		{[]int32{rl.KeyRight, rl.KeyD}, actionRight},
	}
	for _, d := range dirs {
		if a := keyDir(d.keys, d.a); a != actionNone {
			return a
		}
	}
	return actionNone
}

func padHeld(g, button int32, axis float32, negative bool) bool {
	if rl.IsGamepadButtonDown(g, button) {
		return true
	}
	if negative {
		return axis < -stickDeadzone
	}
	return axis > stickDeadzone
}

func gamepadDirectionOn(g int32) action {
	if !rl.IsGamepadAvailable(g) {
		return actionNone
	}
	return padDirection(g, rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftX), rl.GetGamepadAxisMovement(g, rl.GamepadAxisLeftY))
}

func padDirection(g int32, x, y float32) action {
	checks := []struct {
		button   int32
		axis     float32
		a        action
		negative bool
	}{
		{rl.GamepadButtonLeftFaceUp, y, actionUp, true},
		{rl.GamepadButtonLeftFaceDown, y, actionDown, false},
		{rl.GamepadButtonLeftFaceLeft, x, actionLeft, true},
		{rl.GamepadButtonLeftFaceRight, x, actionRight, false},
	}
	for _, c := range checks {
		if padHeld(g, c.button, c.axis, c.negative) {
			return c.a
		}
	}
	return actionNone
}

func (in *input) directionHeld() action {
	if a := keyboardDirection(); a != actionNone {
		return a
	}
	for g := range int32(maxGamepads) {
		if a := gamepadDirectionOn(g); a != actionNone {
			return a
		}
	}
	return actionNone
}

func faceOrStartPressed(g int32) bool {
	if rl.IsGamepadButtonPressed(g, rl.GamepadButtonRightFaceDown) {
		return true
	}
	return rl.IsGamepadButtonPressed(g, rl.GamepadButtonMiddleRight)
}

func gamepadActivatePressed() bool {
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) && faceOrStartPressed(g) {
			return true
		}
	}
	return false
}

func gamepadBackPressed() bool {
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) && rl.IsGamepadButtonPressed(g, rl.GamepadButtonRightFaceRight) {
			return true
		}
	}
	return false
}

// poll returns at most one action per frame.
func (in *input) poll() action {
	in.notePrimary()
	in.collectPressed()
	if a, ok := in.dequeue(); ok {
		return a
	}
	return in.repeatHeld()
}

func (in *input) notePrimary() {
	before := in.primary
	defer in.logPrimaryChange(before)
	padUsed, pad := gamepadActive()
	in.active = padUsed
	in.padActive = padUsed
	if padUsed {
		in.primary, in.pad, in.usedInput = deviceGamepad, pad, true
		return
	}
	in.startupPad()
}

func (in *input) logPrimaryChange(before inputDevice) {
	if in.primary == before {
		return
	}
	lg().Debug("primary input changed", zap.Int("device", int(in.primary)), zap.Int32("pad", in.pad), zap.String("padName", in.padName()))
}

func (in *input) startupPad() {
	if in.usedInput {
		return
	}
	if rl.GetTime() >= startupPadWindow {
		return
	}
	if g, ok := firstGamepad(); ok {
		in.primary, in.pad = deviceGamepad, g
	}
}

func (in *input) collectPressed() {
	in.collectKeys()
	if gamepadActivatePressed() {
		in.queued = append(in.queued, actionActivate)
	}
	if gamepadBackPressed() {
		in.queued = append(in.queued, actionBack)
	}
	in.queued = append(in.queued, gamepadShoulders()...)
}

// gamepadShoulders turns LB and RB (L1 and R1) into volume down and up.
func gamepadShoulders() []action {
	var out []action
	for g := range int32(maxGamepads) {
		if !rl.IsGamepadAvailable(g) {
			continue
		}
		if rl.IsGamepadButtonPressed(g, rl.GamepadButtonLeftTrigger1) {
			out = append(out, actionVolumeDown)
		}
		if rl.IsGamepadButtonPressed(g, rl.GamepadButtonRightTrigger1) {
			out = append(out, actionVolumeUp)
		}
	}
	return out
}

func (in *input) collectKeys() {
	for k := rl.GetKeyPressed(); k != 0; k = rl.GetKeyPressed() {
		in.noteKey(k)
	}
}

func (in *input) noteKey(k int32) {
	in.active = true
	in.noteRecent(keyName(k))
	in.primary, in.usedInput = deviceKeyboard, true
	if a := keyAction(k); a != actionNone {
		in.queued = append(in.queued, a)
	}
}

func (in *input) dequeue() (action, bool) {
	if len(in.queued) == 0 {
		return actionNone, false
	}
	a := in.queued[0]
	in.queued = in.queued[1:]
	if a == in.directionHeld() {
		in.held = a
		in.nextFire = rl.GetTime() + repeatDelay
	}
	return a, true
}

func (in *input) repeatHeld() action {
	now := rl.GetTime()
	dir := in.directionHeld()
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
func anyGuideDown() bool {
	for g := range int32(maxGamepads) {
		if rl.IsGamepadAvailable(g) && rl.IsGamepadButtonDown(g, rl.GamepadButtonMiddle) {
			return true
		}
	}
	return false
}

func (in *input) guideHeldFor(seconds float64) bool {
	if !anyGuideDown() {
		in.guideHeld = 0
		return false
	}
	return in.guideHeldLongEnough(seconds)
}

func (in *input) guideHeldLongEnough(seconds float64) bool {
	now := rl.GetTime()
	if in.guideHeld == 0 {
		in.guideHeld = now
	}
	if now-in.guideHeld < seconds {
		return false
	}
	in.guideHeld = 0
	return true
}
