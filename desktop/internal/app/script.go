package app

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// A script drives the UI without a person, for repeatable screenshots
// (see var/marketing/screenshots). It is a text file with one step per line;
// blank lines and lines starting with # are ignored:
//
//	clock 20:15          show this time instead of the real clock
//	input xbox           show gamepad hints: xbox, playstation or keyboard
//	press right          up, down, left, right, select or back
//	sleep 0.5            wait this many seconds
//	shot out/home.png    save the window as a PNG
//
// BiscuitTin quits after the last step, and exits non-zero if a step fails.
// press and shot first wait until the screen is settled: icons and the
// background image loaded, and every animation finished. So scripts do not
// depend on timing, and a press is never swallowed by a fade. As with a
// person, the first press after the tiles are hidden (back on the home
// screen) only brings them back.

// scriptSettleTimeout is how long a step waits for the screen to settle.
const scriptSettleTimeout = 90.0

type scriptStep struct {
	cmd  string
	arg  string
	line int
}

type script struct {
	steps []scriptStep
	pos   int

	waitUntil   float64 // sleep: GetTime to resume at
	settleSince float64 // shot: GetTime the wait started, 0 when not waiting
	settled     int     // shot: consecutive settled frames
}

var scriptButtons = map[string]action{
	"up":     actionUp,
	"down":   actionDown,
	"left":   actionLeft,
	"right":  actionRight,
	"select": actionActivate,
	"back":   actionBack,
}

// scriptPadNames stand in for a connected gamepad's name, which picks the
// button glyphs.
var scriptPadNames = map[string]string{
	"xbox":        "Xbox Wireless Controller",
	"playstation": "DualSense Wireless Controller",
}

func loadScript(path string) (*script, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseScript(f, path)
}

func skipScriptLine(line string) bool {
	if line == "" {
		return true
	}
	return strings.HasPrefix(line, "#")
}

func parseScript(f *os.File, path string) (*script, error) {
	sc := &script{}
	lines := bufio.NewScanner(f)
	for n := 1; lines.Scan(); n++ {
		if err := appendScriptLine(sc, path, n, lines.Text()); err != nil {
			return nil, err
		}
	}
	return sc, lines.Err()
}

func appendScriptLine(sc *script, path string, n int, line string) error {
	line = strings.TrimSpace(line)
	if skipScriptLine(line) {
		return nil
	}
	cmd, arg, _ := strings.Cut(line, " ")
	st := scriptStep{line: n, cmd: cmd, arg: strings.TrimSpace(arg)}
	if err := st.validate(); err != nil {
		return fmt.Errorf("%s:%d: %w", path, n, err)
	}
	sc.steps = append(sc.steps, st)
	return nil
}

var scriptValidators = map[string]func(string) error{
	"clock": validateClock,
	"input": validateInput,
	"press": validatePress,
	"sleep": validateSleep,
	"shot":  validateShot,
}

func (st scriptStep) validate() error {
	fn, ok := scriptValidators[st.cmd]
	if !ok {
		return fmt.Errorf("unknown step %q", st.cmd)
	}
	return fn(st.arg)
}

func validateClock(arg string) error {
	if len(arg) != 5 {
		return fmt.Errorf("clock wants HH:MM, got %q", arg)
	}
	if arg[2] != ':' {
		return fmt.Errorf("clock wants HH:MM, got %q", arg)
	}
	return nil
}

func validateInput(arg string) error {
	if arg == "keyboard" {
		return nil
	}
	if _, ok := scriptPadNames[arg]; ok {
		return nil
	}
	return fmt.Errorf("input wants xbox, playstation or keyboard, got %q", arg)
}

func validatePress(arg string) error {
	if _, ok := scriptButtons[arg]; ok {
		return nil
	}
	return fmt.Errorf("unknown button %q", arg)
}

func validateSleep(arg string) error {
	v, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return fmt.Errorf("sleep wants seconds, got %q", arg)
	}
	if v < 0 {
		return fmt.Errorf("sleep wants seconds, got %q", arg)
	}
	return nil
}

