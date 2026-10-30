package cec

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		line string
		want Press
		ok   bool
	}{
		{"DEBUG:   [            9877]\tkey pressed: up (1) current(ff) duration(0)", Press{Name: "up", Key: KeyUp}, true},
		{"DEBUG:   [           10377]\tkey pressed: select (0) current(ff) duration(0)", Press{Name: "select", Key: KeySelect}, true},
		{"DEBUG:   [           11001]\tkey pressed: exit (d) current(ff) duration(0)", Press{Name: "exit", Key: KeyBack}, true},
		{"DEBUG:   [           11002]\tkey pressed: channel up (30) current(ff) duration(0)", Press{Name: "channel up", Key: KeyNone}, true},
		{"DEBUG:   [           11003]\tkey released: up (1) D:120ms", Press{}, false},
		{"TRAFFIC: [             440]\t>> 0f:87:00:15:82", Press{}, false},
	}
	for _, c := range cases {
		got, ok := ParseLine(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseLine(%q) = %+v, %v; want %+v, %v", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestReaderReadsPresses(t *testing.T) {
	r := Start(func() *exec.Cmd {
		return exec.CommandContext(context.Background(), "printf", `key pressed: left (3) current(ff) duration(0)\nnoise\n`)
	})
	defer r.Stop()
	select {
	case p := <-r.Presses:
		if p.Key != KeyLeft || p.Name != "left" {
			t.Fatalf("press = %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no press read")
	}
}

func TestReaderMissingCommand(t *testing.T) {
	r := Start(func() *exec.Cmd { return exec.CommandContext(context.Background(), "/nonexistent/cec-client") })
	defer r.Stop()
	deadline := time.Now().Add(5 * time.Second)
	for r.Status() == "Starting" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := r.Status(); len(got) < 11 || got[:11] != "Not running" {
		t.Fatalf("status = %q", got)
	}
	var nilReader *Reader
	if nilReader.Status() != "Off" {
		t.Fatal("nil reader should be Off")
	}
	nilReader.Stop()
}
