package backgrounds

import (
	"strings"
	"unicode"
)

// Terms that mean a photo is a person, a religious subject, or otherwise
// a poor fit for a living-room background. Matched as whole words in titles,
// descriptions and Commons categories.
var unsafeScene = []string{
	"portrait", "portraits", "self-portrait", "selfie",
	"people", "person", "crowd", "family", "nude", "nudes", "naked",
	"church", "churches", "cathedral", "cathedrals", "mosque", "mosques",
	"temple", "temples", "synagogue", "synagogues", "chapel", "chapels",
	"monastery", "monasteries", "shrine", "shrines", "madonna", "crucifix",
	"buddha", "prayer", "worship", "altar", "saint", "saints", "priest",
	"nun", "nuns", "religion", "religious", "communion", "baptism",
	"iconostasis", "icon", "icons",
}

// sceneSafe is true when none of the texts look like a person or religious
// subject. Commons category pulls already prefer landscapes and cities; this
// is a second pass on the file name and metadata.
func sceneSafe(texts ...string) bool {
	var b strings.Builder
	for _, t := range texts {
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t)
	}
	blob := strings.ToLower(b.String())
	if blob == "" {
		return true
	}
	for _, w := range unsafeScene {
		if containsWord(blob, w) {
			return false
		}
	}
	return true
}

func containsWord(blob, word string) bool {
	for i := 0; ; {
		j := strings.Index(blob[i:], word)
		if j < 0 {
			return false
		}
		j += i
		before := j == 0 || !isWordChar(rune(blob[j-1]))
		after := j+len(word) >= len(blob) || !isWordChar(rune(blob[j+len(word)]))
		if before && after {
			return true
		}
		i = j + 1
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '-'
}