func validateShot(arg string) error {
	if arg == "" {
		return fmt.Errorf("shot needs a file name")
	}
	return nil
}

// step runs once per frame, before drawing. It returns an action to apply
// this frame, and done once the last step has finished. ready reports work
// outside the UI (icon lookups, background loading) still to finish.
func (sc *script) step(u *ui, in *input, ready bool) (a action, done bool, err error) {
	now := rl.GetTime()
	if now < sc.waitUntil {
		return actionNone, false, nil
	}
	if u.captureErr != nil {
		return actionNone, false, u.captureErr
	}
	return sc.runSteps(u, in, ready, now)
}

func (sc *script) runSteps(u *ui, in *input, ready bool, now float64) (action, bool, error) {
	for sc.pos < len(sc.steps) {
		a, err, cont := sc.runStep(u, in, ready, now)
		if !cont {
			return a, false, err
		}
	}
	return actionNone, true, nil
}

var scriptInstant = map[string]func(*ui, *input, scriptStep){
	"clock": func(u *ui, _ *input, st scriptStep) { u.clock = st.arg },
	"input": func(_ *ui, in *input, st scriptStep) { applyScriptInput(in, st.arg) },
}

func applyScriptInput(in *input, arg string) {
	if arg == "keyboard" {
		in.primary, in.padOverride = deviceKeyboard, ""
	} else {
		in.primary, in.padOverride = deviceGamepad, scriptPadNames[arg]
	}
	in.usedInput = true // keep the startup gamepad check from overriding it
}

func (sc *script) runStep(u *ui, in *input, ready bool, now float64) (a action, err error, cont bool) {
	st := sc.steps[sc.pos]
	if fn, ok := scriptInstant[st.cmd]; ok {
		fn(u, in, st)
		sc.pos++
		return actionNone, nil, true
	}
	return sc.runBlocking(u, ready, st, now)
}

func (sc *script) runBlocking(u *ui, ready bool, st scriptStep, now float64) (action, error, bool) {
	switch st.cmd {
	case "press":
		return sc.doPress(u, ready, st, now)
	case "sleep":
		return sc.doSleep(st, now)
	case "shot":
		return sc.doShot(u, ready, st, now)
	}
	sc.pos++
	return actionNone, nil, true
}

func (sc *script) doPress(u *ui, ready bool, st scriptStep, now float64) (action, error, bool) {
	if waiting, err := sc.waitSettled(u, ready, st, now); waiting {
		return actionNone, err, false
	}
	sc.pos++
	return scriptButtons[st.arg], nil, false
}

func (sc *script) doSleep(st scriptStep, now float64) (action, error, bool) {
	v, _ := strconv.ParseFloat(st.arg, 64)
	sc.pos++
	sc.waitUntil = now + v
	return actionNone, nil, false
}

func (sc *script) doShot(u *ui, ready bool, st scriptStep, now float64) (action, error, bool) {
	if waiting, err := sc.waitSettled(u, ready, st, now); waiting {
		return actionNone, err, false
	}
	sc.pos++
	u.captureTo = st.arg
	return actionNone, nil, false
}

// waitSettled reports whether st must keep waiting for the screen to settle.
// It needs a few settled frames in a row, so one-frame lulls don't count.
func (sc *script) waitSettled(u *ui, ready bool, st scriptStep, now float64) (bool, error) {
	if sc.settleSince == 0 {
		sc.settleSince = now
	}
	reason := u.unsettledReason(ready)
	sc.countSettled(reason)
	if sc.settled >= 3 {
		sc.settleSince, sc.settled = 0, 0
		return false, nil
	}
	return true, sc.settleTimeout(st, now, reason)
}

func (sc *script) countSettled(reason string) {
	if reason == "" {
		sc.settled++
		return
	}
	sc.settled = 0
}

func (sc *script) settleTimeout(st scriptStep, now float64, reason string) error {
	if now-sc.settleSince > scriptSettleTimeout {
		return fmt.Errorf("line %d: screen did not settle within %.0fs (%s)", st.line, scriptSettleTimeout, reason)
	}
	return nil
}

