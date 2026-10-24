package app

import "strings"

// The UI is designed for a 1920x1080 landscape frame. layoutScale is the
// uniform factor that fits that frame in the current window, so portrait
// displays and resized windows keep text and chrome inside the viewport.
const (
	refWidth  = 1920
	refHeight = 1080
)

func layoutScale(w, h float32) float32 {
	if w <= 0 || h <= 0 {
		return 1
	}
	sw, sh := w/refWidth, h/refHeight
	if sw < sh {
		return sw
	}
	return sh
}

// columnsFit is the configured column count, capped so each tile stays at
// least minTile wide in the grid.
func defaultColumns(configured, n int) int {
	c := configured
	if c <= 0 {
		c = 5
	}
	if n <= 0 {
		return 0
	}
	if c > n {
		return n
	}
	return c
}

func maxColumns(gridW, gap, minTile float32) int {
	if minTile+gap <= 0 {
		return 1
	}
	return max(int(gridW/(minTile+gap)), 1)
}

func columnsFit(configured, n int, gridW, gap, minTile float32) int {
	c := defaultColumns(configured, n)
	if c == 0 {
		return 0
	}
	maxFit := maxColumns(gridW, gap, minTile)
	if c > maxFit {
		return maxFit
	}
	return c
}

// wrapLines splits s onto at most maxLines of maxW, as measured by width.
// The last line is ellipsised when there are leftover words.
func wrapLines(s string, maxW float32, maxLines int, width func(string) float32) []string {
	if maxLines < 1 {
		return nil
	}
	if maxW <= 0 {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	return finishWrap(placeWords(words, maxW, maxLines, width), words, maxW, width)
}

func joinWord(cur, w string) string {
	if cur == "" {
		return w
	}
	return cur + " " + w
}

func placeWords(words []string, maxW float32, maxLines int, width func(string) float32) []string {
	var lines []string
	cur := ""
	for _, w := range words {
		if len(lines) == maxLines {
			break
		}
		lines, cur = placeWord(lines, cur, w, maxW, maxLines, width)
	}
	return flushCur(lines, cur, maxLines)
}

func placeWord(lines []string, cur, w string, maxW float32, maxLines int, width func(string) float32) ([]string, string) {
	trial := joinWord(cur, w)
	if width(trial) <= maxW {
		return lines, trial
	}
	return breakWord(lines, cur, w, maxW, maxLines, width)
}

func breakWord(lines []string, cur, w string, maxW float32, maxLines int, width func(string) float32) ([]string, string) {
	if cur == "" {
		return append(lines, ellipsisLine(w, maxW, width)), ""
	}
	lines = append(lines, cur)
	if len(lines) == maxLines {
		return lines, ""
	}
	if width(w) <= maxW {
		return lines, w
	}
	return append(lines, ellipsisLine(w, maxW, width)), ""
}

func flushCur(lines []string, cur string, maxLines int) []string {
	if cur == "" {
		return lines
	}
	if len(lines) >= maxLines {
		return lines
	}
	return append(lines, cur)
}

func finishWrap(lines, words []string, maxW float32, width func(string) float32) []string {
	if !moreWords(words, lines) {
		return lines
	}
	if len(lines) == 0 {
		return lines
	}
	lines[len(lines)-1] = ellipsisLine(lines[len(lines)-1], maxW, width)
	return lines
}

func moreWords(words, lines []string) bool {
	n := 0
	for _, line := range lines {
		n += len(strings.Fields(line))
	}
	return n < len(words)
}

func alreadyEllipsis(s string, maxW float32, width func(string) float32) bool {
	if !strings.HasSuffix(s, "...") {
		return false
	}
	return width(s) <= maxW
}

func shrinkEllipsis(s string, maxW float32, width func(string) float32) string {
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		t := strings.TrimRight(string(r), " ") + "..."
		if width(t) <= maxW {
			return t
		}
	}
	return "..."
}

func ellipsisLine(s string, maxW float32, width func(string) float32) string {
	if alreadyEllipsis(s, maxW, width) {
		return s
	}
	if width(s+"...") <= maxW {
		return s + "..."
	}
	return shrinkEllipsis(s, maxW, width)
}
