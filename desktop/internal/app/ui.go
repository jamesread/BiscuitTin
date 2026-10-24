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
	key      string
	title    string
	subtitle string
	action   integrations.PinnedAction
	kind     tileKind
	entry    int
}

func entryKey(i int) string { return fmt.Sprintf("entry:%d", i) }

type ui struct {
	captureErr      error
	scales          map[string]float32
	rssSeen         map[string]bool
	icons           map[string]rl.Texture2D
	input           *input
	surface         *surfaceTint
	actionsRunning  map[string]int
	rss             *rss.Fetcher
	lists           map[string]*uiActionList
	cfg             *config.Config
	settings        *settings.Settings
	confirmDo       func()
	qr              *qr.Code
	glyphs          map[string]rl.Texture2D
	sources         []integrations.Source
	running         []int
	tiles           []tile
	listSource      string
	captureTo       string
	toast           string
	qrURL           string
	clock           string
	confirmTitle    string
	confirmLabel    string
	debugSocket     string
	configPath      string
	webURL          string
	rssPrev         rss.Item
	rssCur          rss.Item
	font            rl.Font
	startedAt       time.Time
	bgTex           uiBackground
	wantSize        image.Point
	confirmBack     screen
	toastUntil      float64
	menuSel         int
	rssShownAt      float64
	selected        int
	screen          screen
	rssAnim         float32
	drawAlpha       float32
	bodyAlpha       float32
	tileColor       color.RGBA
	accent          color.RGBA
	toastColor      color.RGBA
	bg              color.RGBA
	onTile          bool
	idle            bool
	rssHavePrev     bool
	rssHaveCur      bool
	settingsChanged bool
	quit            bool
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
		out = append(out, r)
	}
	for r := rune(0x2000); r <= 0x206F; r++ {
		out = append(out, r)
	}
	return out
}

// rebuildTiles lays out entries, then pinned OliveTin actions, then More.
// Icons and animation state are keyed by tile, so they survive a rebuild.
func (u *ui) rebuildTiles() {
	u.tiles = u.tiles[:0]
	u.appendEntryTiles()
	u.appendPinnedTiles()
	u.tiles = append(u.tiles, tile{kind: tileMore, key: "more", title: "More", subtitle: "Settings and info"})
	u.resetTileScales()
	if u.selected >= len(u.tiles) {
		u.selected = len(u.tiles) - 1
	}
}

func (u *ui) appendEntryTiles() {
	for i, e := range u.cfg.Entries {
		u.tiles = append(u.tiles, tile{kind: tileEntry, key: entryKey(i), entry: i, title: e.Title, subtitle: e.Subtitle})
	}
}

func (u *ui) appendPinnedTiles() {
	for _, a := range u.settings.PinnedActions {
		u.tiles = append(u.tiles, tile{kind: tileAction, key: a.Key(), action: a, title: a.Title, subtitle: a.Group})
	}
}

