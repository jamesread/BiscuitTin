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
	items := u.rss.Items()
	if len(items) == 0 {
		return
	}
	if u.rssSeen == nil {
		u.rssSeen = map[string]bool{}
	}

	now := rl.GetTime()
	rot := float64(u.settings.RSSInterval)
	if rot <= 0 {
		rot = float64(u.cfg.RSS.RotateSeconds)
	}
	if rot < 60 {
		rot = 60
	}

	if !u.rssHaveCur {
		if next, ok := pickUnseenRSS(items, u.rssSeen); ok {
			u.showRSS(next, false, now)
		} else {
			return
		}
	} else if u.rssAnim >= 1 && u.rssShownAt > 0 && now-u.rssShownAt >= rot {
		if next, ok := pickUnseenRSS(items, u.rssSeen); ok {
			u.showRSS(next, true, now)
		}
	}

	if u.rssAnim < 1 {
		u.rssAnim += rl.GetFrameTime() / rssSlideSeconds
		if u.rssAnim > 1 {
			u.rssAnim = 1
			u.rssHavePrev = false
		}
	}

	u.drawAlpha = alpha
	defer func() { u.drawAlpha = u.bodyAlpha }()

	cardW := float32(math.Min(float64(w-2*margin), float64(1200*s)))
	restX := margin
	slide := (cardW + 80*s) * (1 - easeInOut(u.rssAnim))
	if u.rssHavePrev {
		u.drawRSSCard(u.rssPrev, restX-easeInOut(u.rssAnim)*(cardW+80*s), h, s, cardW)
	}
	if u.rssHaveCur {
		u.drawRSSCard(u.rssCur, restX+slide, h, s, cardW)
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

func (u *ui) drawRSSCard(item rss.Item, x, h, s, cardW float32) {
	sourceSize := 28 * s
	titleSize := 52 * s
	sumSize := 32 * s
	gap := 14 * s
	pad := 28 * s

	source := rssSourceLine(item, time.Now())
	titleLines := wrapLines(item.Title, cardW, 2, func(t string) float32 { return u.measure(t, titleSize).X })
	sumLines := wrapLines(item.Summary, cardW, 4, func(t string) float32 { return u.measure(t, sumSize).X })

	blockH := float32(0)
	if source != "" {
		blockH += sourceSize
	}
	if len(titleLines) > 0 {
		if blockH > 0 {
			blockH += gap
		}
		blockH += float32(len(titleLines))*titleSize + float32(len(titleLines)-1)*8*s
	}
	if len(sumLines) > 0 {
		if blockH > 0 {
			blockH += gap
		}
		blockH += float32(len(sumLines))*sumSize + float32(len(sumLines)-1)*6*s
	}
	if blockH == 0 {
		return
	}

	// Sit above the credit line, left-aligned.
	y := h - 64*s - blockH
	if y < 210*s {
		y = 210 * s
	}

	panel := rl.Rectangle{
		X:      x - pad,
		Y:      y - pad,
		Width:  cardW + 2*pad,
		Height: blockH + 2*pad,
	}
	backdrop := u.backdropFor(panel.X, panel.Y, panel.X+panel.Width, panel.Y+panel.Height)
	backing, _ := contrast.SolveBacking(contrast.RGBOf(rl.RayWhite), 1, backdrop)
	if backing < 0.55 {
		backing = 0.55
	}
	rl.DrawRectangleRounded(panel, 0.08, 12, rl.ColorAlpha(rl.Black, float32(backing)*u.drawAlpha))

	u.onTile = true
	ty := y
	if source != "" {
		u.text(source, x, ty, sourceSize, rl.ColorAlpha(u.accent, 0.95))
		ty += sourceSize + gap
	}
	for i, line := range titleLines {
		if i > 0 {
			ty += 8 * s
		}
		u.text(line, x, ty, titleSize, rl.RayWhite)
		ty += titleSize
	}
	if len(sumLines) > 0 && (source != "" || len(titleLines) > 0) {
		ty += gap
	}
	for i, line := range sumLines {
		if i > 0 {
			ty += 6 * s
		}
		u.text(line, x, ty, sumSize, rl.ColorAlpha(rl.RayWhite, 0.88))
		ty += sumSize
	}
	u.onTile = false
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
