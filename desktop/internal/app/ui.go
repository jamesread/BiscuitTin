package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/integrations"
	"github.com/jamesread/biscuittin/internal/rss"
	"github.com/jamesread/biscuittin/internal/settings"
	"rsc.io/qr"
)

type tileKind int

const (
	tileEntry tileKind = iota
	tileAction
	tileMore
)

// tile is one button on the home grid: a configured entry, a pinned OliveTin
// action, or the built-in More tile (always last).
type tile struct {
	kind     tileKind
	key      string
	entry    int
	action   integrations.PinnedAction
	title    string
	subtitle string
}

func entryKey(i int) string { return fmt.Sprintf("entry:%d", i) }

type ui struct {
	cfg       *config.Config
	font      rl.Font
	bg        color.RGBA
	accent    color.RGBA
	tileColor color.RGBA
	selected  int

	tiles   []tile
	icons   map[string]rl.Texture2D // by tile key
	scales  map[string]float32      // selection zoom, by tile key
	running []int                   // launched processes, by entry index
	// actionsRunning counts in-flight pinned actions by PinnedAction.key().
	actionsRunning map[string]int

	sources    []integrations.Source
	lists      map[string]*uiActionList
	listSource string

	webURL      string   // web remote address while web control is on
	debugSocket string   // local debug socket while listening
	qr          *qr.Code // QR code for webURL, cached
	qrURL       string

	input  *input                  // for the primary device in control help
	glyphs map[string]rl.Texture2D // gamepad button glyphs, by file name

	screen  screen
	menuSel int
	quit    bool

	confirmTitle string
	confirmLabel string
	confirmDo    func()
	confirmBack  screen

	settings        *settings.Settings
	settingsChanged bool
	bgTex           uiBackground
	rss             *rss.Fetcher
	rssCur          rss.Item
	rssPrev         rss.Item
	rssHaveCur      bool
	rssHavePrev     bool
	rssAnim         float32
	rssShownAt      float64
	rssSeen         map[string]bool

	// idle is set by the main loop when there has been no input for a while.
	// bodyAlpha fades the tiles, menus and hints towards 0 while idle, so the
	// background can be seen; drawAlpha is the multiplier currently applied
	// (the header is always drawn at 1).
	idle      bool
	bodyAlpha float32
	drawAlpha float32

	configPath string
	startedAt  time.Time

	// onTile marks text drawn on an opaque tile rather than the background,
	// which needs no contrast backing. surface is a translucent panel fill
	// under the text, folded into the contrast estimate.
	onTile  bool
	surface *surfaceTint

	toast      string
	toastColor color.RGBA
	toastUntil float64

	// Scripted runs (see script.go): clock replaces the real time when set;
	// captureTo is a PNG to save from the next frame, and captureErr its
	// failure. wantSize is the window size asked for with -size.
	clock      string
	captureTo  string
	captureErr error
	wantSize   image.Point
}

func newUI(cfg *config.Config, settings *settings.Settings) *ui {
	u := &ui{
		cfg:            cfg,
		settings:       settings,
		bodyAlpha:      1,
		drawAlpha:      1,
		startedAt:      time.Now(),
		bg:             contrast.ParseHex(cfg.Background, color.RGBA{20, 22, 31, 255}),
		accent:         contrast.ParseHex(cfg.Accent, color.RGBA{242, 177, 52, 255}),
		tileColor:      contrast.AccessibleTileColor(contrast.ParseHex(cfg.TileColor, color.RGBA{61, 90, 128, 255})),
		icons:          map[string]rl.Texture2D{},
		scales:         map[string]float32{},
		running:        make([]int, len(cfg.Entries)),
		actionsRunning: map[string]int{},
		sources:        integrations.NewSources(cfg),
		lists:          map[string]*uiActionList{},
	}
	u.rebuildTiles()
	u.glyphs = loadGlyphs()

	u.font = rl.GetFontDefault()
	if cfg.Font != "" {
		f := rl.LoadFontEx(cfg.Font, 128, fontCodepoints())
		if rl.IsFontValid(f) {
			rl.GenTextureMipmaps(&f.Texture)
			rl.SetTextureFilter(f.Texture, rl.FilterTrilinear)
			u.font = f
		}
	}

	for i, e := range cfg.Entries {
		if e.Icon != "" {
			u.setIcon(entryKey(i), e.Icon)
		}
	}

	return u
}

