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
func columnsFit(configured, n int, gridW, gap, minTile float32) int {
	c := configured
	if c <= 0 {
		c = 5
	}
	if n <= 0 {
		return 0
	}
	if c > n {
		c = n
	}
	maxFit := 1
	if minTile+gap > 0 {
		maxFit = int(gridW / (minTile + gap))
		if maxFit < 1 {
			maxFit = 1
		}
	}
	if c > maxFit {
		return maxFit
	}
	return c
}

// wrapLines splits s onto at most maxLines of maxW, as measured by width.
// The last line is ellipsised when there are leftover words.
func wrapLines(s string, maxW float32, maxLines int, width func(string) float32) []string {
	if maxLines < 1 || maxW <= 0 {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, w := range words {
		if len(lines) == maxLines {
			break
		}
		trial := w
		if cur != "" {
			trial = cur + " " + w
		}
		if width(trial) <= maxW {
			cur = trial
			continue
		}
		if cur != "" {
			flush()
			if len(lines) == maxLines {
				break
			}
			if width(w) <= maxW {
				cur = w
				continue
			}
		}
		lines = append(lines, ellipsisLine(w, maxW, width))
		cur = ""
	}
	if cur != "" && len(lines) < maxLines {
		lines = append(lines, cur)
	}
	if moreWords(words, lines) && len(lines) > 0 {
		lines[len(lines)-1] = ellipsisLine(lines[len(lines)-1], maxW, width)
	}
	return lines
}

func moreWords(words, lines []string) bool {
	n := 0
	for _, line := range lines {
		n += len(strings.Fields(line))
	}
	return n < len(words)
}

func ellipsisLine(s string, maxW float32, width func(string) float32) string {
	if strings.HasSuffix(s, "...") && width(s) <= maxW {
		return s
	}
	if width(s+"...") <= maxW {
		return s + "..."
	}
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