func (u *ui) resetTileScales() {
	for _, t := range u.tiles {
		if _, ok := u.scales[t.key]; !ok {
			u.scales[t.key] = 1
		}
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

func horizStep(a action) int {
	if a == actionLeft {
		return -1
	}
	return 1
}

func (u *ui) nudgeSelected(delta, n int) {
	next := u.selected + delta
	if next < 0 {
		return
	}
	if next >= n {
		return
	}
	u.selected = next
}

func (u *ui) moveDown(n, cols int) {
	if u.selected+cols < n {
		u.selected += cols
		return
	}
	if u.selected/cols < (n-1)/cols {
		u.selected = n - 1
	}
}

func (u *ui) move(a action) {
	n, cols := u.tileCount(), u.columns()
	switch a {
	case actionLeft, actionRight:
		u.nudgeSelected(horizStep(a), n)
	case actionUp:
		u.nudgeSelected(-cols, n)
	case actionDown:
		u.moveDown(n, cols)
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
func (u *ui) shouldDrawText(s string, c color.RGBA) bool {
	if s == "" {
		return false
	}
	if c.A == 0 {
		return false
	}
	return u.drawAlpha > 0
}

func (u *ui) ensureTextContrast(s string, x, y, size float32, c color.RGBA) color.RGBA {
	m := u.measure(s, size)
	padX, padY := size*0.35, size*0.14
	r := rl.Rectangle{X: x - padX, Y: y - padY, Width: m.X + 2*padX, Height: m.Y + 2*padY}
	backdrop := u.backdropFor(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
	backing, ta := contrast.SolveBacking(contrast.RGBOf(c), float64(c.A)/255, backdrop)
	if backing > 0.01 {
		rl.DrawRectangleRounded(r, 0.5, 8, rl.ColorAlpha(rl.Black, float32(backing)*u.drawAlpha))
	}
	c.A = uint8(math.Round(ta * 255))
	return c
}

func (u *ui) paintText(s string, x, y, size float32, c color.RGBA) {
	c = u.fa(c)
	off := size / 24
	shadow := rl.ColorAlpha(rl.Black, 0.55*float32(c.A)/255)
	rl.DrawTextEx(u.font, s, rl.Vector2{X: x + off, Y: y + off}, size, size/32, shadow)
	rl.DrawTextEx(u.font, s, rl.Vector2{X: x, Y: y}, size, size/32, c)
}

func (u *ui) text(s string, x, y, size float32, c color.RGBA) {
	if !u.shouldDrawText(s, c) {
		return
	}
	if !u.onTile {
		c = u.ensureTextContrast(s, x, y, size, c)
	}
	u.paintText(s, x, y, size, c)
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
	u.drawBackdrop(w, h, dt)
	margin := 90 * s
	u.drawHeader(w, s, margin)
	u.drawBody(w, h, s, margin, dt)
	u.endFrame()
}

func (u *ui) drawBackdrop(w, h, dt float32) {
	rl.BeginDrawing()
	rl.ClearBackground(u.bg)
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), u.bg, contrast.Shade(u.bg, 0.55))
	rl.DrawCircleGradient(rl.Vector2{X: w / 2, Y: -h * 0.1}, w*0.55, rl.ColorAlpha(u.accent, 0.16), rl.ColorAlpha(u.accent, 0))
	u.drawBackground(w, h, dt)
}

func headerClock(override string) string {
	if override != "" {
		return override
	}
	return time.Now().Format("15:04")
}

func (u *ui) drawHeader(w, s, margin float32) {
	u.drawAlpha = 1
	clock := headerClock(u.clock)
	clockSize := 56 * s
	titleSize := 72 * s
	cw := u.measure(clock, clockSize)
	title := u.fitText(u.cfg.Title, titleSize, w-2*margin-cw.X-40*s)
	u.text(title, margin, 60*s, titleSize, rl.RayWhite)
	u.text(clock, w-margin-cw.X, 68*s, clockSize, rl.ColorAlpha(rl.RayWhite, 0.7))
	u.drawAlpha = u.bodyAlpha
}

func (u *ui) drawBody(w, h, s, margin, dt float32) {
	if u.screen != screenHome {
		u.drawMenu(w, h, s, margin)
		u.drawFooter(w, h, s, margin)
		return
	}
	u.drawHome(w, h, s, margin, dt)
}

type homeGrid struct {
	n, cols, rows                                           int
	gap, tileW, tileH, rowH, rowGap, gridLeft, gridW, y0, s float32
}

func gridY0(areaTop, areaBottom, areaH, totalH float32, selected, cols int, rowH, rowGap float32) float32 {
	y0 := areaTop + (areaH-totalH)/2
	if totalH <= areaH {
		return y0
	}
	selRow := float32(selected / cols)
	y0 = areaTop + areaH/2 - (selRow*(rowH+rowGap) + rowH/2)
	y0 = float32(math.Min(float64(y0), float64(areaTop)))
	return float32(math.Max(float64(y0), float64(areaBottom-totalH)))
}

func (u *ui) homeGrid(w, h, s, margin float32) homeGrid {
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
	rowH := tileH + 90*s
	rowGap := 30 * s
	areaTop := 210 * s
	areaBottom := h - 130*s
	areaH := areaBottom - areaTop
	totalH := float32(rows)*rowH + float32(rows-1)*rowGap
	return homeGrid{
		n: n, cols: cols, rows: rows, gap: gap, tileW: tileW, tileH: tileH,
		rowH: rowH, rowGap: rowGap, gridLeft: gridLeft, gridW: gridW,
		y0: gridY0(areaTop, areaBottom, areaH, totalH, u.selected, cols, rowH, rowGap), s: s,
	}
}

func (u *ui) easeTile(i int, dt float32) {
	target := tileTargetScale(i == u.selected)
	key := u.tiles[i].key
	u.scales[key] += (target - u.scales[key]) * float32(math.Min(1, float64(dt*14)))
	// Snap the tail of the ease, so a settled screen is exactly settled.
	if math.Abs(float64(target-u.scales[key])) < 0.001 {
		u.scales[key] = target
	}
}

func tilePos(g homeGrid, i int) (x, y float32) {
	row := i / g.cols
	col := i % g.cols
	inRow := g.cols
	if row == g.rows-1 {
		inRow = g.n - row*g.cols
	}
	rowW := float32(inRow)*g.tileW + float32(inRow-1)*g.gap
	x = g.gridLeft + (g.gridW-rowW)/2 + float32(col)*(g.tileW+g.gap)
	y = g.y0 + float32(row)*(g.rowH+g.rowGap)
	return x, y
}

func (u *ui) drawHomeTiles(g homeGrid, dt float32) {
	for i := range g.n {
		u.easeTile(i, dt)
		x, y := tilePos(g, i)
		u.drawTile(i, x, y, g.tileW, g.tileH, g.s)
	}
}

func (u *ui) drawHome(w, h, s, margin, dt float32) {
	u.drawHomeTiles(u.homeGrid(w, h, s, margin), dt)
	u.drawQR(w, h, s, margin)
	u.drawRSS(w, h, s, margin)
	u.drawFooter(w, h, s, margin)
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

func tileBaseColor(base rl.Color, selected bool) rl.Color {
	if selected {
		return base
	}
	return contrast.Shade(base, 0.6)
}

func tileLetter(tl tile) string {
	if tl.kind == tileMore {
		return "..."
	}
	r, _ := utf8.DecodeRuneInString(tl.title)
	return strings.ToUpper(string(r))
}

func tileLabelColor(selected bool) rl.Color {
	if selected {
		return rl.RayWhite
	}
	return rl.ColorAlpha(rl.RayWhite, 0.6)
}

func (u *ui) paintTileCard(rect rl.Rectangle, selected bool, s float32) {
	shadow := rect
	shadow.Y += 14 * s
	rl.DrawRectangleRounded(shadow, 0.12, 12, u.fa(rl.ColorAlpha(rl.Black, 0.35)))
	rl.DrawRectangleRounded(rect, 0.12, 12, u.fa(tileBaseColor(u.tileColor, selected)))
	hl := rect
	hl.Height = rect.Height * 0.5
	rl.DrawRectangleRounded(hl, 0.24, 12, u.fa(rl.ColorAlpha(rl.White, contrast.TileHighlightAlpha)))
}

func (u *ui) paintTileIcon(t rl.Texture2D, cx, cy, rw, rh float32) {
	pad := rw * 0.16
	maxW, maxH := rw-2*pad, rh-2*pad
	k := float32(math.Min(float64(maxW/float32(t.Width)), float64(maxH/float32(t.Height))))
	dw, dh := float32(t.Width)*k, float32(t.Height)*k
	rl.DrawTexturePro(t,
		rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
		rl.Rectangle{X: cx - dw/2, Y: cy - dh/2, Width: dw, Height: dh},
		rl.Vector2{}, 0, u.fa(rl.White))
}

func (u *ui) paintTileLetter(tl tile, cx, cy, rh float32) {
	letter := tileLetter(tl)
	size := rh * 0.55
	m := u.measure(letter, size)
	u.onTile = true
	u.text(letter, cx-m.X/2, cy-m.Y/2, size, rl.ColorAlpha(rl.White, contrast.TileGlyphAlpha))
	u.onTile = false
}

func (u *ui) paintTileFace(tl tile, cx, cy, rw, rh float32) {
	if t := u.icons[tl.key]; t.ID != 0 {
		u.paintTileIcon(t, cx, cy, rw, rh)
		return
	}
	u.paintTileLetter(tl, cx, cy, rh)
}

func (u *ui) paintTileChrome(tl tile, rect rl.Rectangle, selected bool, s float32) {
	if selected {
		rl.DrawRectangleRoundedLinesEx(rect, 0.12, 12, 6*s, u.fa(u.accent))
	}
	if u.tileRunning(tl) {
		rl.DrawCircleV(rl.Vector2{X: rect.X + rect.Width - 28*s, Y: rect.Y + 28*s}, 10*s, u.fa(u.accent))
	}
}

func (u *ui) paintTileSubtitle(subtitle string, cx, ly, labelSize, maxLabel, s float32, selected bool) {
	if subtitle == "" {
		return
	}
	if !selected {
		return
	}
	ss := 26 * s
	fitted := u.fitText(subtitle, ss, maxLabel)
	sm := u.measure(fitted, ss)
	u.text(fitted, cx-sm.X/2, ly+labelSize+4*s, ss, rl.ColorAlpha(rl.RayWhite, 0.5))
}

func (u *ui) paintTileLabels(tl tile, cx, y, tw, th, s float32, selected bool) {
	labelSize := 40 * s
	maxLabel := tw + 40*s
	title := u.fitText(tl.title, labelSize, maxLabel)
	m := u.measure(title, labelSize)
	ly := y + th*1.05 + 20*s
	u.text(title, cx-m.X/2, ly, labelSize, tileLabelColor(selected))
	u.paintTileSubtitle(tl.subtitle, cx, ly, labelSize, maxLabel, s, selected)
}

func (u *ui) drawTile(i int, x, y, tw, th, s float32) {
	tl := u.tiles[i]
	sc := u.scales[tl.key]
	cx, cy := x+tw/2, y+th/2
	rw, rh := tw*sc, th*sc
	rect := rl.Rectangle{X: cx - rw/2, Y: cy - rh/2, Width: rw, Height: rh}
	u.paintTileCard(rect, i == u.selected, s)
	u.paintTileFace(tl, cx, cy, rw, rh)
	u.paintTileChrome(tl, rect, i == u.selected, s)
	u.paintTileLabels(tl, cx, y, tw, th, s, i == u.selected)
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
