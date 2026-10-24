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
// biscuittin quits after the last step, and exits non-zero if a step fails.
// press and shot first wait until the screen is settled: icons and the
// background image loaded, and every animation finished. So scripts do not
// depend on timing, and a press is never swallowed by a fade. As with a
// person, the first press after the tiles are hidden (back on the home
// screen) only brings them back.

// scriptSettleTimeout is how long a step waits for the screen to settle.
const scriptSettleTimeout = 90.0

type scriptStep struct {
	line int
	cmd  string
	arg  string
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
	defer f.Close()

	sc := &script{}
	lines := bufio.NewScanner(f)
	for n := 1; lines.Scan(); n++ {
		line := strings.TrimSpace(lines.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmd, arg, _ := strings.Cut(line, " ")
		st := scriptStep{line: n, cmd: cmd, arg: strings.TrimSpace(arg)}
		if err := st.validate(); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		sc.steps = append(sc.steps, st)
	}
	return sc, lines.Err()
}

func (st scriptStep) validate() error {
	switch st.cmd {
	case "clock":
		if len(st.arg) != 5 || st.arg[2] != ':' {
			return fmt.Errorf("clock wants HH:MM, got %q", st.arg)
		}
	case "input":
		if _, ok := scriptPadNames[st.arg]; !ok && st.arg != "keyboard" {
			return fmt.Errorf("input wants xbox, playstation or keyboard, got %q", st.arg)
		}
	case "press":
		if _, ok := scriptButtons[st.arg]; !ok {
			return fmt.Errorf("unknown button %q", st.arg)
		}
	case "sleep":
		if v, err := strconv.ParseFloat(st.arg, 64); err != nil || v < 0 {
			return fmt.Errorf("sleep wants seconds, got %q", st.arg)
		}
	case "shot":
		if st.arg == "" {
			return fmt.Errorf("shot needs a file name")
		}
	default:
		return fmt.Errorf("unknown step %q", st.cmd)
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

	for sc.pos < len(sc.steps) {
		st := sc.steps[sc.pos]
		switch st.cmd {
		case "clock":
			u.clock = st.arg
		case "input":
			if st.arg == "keyboard" {
				in.primary, in.padOverride = deviceKeyboard, ""
			} else {
				in.primary, in.padOverride = deviceGamepad, scriptPadNames[st.arg]
			}
			in.usedInput = true // keep the startup gamepad check from overriding it
		case "press":
			if waiting, err := sc.waitSettled(u, ready, st, now); waiting || err != nil {
				return actionNone, false, err
			}
			sc.pos++
			return scriptButtons[st.arg], false, nil
		case "sleep":
			v, _ := strconv.ParseFloat(st.arg, 64)
			sc.pos++
			sc.waitUntil = now + v
			return actionNone, false, nil
		case "shot":
			if waiting, err := sc.waitSettled(u, ready, st, now); waiting || err != nil {
				return actionNone, false, err
			}
			sc.pos++
			u.captureTo = st.arg
			return actionNone, false, nil
		}
		sc.pos++
	}
	return actionNone, true, nil
}

// waitSettled reports whether st must keep waiting for the screen to settle.
// It needs a few settled frames in a row, so one-frame lulls don't count.
func (sc *script) waitSettled(u *ui, ready bool, st scriptStep, now float64) (bool, error) {
	if sc.settleSince == 0 {
		sc.settleSince = now
	}
	reason := u.unsettledReason(ready)
	if reason == "" {
		sc.settled++
	} else {
		sc.settled = 0
	}
	if sc.settled < 3 {
		if now-sc.settleSince > scriptSettleTimeout {
			return true, fmt.Errorf("line %d: screen did not settle within %.0fs (%s)", st.line, scriptSettleTimeout, reason)
		}
		return true, nil
	}
	sc.settleSince, sc.settled = 0, 0
	return false, nil
}

// unsettledReason says what is still loading or animating, or "" when the
// screen is settled.
func (u *ui) unsettledReason(ready bool) string {
	switch {
	case !ready:
		return "icons or background image still loading"
	case u.bgTex.fade < 1:
		return "background crossfade"
	case u.idle && u.bodyAlpha > 0, !u.idle && u.bodyAlpha < 1:
		return "idle fade"
	case u.idle && u.bodyAlpha == 0 && u.rssHaveCur && u.rssAnim < 1:
		return "rss slide"
	case u.toast != "" && rl.GetTime() < u.toastUntil:
		return "toast showing"
	}
	if u.screen != screenHome {
		return "" // tiles only animate while drawn
	}
	for i, t := range u.tiles {
		target := float32(1)
		if i == u.selected {
			target = 1.1
		}
		if u.scales[t.key] != target {
			return "tile animation"
		}
	}
	return ""
}

// capture saves the frame drawn so far as a PNG. Must run on the main thread,
// before EndDrawing swaps buffers.
func (u *ui) capture(path string) error {
	rl.DrawRenderBatchActive()
	img := rl.LoadImageFromScreen()
	defer rl.UnloadImage(img)

	w, h := int(img.Width), int(img.Height)
	if u.wantSize != (image.Point{}) && (w != u.wantSize.X || h != u.wantSize.Y) {
		return fmt.Errorf("window is %dx%d, not the %dx%d asked for; a window manager may have resized it", w, h, u.wantSize.X, u.wantSize.Y)
	}

	colors := rl.LoadImageColors(img)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, c := range colors {
		// The window is opaque; the framebuffer's alpha is not meaningful.
		out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = c.R, c.G, c.B, 255
	}
	rl.UnloadImageColors(colors)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, out); err != nil {
		f.Close()
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
	w, err1 := strconv.Atoi(ws)
	h, err2 := strconv.Atoi(hs)
	if !ok || err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return image.Point{}, fmt.Errorf("size %q: want WIDTHxHEIGHT, e.g. 1920x1080", s)
	}
	return image.Point{X: w, Y: h}, nil
}
