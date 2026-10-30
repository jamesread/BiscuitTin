package app

import (
	"errors"
	"testing"
	"time"
)

type fakeResetter struct {
	n   int
	err error
}

func (f *fakeResetter) Reset() error {
	f.n++
	return f.err
}

func TestKeepAwakeThrottles(t *testing.T) {
	f := &fakeResetter{}
	k := &keepAwake{r: f, enabled: true}
	t0 := time.Unix(1000, 0)
	k.padUsed(t0)
	k.padUsed(t0.Add(time.Second))
	k.padUsed(t0.Add(keepAwakeEvery - time.Millisecond))
	if f.n != 1 {
		t.Fatalf("resets = %d within the interval, want 1", f.n)
	}
	k.padUsed(t0.Add(keepAwakeEvery))
	if f.n != 2 {
		t.Fatalf("resets = %d after the interval, want 2", f.n)
	}
}

func TestKeepAwakeDisabled(t *testing.T) {
	f := &fakeResetter{}
	(&keepAwake{r: f}).padUsed(time.Now())
	if f.n != 0 {
		t.Fatal("disabled keep-awake should not reset")
	}
	f.err = errors.New("no X")
	k := &keepAwake{r: f, enabled: true}
	k.padUsed(time.Now()) // logs, doesn't panic
	if f.n != 1 {
		t.Fatal("enabled keep-awake should try once")
	}
}
