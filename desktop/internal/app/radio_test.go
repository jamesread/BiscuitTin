package app

import "testing"

func TestRadioGroup(t *testing.T) {
	v := true
	rg := boolRadio("Shown", "Hidden", func() bool { return v }, func(b bool) { v = b })
	rg.step(-1) // already at the first option: stays
	if !v {
		t.Fatal("left at first option should not change the value")
	}
	rg.step(1)
	if v {
		t.Fatal("right should select Hidden")
	}
	rg.step(1) // clamps at the last option
	rg.cycle() // A wraps round
	if !v {
		t.Fatal("cycle from Hidden should wrap to Shown")
	}
}
