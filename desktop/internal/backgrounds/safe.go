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
func joinTexts(texts ...string) string {
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
	return strings.ToLower(b.String())
}

func hasUnsafeWord(blob string) bool {
	for _, w := range unsafeScene {
		if containsWord(blob, w) {
			return true
		}
	}
	return false
}

func sceneSafe(texts ...string) bool {
	blob := joinTexts(texts...)
	if blob == "" {
		return true
	}
	return !hasUnsafeWord(blob)
}

func wordBoundBefore(blob string, j int) bool {
	if j == 0 {
		return true
	}
	return !isWordChar(rune(blob[j-1]))
}

func wordBoundAfter(blob string, end int) bool {
	if end >= len(blob) {
		return true
	}
	return !isWordChar(rune(blob[end]))
}

func wordMatch(blob string, j, n int) bool {
	if !wordBoundBefore(blob, j) {
		return false
	}
	return wordBoundAfter(blob, j+n)
}

func containsWord(blob, word string) bool {
	for i := 0; ; {
		j := strings.Index(blob[i:], word)
		if j < 0 {
			return false
		}
		j += i
		if wordMatch(blob, j, len(word)) {
			return true
		}
		i = j + 1
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '-'
}
