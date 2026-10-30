package settings

import (
	"slices"
	"strings"
)

// ColorChoice is a named colour offered for the focus border, focus
// background and accent text. An empty Hex follows the config's accent.
type ColorChoice struct {
	Name, Hex string
}

// ColorChoices is the Settings order for each of those colours.
var ColorChoices = []ColorChoice{
	{"Accent", ""},
	{"White", "#ffffff"},
	{"Orange", "#ff8c32"},
	{"Red", "#ff5c5c"},
	{"Pink", "#ff6fb5"},
	{"Purple", "#b07cff"},
	{"Blue", "#4fa3ff"},
	{"Teal", "#2ec4b6"},
	{"Green", "#5fd068"},
}

// colorChoiceIndex is hex's place in ColorChoices, or -1 for a colour set by
// hand in the settings file.
func colorChoiceIndex(hex string) int {
	for i, c := range ColorChoices {
		if strings.EqualFold(c.Hex, strings.TrimSpace(hex)) {
			return i
		}
	}
	return -1
}

// StepColorChoice moves to the previous (dir < 0) or next (dir > 0) colour,
// clamped, or cycles round for dir 0. Hand-set colours step from Accent.
func StepColorChoice(hex string, dir int) string {
	i := colorChoiceIndex(hex)
	if dir == 0 {
		return ColorChoices[(i+1)%len(ColorChoices)].Hex
	}
	i = min(max(i+dir, 0), len(ColorChoices)-1)
	return ColorChoices[i].Hex
}

// ColorChoiceLabel is the name shown in Settings.
func ColorChoiceLabel(hex string) string {
	if i := colorChoiceIndex(hex); i >= 0 {
		return ColorChoices[i].Name
	}
	return "Custom " + strings.TrimSpace(hex)
}

// Theme sets the focus border, focus background and accent text together.
type Theme struct {
	Name, Border, Fill, Text string
}

// Themes is the Settings order. Accent follows the config's accent for all
// three; any other mix of colours shows as Custom.
var Themes = []Theme{
	{"Accent", "", "", ""},
	{"Mono", "#ffffff", "#ffffff", "#ffffff"},
	{"Ocean", "#4fa3ff", "#2ec4b6", "#4fa3ff"},
	{"Sunset", "#ff8c32", "#ff6fb5", "#ff8c32"},
	{"Forest", "#5fd068", "#2ec4b6", "#5fd068"},
	{"Royal", "#b07cff", "#b07cff", "#ff6fb5"},
}

// themeIndex is the theme matching s's colours, or -1 for Custom.
func (s *Settings) themeIndex() int {
	for i, t := range Themes {
		if strings.EqualFold(t.Border, s.FocusBorder) && strings.EqualFold(t.Fill, s.FocusFill) && strings.EqualFold(t.Text, s.AccentText) {
			return i
		}
	}
	return -1
}

// ThemeLabel is the name shown in Settings.
func (s *Settings) ThemeLabel() string {
	if i := s.themeIndex(); i >= 0 {
		return Themes[i].Name
	}
	return "Custom"
}

// StepTheme applies the previous (dir < 0) or next (dir > 0) theme, or
// cycles round for dir 0. Custom colours step from Accent.
func (s *Settings) StepTheme(dir int) {
	i := s.themeIndex()
	if dir == 0 {
		i = (i + 1) % len(Themes)
	} else {
		i = min(max(i+dir, 0), len(Themes)-1)
	}
	t := Themes[i]
	s.FocusBorder, s.FocusFill, s.AccentText = t.Border, t.Fill, t.Text
}

// InterfaceSizes are the Interface size choices, as percentages of the
// normal size.
var InterfaceSizes = []int{85, 100, 115, 130}

// InterfaceSizeLabels go with InterfaceSizes.
var InterfaceSizeLabels = []string{"Small", "Normal", "Large", "Extra large"}

// interfaceSizeIndex treats unset or hand-set sizes as Normal.
func (s *Settings) interfaceSizeIndex() int {
	if i := slices.Index(InterfaceSizes, s.InterfaceSize); i >= 0 {
		return i
	}
	return 1
}

// InterfaceScale is the factor applied on top of the window's own scale.
func (s *Settings) InterfaceScale() float32 {
	return float32(InterfaceSizes[s.interfaceSizeIndex()]) / 100
}

func (s *Settings) InterfaceSizeLabel() string {
	return InterfaceSizeLabels[s.interfaceSizeIndex()]
}

// StepInterfaceSize clamps for dir -1/+1 and cycles round for dir 0.
func (s *Settings) StepInterfaceSize(dir int) {
	i := s.interfaceSizeIndex()
	if dir == 0 {
		i = (i + 1) % len(InterfaceSizes)
	} else {
		i = min(max(i+dir, 0), len(InterfaceSizes)-1)
	}
	s.InterfaceSize = InterfaceSizes[i]
}