// fontCodepoints is Latin-1, Latin Extended-A and general punctuation, so
// RSS curly quotes, dashes and ellipses draw instead of "?".
func fontCodepoints() []int32 {
	var out []int32
	for r := rune(32); r <= 0x17F; r++ {
		out = append(out, int32(r))
	}
	for r := rune(0x2000); r <= 0x206F; r++ {
		out = append(out, int32(r))
	}
	return out
}

// rebuildTiles lays out entries, then pinned OliveTin actions, then More.
// Icons and animation state are keyed by tile, so they survive a rebuild.
func (u *ui) rebuildTiles() {
	u.tiles = u.tiles[:0]
	for i, e := range u.cfg.Entries {
		u.tiles = append(u.tiles, tile{kind: tileEntry, key: entryKey(i), entry: i, title: e.Title, subtitle: e.Subtitle})
	}
	for _, a := range u.settings.PinnedActions {
		u.tiles = append(u.tiles, tile{kind: tileAction, key: a.Key(), action: a, title: a.Title, subtitle: a.Group})
	}
	u.tiles = append(u.tiles, tile{kind: tileMore, key: "more", title: "More", subtitle: "Settings and info"})

	for _, t := range u.tiles {
		if _, ok := u.scales[t.key]; !ok {
			u.scales[t.key] = 1
		}
	}
	if u.selected >= len(u.tiles) {
		u.selected = len(u.tiles) - 1
	}
}

// setIcon loads an image file as a tile's icon. Must run on the main thread.
func (u *ui) setIcon(key, path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	t := rl.LoadTexture(path)
	if !rl.IsTextureValid(t) {
		return
	}
	rl.GenTextureMipmaps(&t)
	rl.SetTextureFilter(t, rl.FilterTrilinear)
	if old := u.icons[key]; old.ID != 0 {
		rl.UnloadTexture(old)
	}
	u.icons[key] = t
}

func (u *ui) tileCount() int {
	return len(u.tiles)
}

func (u *ui) columns() int {
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	s := layoutScale(w, h)
	margin := 90 * s
	gap := 56 * s
	gridW := w - 2*margin
	if u.qrVisible() {
		gridW -= qrSide*s + 48*s
	}
	return columnsFit(u.cfg.Columns, u.tileCount(), gridW, gap, 200*s)
}

func (u *ui) move(a action) {
	n := u.tileCount()
	cols := u.columns()
	switch a {
	case actionLeft:
		if u.selected > 0 {
			u.selected--
		}
	case actionRight:
		if u.selected < n-1 {
			u.selected++
		}
	case actionUp:
		if u.selected-cols >= 0 {
			u.selected -= cols
		}
	case actionDown:
		if u.selected+cols < n {
			u.selected += cols
		} else if u.selected/cols < (n-1)/cols {
			u.selected = n - 1
		}
	}
}

func (u *ui) setToast(msg string, c color.RGBA, seconds float64) {
	u.toast = msg
	u.toastColor = c
	u.toastUntil = rl.GetTime() + seconds
}

// fa applies the current fade to a colour.
func (u *ui) fa(c color.RGBA) color.RGBA {
	c.A = uint8(float32(c.A) * u.drawAlpha)
	return c
}

const (
	idleFadeOutSeconds = 1.5
	idleFadeInSeconds  = 0.2
)

func (u *ui) updateFade(dt float32) {
	if u.idle {
		u.bodyAlpha -= dt / idleFadeOutSeconds
	} else {
		u.bodyAlpha += dt / idleFadeInSeconds
	}
	u.bodyAlpha = float32(math.Max(0, math.Min(1, float64(u.bodyAlpha))))
}

type surfaceTint struct {
	c contrast.RGB
	a float64
}