// unsettledReason says what is still loading or animating, or "" when the
// screen is settled.
func (u *ui) unsettledReason(ready bool) string {
	if !ready {
		return "icons or background image still loading"
	}
	if reason := u.animReason(); reason != "" {
		return reason
	}
	return u.tileAnimReason()
}

func (u *ui) animReason() string {
	if u.bgTex.fade < 1 {
		return "background crossfade"
	}
	return u.fadeReason()
}

func (u *ui) fadeReason() string {
	if u.idleFading() {
		return "idle fade"
	}
	if u.rssSliding() {
		return "rss slide"
	}
	if u.toastVisible() {
		return "toast showing"
	}
	return ""
}

func (u *ui) idleFading() bool {
	if u.idle {
		return u.bodyAlpha > 0
	}
	return u.bodyAlpha < 1
}

func (u *ui) rssSliding() bool {
	if !u.idle {
		return false
	}
	if u.bodyAlpha != 0 {
		return false
	}
	if !u.rssHaveCur {
		return false
	}
	return u.rssAnim < 1
}

func (u *ui) toastVisible() bool {
	if u.toast == "" {
		return false
	}
	return rl.GetTime() < u.toastUntil
}

func (u *ui) tileAnimReason() string {
	if u.screen != screenHome {
		return "" // tiles only animate while drawn
	}
	for i, t := range u.tiles {
		if u.scales[t.key] != tileTargetScale(i == u.selected) {
			return "tile animation"
		}
	}
	return ""
}

func tileTargetScale(selected bool) float32 {
	if selected {
		return 1.1
	}
	return 1
}

// capture saves the frame drawn so far as a PNG. Must run on the main thread,
// before EndDrawing swaps buffers.
func (u *ui) capture(path string) error {
	rl.DrawRenderBatchActive()
	img := rl.LoadImageFromScreen()
	defer rl.UnloadImage(img)
	if err := u.checkCaptureSize(int(img.Width), int(img.Height)); err != nil {
		return err
	}
	return writePNG(path, screenshotRGBA(img))
}

func (u *ui) checkCaptureSize(w, h int) error {
	if u.wantSize == (image.Point{}) {
		return nil
	}
	if w == u.wantSize.X && h == u.wantSize.Y {
		return nil
	}
	return fmt.Errorf("window is %dx%d, not the %dx%d asked for; a window manager may have resized it", w, h, u.wantSize.X, u.wantSize.Y)
}

func screenshotRGBA(img *rl.Image) *image.RGBA {
	w, h := int(img.Width), int(img.Height)
	colors := rl.LoadImageColors(img)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, c := range colors {
		// The window is opaque; the framebuffer's alpha is not meaningful.
		out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = c.R, c.G, c.B, 255
	}
	rl.UnloadImageColors(colors)
	return out
}

func writePNG(path string, out *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return encodePNG(f, out)
}

func encodePNG(f *os.File, out *image.RGBA) error {
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, out); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// endFrame takes any pending capture, then presents the frame.
func (u *ui) endFrame() {
	if u.captureTo != "" {
		if err := u.capture(u.captureTo); err != nil {
			u.captureErr = fmt.Errorf("shot %s: %w", u.captureTo, err)
		}
		u.captureTo = ""
	}
	rl.EndDrawing()
}

// ParseSize parses "1920x1080".
func ParseSize(s string) (image.Point, error) {
	ws, hs, ok := strings.Cut(s, "x")
	if !ok {
		return sizeErr(s)
	}
	w, err1 := strconv.Atoi(ws)
	h, err2 := strconv.Atoi(hs)
	if err1 != nil || err2 != nil {
		return sizeErr(s)
	}
	return validSize(s, w, h)
}

func validSize(s string, w, h int) (image.Point, error) {
	if w <= 0 {
		return sizeErr(s)
	}
	if h <= 0 {
		return sizeErr(s)
	}
	return image.Point{X: w, Y: h}, nil
}

func sizeErr(s string) (image.Point, error) {
	return image.Point{}, fmt.Errorf("size %q: want WIDTHxHEIGHT, e.g. 1920x1080", s)
}
