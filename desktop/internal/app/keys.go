package app

import (
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// recentKeysKept is how many presses Input devices diagnostics lists.
const recentKeysKept = 8

var keyNames = map[int32]string{
	rl.KeyUp:           "Up",
	rl.KeyDown:         "Down",
	rl.KeyLeft:         "Left",
	rl.KeyRight:        "Right",
	rl.KeyEnter:        "Enter",
	rl.KeyKpEnter:      "Keypad Enter",
	rl.KeySpace:        "Space",
	rl.KeyEscape:       "Esc",
	rl.KeyBackspace:    "Backspace",
	rl.KeyTab:          "Tab",
	rl.KeyInsert:       "Insert",
	rl.KeyDelete:       "Delete",
	rl.KeyHome:         "Home",
	rl.KeyEnd:          "End",
	rl.KeyPageUp:       "Page Up",
	rl.KeyPageDown:     "Page Down",
	rl.KeyLeftShift:    "Shift",
	rl.KeyRightShift:   "Shift",
	rl.KeyLeftControl:  "Ctrl",
	rl.KeyRightControl: "Ctrl",
	rl.KeyLeftAlt:      "Alt",
	rl.KeyRightAlt:     "Alt",
	rl.KeyLeftSuper:    "Super",
	rl.KeyRightSuper:   "Super",
	rl.KeyKbMenu:       "Menu",
	rl.KeyBack:         "Back",
	rl.KeyMenu:         "Menu",
	rl.KeyVolumeUp:     "Volume Up",
	rl.KeyVolumeDown:   "Volume Down",
}

// keyRanges name runs of keys: letters and digits by their own character,
// then function and keypad keys by number.
var keyRanges = []struct {
	name     func(k int32) string
	from, to int32
}{
	{func(k int32) string { return fmt.Sprintf("%c", k) }, rl.KeyA, rl.KeyZ},
	{func(k int32) string { return fmt.Sprintf("%c", k) }, rl.KeyZero, rl.KeyNine},
	{func(k int32) string { return fmt.Sprintf("F%d", k-rl.KeyF1+1) }, rl.KeyF1, rl.KeyF12},
	{func(k int32) string { return fmt.Sprintf("Keypad %d", k-rl.KeyKp0) }, rl.KeyKp0, rl.KeyKp9},
}

// keyName is a key's label, e.g. "Enter", "F5", "Q" or "Key 348".
func keyName(k int32) string {
	if n, ok := keyNames[k]; ok {
		return n
	}
	for _, r := range keyRanges {
		if k >= r.from && k <= r.to {
			return r.name(k)
		}
	}
	return fmt.Sprintf("Key %d", k)
}

// noteRecent remembers a press for Input devices diagnostics, newest last.
func (in *input) noteRecent(name string) {
	in.recent = append(in.recent, name)
	if len(in.recent) > recentKeysKept {
		in.recent = in.recent[len(in.recent)-recentKeysKept:]
	}
}

func (in *input) recentText() string {
	if len(in.recent) == 0 {
		return "Last keys: none yet"
	}
	return "Last keys: " + strings.Join(in.recent, "   ")
}

// drawKeysCard shows recent keyboard, remote and CEC presses, so a remote
// that acts as a keyboard can be checked. Esc, Backspace and Enter also
// leave the screen.
func (u *ui) drawKeysCard(margin, y, w, s float32) float32 {
	cardH := 140 * s
	r := rl.Rectangle{X: margin, Y: y, Width: w - 2*margin, Height: cardH}
	rl.DrawRectangleRounded(r, 0.12, 10, u.fa(rl.ColorAlpha(rl.White, 0.06)))
	u.text("Keyboard, remote and CEC", r.X+32*s, r.Y+24*s, 40*s, rl.RayWhite)
	text := "Last keys: none yet"
	if u.input != nil {
		text = u.input.recentText()
	}
	u.text(u.fitText(text, 28*s, r.Width-64*s), r.X+32*s, r.Y+84*s, 28*s, u.accentText())
	return y + cardH + 24*s
}