// text draws with a soft drop shadow. Unless it sits on a tile, it is checked
// against the brightest part of what is behind it and given a translucent
// black backing (and more opacity if needed) to meet WCAG AA contrast.
func (u *ui) text(s string, x, y, size float32, c color.RGBA) {
	if s == "" || c.A == 0 || u.drawAlpha <= 0 {
		return
	}

	if !u.onTile {
		m := u.measure(s, size)
		padX, padY := size*0.35, size*0.14
		r := rl.Rectangle{X: x - padX, Y: y - padY, Width: m.X + 2*padX, Height: m.Y + 2*padY}

		backdrop := u.backdropFor(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
		backing, ta := contrast.SolveBacking(contrast.RGBOf(c), float64(c.A)/255, backdrop)
		if backing > 0.01 {
			rl.DrawRectangleRounded(r, 0.5, 8, rl.ColorAlpha(rl.Black, float32(backing)*u.drawAlpha))
		}
		c.A = uint8(math.Round(ta * 255))
	}

	c = u.fa(c)
	off := size / 24
	shadow := rl.ColorAlpha(rl.Black, 0.55*float32(c.A)/255)
	rl.DrawTextEx(u.font, s, rl.Vector2{X: x + off, Y: y + off}, size, size/32, shadow)
	rl.DrawTextEx(u.font, s, rl.Vector2{X: x, Y: y}, size, size/32, c)
}

func (u *ui) measure(s string, size float32) rl.Vector2 {
	return rl.MeasureTextEx(u.font, s, size, size/32)
}

func (u *ui) draw() {
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	s := layoutScale(w, h)
	dt := rl.GetFrameTime()

	u.updateFade(dt)

	rl.BeginDrawing()
	rl.ClearBackground(u.bg)
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), u.bg, contrast.Shade(u.bg, 0.55))
	rl.DrawCircleGradient(rl.Vector2{X: w / 2, Y: -h * 0.1}, w*0.55, rl.ColorAlpha(u.accent, 0.16), rl.ColorAlpha(u.accent, 0))
	u.drawBackground(w, h, dt)

	margin := 90 * s

	// Header: always fully visible.
	u.drawAlpha = 1
	clock := time.Now().Format("15:04")
	if u.clock != "" {
		clock = u.clock
	}
	clockSize := 56 * s
	titleSize := 72 * s
	cw := u.measure(clock, clockSize)
	title := u.fitText(u.cfg.Title, titleSize, w-2*margin-cw.X-40*s)
	u.text(title, margin, 60*s, titleSize, rl.RayWhite)
	u.text(clock, w-margin-cw.X, 68*s, clockSize, rl.ColorAlpha(rl.RayWhite, 0.7))
	u.drawAlpha = u.bodyAlpha

	if u.screen != screenHome {
		u.drawMenu(w, h, s, margin)
		u.drawFooter(w, h, s, margin)
		u.endFrame()
		return
	}

	// Grid
	n := u.tileCount()
	cols := u.columns()
	rows := (n + cols - 1) / cols
	gap := 56 * s
	// Tiles fill the width between the margins, less the QR code's column
	// when it is shown.
	gridLeft, gridRight := margin, w-margin
	if u.qrVisible() {
		gridRight -= qrSide*s + 48*s
	}
	gridW := gridRight - gridLeft
	tileW := float32(math.Min(float64((gridW-float32(cols-1)*gap)/float32(cols)), float64(380*s)))
	tileH := tileW * 1.05
	labelH := 90 * s
	rowH := tileH + labelH
	rowGap := 30 * s

	areaTop := 210 * s
	areaBottom := h - 130*s
	areaH := areaBottom - areaTop
	totalH := float32(rows)*rowH + float32(rows-1)*rowGap

	y0 := areaTop + (areaH-totalH)/2
	if totalH > areaH {
		selRow := float32(u.selected / cols)
		y0 = areaTop + areaH/2 - (selRow*(rowH+rowGap) + rowH/2)
		y0 = float32(math.Min(float64(y0), float64(areaTop)))
		y0 = float32(math.Max(float64(y0), float64(areaBottom-totalH)))
	}

	for i := 0; i < n; i++ {
		target := float32(1)
		if i == u.selected {
			target = 1.1
		}
		key := u.tiles[i].key
		u.scales[key] += (target - u.scales[key]) * float32(math.Min(1, float64(dt*14)))
		// Snap the tail of the ease, so a settled screen is exactly settled.
		if math.Abs(float64(target-u.scales[key])) < 0.001 {
			u.scales[key] = target
		}

		row := i / cols
		col := i % cols
		inRow := cols
		if row == rows-1 {
			inRow = n - row*cols
		}
		rowW := float32(inRow)*tileW + float32(inRow-1)*gap
		x := gridLeft + (gridW-rowW)/2 + float32(col)*(tileW+gap)
		y := y0 + float32(row)*(rowH+rowGap)

		u.drawTile(i, x, y, tileW, tileH, s)
	}

	u.drawQR(w, h, s, margin)
	u.drawRSS(w, h, s, margin)
	u.drawFooter(w, h, s, margin)
	u.endFrame()
}

func (u *ui) drawFooter(w, h, s, margin float32) {
	u.drawHints(h, s, margin)

	// The image credit and status messages stay visible while idle.
	u.drawAlpha = 1
	u.drawCredit(w, h, s, margin)
	if u.toast != "" && rl.GetTime() < u.toastUntil {
		toast := u.fitText(u.toast, 36*s, w-2*margin)
		ts := u.measure(toast, 36*s)
		u.text(toast, w-margin-ts.X, h-86*s, 36*s, u.toastColor)
	}
	u.drawAlpha = u.bodyAlpha
}

