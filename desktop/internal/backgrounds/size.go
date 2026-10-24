package backgrounds

// Size is a requested image size in pixels.
type Size struct {
	W, H int
}

var (
	standardLandscape = []Size{{1920, 1080}, {2560, 1440}, {3840, 2160}}
	standardPortrait  = []Size{{1080, 1920}, {1440, 2560}, {2160, 3840}}
	standardSquare    = []Size{{1080, 1080}, {1920, 1920}}
)

func (s Size) ok() bool { return s.W > 0 && s.H > 0 }

func normalizeSize(s Size) Size {
	if s.ok() {
		return s
	}
	return Size{W: bgWidth, H: bgHeight}
}

// requestSizes is the viewport size, then standard sizes of the same
// orientation, used when the exact size is missing or unreadable.
func requestSizes(want Size) []Size {
	want = normalizeSize(want)
	out := []Size{want}
	var standards []Size
	switch {
	case want.H > want.W:
		standards = standardPortrait
	case want.W > want.H:
		standards = standardLandscape
	default:
		standards = standardSquare
	}
	for _, s := range standards {
		if s != want {
			out = append(out, s)
		}
	}
	return out
}
