package backgrounds

import "testing"

func TestSceneSafe(t *testing.T) {
	ok := []string{
		"Sunset over the Yorkshire Dales",
		"Featured pictures of landscapes|Coasts of Scotland",
		"Tokyo skyline at dusk",
		"Isle of Man from the sea", // "man" is not a whole-word people term
		"Beijing, People's Republic of China",
	}
	for _, s := range ok {
		if !sceneSafe(s) {
			t.Errorf("sceneSafe(%q) = false; want true", s)
		}
	}
	bad := []string{
		"Portrait of a woman in a red dress",
		"Featured pictures of churches|Cathedrals in France",
		"The Madonna and Child",
		"Friday prayer at the mosque",
		"Self-portrait, 1889",
		"iconic", // contains "icon" as prefix — should be safe
	}
	for _, s := range bad[:5] {
		if sceneSafe(s) {
			t.Errorf("sceneSafe(%q) = true; want false", s)
		}
	}
	if !sceneSafe(bad[5]) {
		t.Error(`"iconic" should not match the word "icon"`)
	}
}
