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

func TestChoiceRadio(t *testing.T) {
	i := 0
	rg := choiceRadio([]string{"None", "Main menu", "Idle screen"}, func() int { return i }, func(n int) { i = n })
	rg.step(1)
	if i != 1 {
		t.Fatalf("right = %d", i)
	}
	rg.cycle()
	if i != 2 {
		t.Fatalf("cycle = %d", i)
	}
	rg.cycle()
	if i != 0 {
		t.Fatalf("wrap = %d", i)
	}
}
