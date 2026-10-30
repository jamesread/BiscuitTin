package settings

import "testing"

func TestStepColorChoice(t *testing.T) {
	last := ColorChoices[len(ColorChoices)-1].Hex
	cases := []struct {
		cur  string
		dir  int
		want string
	}{
		{"", 1, "#ffffff"},
		{"", -1, ""}, // clamps at Accent
		{last, 1, last},
		{last, 0, ""}, // A cycles round
		{"#FFFFFF", -1, ""},
		{"#123456", 1, ""}, // hand-set colours step from Accent
	}
	for _, c := range cases {
		if got := StepColorChoice(c.cur, c.dir); got != c.want {
			t.Errorf("StepColorChoice(%q, %d) = %q; want %q", c.cur, c.dir, got, c.want)
		}
	}
}

func TestColorChoiceLabel(t *testing.T) {
	labels := map[string]string{"": "Accent", "#4fa3ff": "Blue", "#123456": "Custom #123456"}
	for in, want := range labels {
		if got := ColorChoiceLabel(in); got != want {
			t.Errorf("ColorChoiceLabel(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestThemes(t *testing.T) {
	s := &Settings{}
	if s.ThemeLabel() != "Accent" {
		t.Fatalf("default theme = %q", s.ThemeLabel())
	}
	s.StepTheme(1)
	if s.ThemeLabel() != "Mono" || s.FocusBorder != "#ffffff" || s.FocusFill != "#ffffff" || s.AccentText != "#ffffff" {
		t.Fatalf("Mono = %+v", s)
	}
	s.FocusFill = "#ff5c5c"
	if s.ThemeLabel() != "Custom" {
		t.Fatalf("mixed colours = %q", s.ThemeLabel())
	}
	s.StepTheme(-1) // Custom steps from Accent
	if s.ThemeLabel() != "Accent" {
		t.Fatalf("after Custom = %q", s.ThemeLabel())
	}
	for range Themes {
		s.StepTheme(0)
	}
	if s.ThemeLabel() != "Accent" {
		t.Fatalf("A should cycle round, got %q", s.ThemeLabel())
	}
}

func TestInterfaceSize(t *testing.T) {
	s := &Settings{}
	if s.InterfaceScale() != 1 || s.InterfaceSizeLabel() != "Normal" {
		t.Fatalf("unset = %v %q", s.InterfaceScale(), s.InterfaceSizeLabel())
	}
	s.StepInterfaceSize(1)
	s.StepInterfaceSize(1)
	s.StepInterfaceSize(1) // clamps
	if s.InterfaceSize != 130 || s.InterfaceSizeLabel() != "Extra large" {
		t.Fatalf("largest = %d %q", s.InterfaceSize, s.InterfaceSizeLabel())
	}
	s.StepInterfaceSize(0)
	if s.InterfaceSize != 85 {
		t.Fatalf("A should cycle to Small, got %d", s.InterfaceSize)
	}
}

func TestStepScreenOff(t *testing.T) {
	cases := []struct{ cur, dir, want int }{
		{0, 1, 300},
		{0, -1, 0},
		{14400, 1, 14400},
		{14400, 0, 0},
		{700, 1, 900}, // hand-set values step from the one below
	}
	for _, c := range cases {
		if got := StepScreenOff(c.cur, c.dir); got != c.want {
			t.Errorf("StepScreenOff(%d, %d) = %d; want %d", c.cur, c.dir, got, c.want)
		}
	}
}
