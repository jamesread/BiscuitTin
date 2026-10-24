package app

import (
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/rss"
)

const rssSlideSeconds = 0.45

func (u *ui) hasRSS() bool {
	return u.rss != nil && len(u.rss.Items()) > 0
}

func (u *ui) drawRSS(w, h, s, margin float32) {
	alpha := 1 - u.bodyAlpha
	if alpha <= 0.01 {
		u.rssShownAt = 0
		return
	}
	if !u.hasRSS() {
		return
	}
	u.tickRSS()
	u.drawRSSCards(w, h, s, margin, alpha)
}

func (u *ui) tickRSS() {
	if u.rssSeen == nil {
		u.rssSeen = map[string]bool{}
	}
	now := rl.GetTime()
	if !u.rssHaveCur {
		u.tryShowRSS(false, now)
	} else if u.rssReadyToAdvance(now) {
		u.tryShowRSS(true, now)
	}
	u.advanceRSSAnim()
}

func rssHoldSeconds(u *ui) float64 {
	rot := float64(u.settings.RSSInterval)
	if rot <= 0 {
		rot = float64(u.cfg.RSS.RotateSeconds)
	}
	if rot < 60 {
		rot = 60
	}
	return rot
}

func (u *ui) rssReadyToAdvance(now float64) bool {
	if u.rssAnim < 1 {
		return false
	}
	if u.rssShownAt == 0 {
		return false
	}
	return now-u.rssShownAt >= rssHoldSeconds(u)
}

func (u *ui) tryShowRSS(slide bool, now float64) {
	next, ok := pickUnseenRSS(u.rss.Items(), u.rssSeen)
	if ok {
		u.showRSS(next, slide, now)
	}
}

func (u *ui) advanceRSSAnim() {
	if u.rssAnim >= 1 {
		return
	}
	u.rssAnim += rl.GetFrameTime() / rssSlideSeconds
	if u.rssAnim > 1 {
		u.rssAnim = 1
		u.rssHavePrev = false
	}
}

func (u *ui) drawRSSCards(w, h, s, margin, alpha float32) {
	u.drawAlpha = alpha
	defer func() { u.drawAlpha = u.bodyAlpha }()
	cardW := float32(math.Min(float64(w-2*margin), float64(1200*s)))
	slide := (cardW + 80*s) * (1 - easeInOut(u.rssAnim))
	if u.rssHavePrev {
		u.drawRSSCard(u.rssPrev, margin-easeInOut(u.rssAnim)*(cardW+80*s), h, s, cardW)
	}
	if u.rssHaveCur {
		u.drawRSSCard(u.rssCur, margin+slide, h, s, cardW)
	}
}

func (u *ui) showRSS(item rss.Item, slideFromPrev bool, now float64) {
	if slideFromPrev && u.rssHaveCur {
		u.rssPrev = u.rssCur
		u.rssHavePrev = true
		u.rssAnim = 0
	} else {
		u.rssHavePrev = false
		u.rssAnim = 0
	}
	u.rssCur = item
	u.rssHaveCur = true
	u.rssShownAt = now
	u.rssSeen[rssItemID(item)] = true
}

func addRSSBlock(h float32, n int, lineH, lineGap, gap float32) float32 {
	if n <= 0 {
		return h
	}
	if h > 0 {
		h += gap
	}
	return h + float32(n)*lineH + float32(n-1)*lineGap
}

func rssBlockHeight(source string, titleN, sumN int, sourceSize, titleSize, sumSize, gap, s float32) float32 {
	h := float32(0)
	if source != "" {
		h = sourceSize
	}
	h = addRSSBlock(h, titleN, titleSize, 8*s, gap)
	return addRSSBlock(h, sumN, sumSize, 6*s, gap)
}

func (u *ui) drawRSSCard(item rss.Item, x, h, s, cardW float32) {
	sourceSize, titleSize, sumSize, gap, pad := 28*s, 52*s, 32*s, 14*s, 28*s
	source := rssSourceLine(item, time.Now())
	titleLines := wrapLines(item.Title, cardW, 2, func(t string) float32 { return u.measure(t, titleSize).X })
	sumLines := wrapLines(item.Summary, cardW, 4, func(t string) float32 { return u.measure(t, sumSize).X })
	blockH := rssBlockHeight(source, len(titleLines), len(sumLines), sourceSize, titleSize, sumSize, gap, s)
	if blockH == 0 {
		return
	}
	u.paintRSSCard(source, titleLines, sumLines, x, h, s, cardW, blockH, sourceSize, titleSize, sumSize, gap, pad)
}

func (u *ui) paintRSSCard(source string, titleLines, sumLines []string, x, h, s, cardW, blockH, sourceSize, titleSize, sumSize, gap, pad float32) {
	// Sit above the credit line, left-aligned.
	y := h - 64*s - blockH
	if y < 210*s {
		y = 210 * s
	}
	u.drawRSSPanel(x, y, cardW, blockH, pad)
	u.onTile = true
	u.paintRSSText(source, titleLines, sumLines, x, y, sourceSize, titleSize, sumSize, gap, s)
	u.onTile = false
}

func (u *ui) drawRSSPanel(x, y, cardW, blockH, pad float32) {
	panel := rl.Rectangle{X: x - pad, Y: y - pad, Width: cardW + 2*pad, Height: blockH + 2*pad}
	backdrop := u.backdropFor(panel.X, panel.Y, panel.X+panel.Width, panel.Y+panel.Height)
	backing, _ := contrast.SolveBacking(contrast.RGBOf(rl.RayWhite), 1, backdrop)
	if backing < 0.55 {
		backing = 0.55
	}
	rl.DrawRectangleRounded(panel, 0.08, 12, rl.ColorAlpha(rl.Black, float32(backing)*u.drawAlpha))
}

func rssNeedsGap(source string, titleLines []string) bool {
	if source != "" {
		return true
	}
	return len(titleLines) > 0
}

func (u *ui) paintRSSText(source string, titleLines, sumLines []string, x, y, sourceSize, titleSize, sumSize, gap, s float32) {
	if source != "" {
		u.text(source, x, y, sourceSize, rl.ColorAlpha(u.accent, 0.95))
		y += sourceSize + gap
	}
	y = u.paintRSSLines(titleLines, x, y, titleSize, 8*s, rl.RayWhite)
	if len(sumLines) > 0 && rssNeedsGap(source, titleLines) {
		y += gap
	}
	u.paintRSSLines(sumLines, x, y, sumSize, 6*s, rl.ColorAlpha(rl.RayWhite, 0.88))
}

func (u *ui) paintRSSLines(lines []string, x, y, size, gap float32, color rl.Color) float32 {
	for i, line := range lines {
		if i > 0 {
			y += gap
		}
		u.text(line, x, y, size, color)
		y += size
	}
	return y
}

func rssItemID(it rss.Item) string {
	if it.Link != "" {
		return it.Link
	}
	return it.Feed + "\n" + it.Title
}

func pickUnseenRSS(items []rss.Item, seen map[string]bool) (rss.Item, bool) {
	for _, it := range items {
		if !seen[rssItemID(it)] {
			return it, true
		}
	}
	return rss.Item{}, false
}

func easeInOut(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func rssSourceLine(item rss.Item, now time.Time) string {
	age := rss.AgeLabel(item.Published, now)
	switch {
	case item.Feed != "" && age != "":
		return item.Feed + "  ·  " + age
	case item.Feed != "":
		return item.Feed
	default:
		return age
	}
}