func (u *ui) tileRunning(t tile) bool {
	switch t.kind {
	case tileEntry:
		return u.running[t.entry] > 0
	case tileAction:
		return u.actionsRunning[t.key] > 0
	}
	return false
}

// fitText shortens s with "..." so it fits within maxW at size.
func (u *ui) fitText(s string, size, maxW float32) string {
	if u.measure(s, size).X <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		t := strings.TrimRight(string(r), " ") + "..."
		if u.measure(t, size).X <= maxW {
			return t
		}
	}
	return "..."
}

func (u *ui) drawTile(i int, x, y, tw, th, s float32) {
	tl := u.tiles[i]
	title, subtitle := tl.title, tl.subtitle
	sc := u.scales[tl.key]
	selected := i == u.selected

	cx, cy := x+tw/2, y+th/2
	rw, rh := tw*sc, th*sc
	rect := rl.Rectangle{X: cx - rw/2, Y: cy - rh/2, Width: rw, Height: rh}

	shadow := rect
	shadow.Y += 14 * s
	rl.DrawRectangleRounded(shadow, 0.12, 12, u.fa(rl.ColorAlpha(rl.Black, 0.35)))

	base := u.tileColor
	if !selected {
		base = contrast.Shade(base, 0.6)
	}
	rl.DrawRectangleRounded(rect, 0.12, 12, u.fa(base))

	// Soft top highlight
	hl := rect
	hl.Height = rect.Height * 0.5
	rl.DrawRectangleRounded(hl, 0.24, 12, u.fa(rl.ColorAlpha(rl.White, contrast.TileHighlightAlpha)))

	if t := u.icons[tl.key]; t.ID != 0 {
		pad := rw * 0.16
		maxW, maxH := rw-2*pad, rh-2*pad
		k := float32(math.Min(float64(maxW/float32(t.Width)), float64(maxH/float32(t.Height))))
		dw, dh := float32(t.Width)*k, float32(t.Height)*k
		rl.DrawTexturePro(t,
			rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
			rl.Rectangle{X: cx - dw/2, Y: cy - dh/2, Width: dw, Height: dh},
			rl.Vector2{}, 0, u.fa(rl.White))
	} else {
		r, _ := utf8.DecodeRuneInString(title)
		letter := strings.ToUpper(string(r))
		if tl.kind == tileMore {
			letter = "..."
		}
		size := rh * 0.55
		m := u.measure(letter, size)
		u.onTile = true
		u.text(letter, cx-m.X/2, cy-m.Y/2, size, rl.ColorAlpha(rl.White, contrast.TileGlyphAlpha))
		u.onTile = false
	}

	if selected {
		rl.DrawRectangleRoundedLinesEx(rect, 0.12, 12, 6*s, u.fa(u.accent))
	}

	if u.tileRunning(tl) {
		rl.DrawCircleV(rl.Vector2{X: rect.X + rect.Width - 28*s, Y: rect.Y + 28*s}, 10*s, u.fa(u.accent))
	}

	labelSize := 40 * s
	labelColor := rl.ColorAlpha(rl.RayWhite, 0.6)
	if selected {
		labelColor = rl.RayWhite
	}
	maxLabel := tw + 40*s
	title = u.fitText(title, labelSize, maxLabel)
	subtitle = u.fitText(subtitle, 26*s, maxLabel)
	m := u.measure(title, labelSize)
	ly := y + th*1.05 + 20*s
	u.text(title, cx-m.X/2, ly, labelSize, labelColor)

	if subtitle != "" && selected {
		ss := 26 * s
		sm := u.measure(subtitle, ss)
		u.text(subtitle, cx-sm.X/2, ly+labelSize+4*s, ss, rl.ColorAlpha(rl.RayWhite, 0.5))
	}
}

func (u *ui) unload() {
	u.unloadBackground()
	for _, t := range u.glyphs {
		rl.UnloadTexture(t)
	}
	for _, t := range u.icons {
		if t.ID != 0 {
			rl.UnloadTexture(t)
		}
	}
	if u.font.Texture.ID != rl.GetFontDefault().Texture.ID {
		rl.UnloadFont(u.font)
	}
}

func describeExit(e config.Entry, err error) string {
	if err == nil {
		return fmt.Sprintf("%s exited", e.Title)
	}
	return fmt.Sprintf("%s exited: %v", e.Title, err)
}
