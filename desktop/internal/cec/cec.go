// Package cec reads TV remote buttons over HDMI-CEC by running libCEC's
// cec-client and reading the key presses it logs. It is off unless
// cec.enabled is set. Kernel CEC drivers present the remote as a keyboard
// instead, which needs none of this.
package cec

import (
	"bufio"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Key is a remote button BiscuitTin uses.
type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeySelect
	KeyBack
)

// Press is one remote button press: the key, and libCEC's name for the
// button, e.g. "select", for Input devices diagnostics.
type Press struct {
	Name string
	Key  Key
}

// keyCodes maps CEC user control codes (HDMI-CEC 1.4, table 30) to keys.
var keyCodes = map[int]Key{
	0x00: KeySelect, // Select
	0x01: KeyUp,
	0x02: KeyDown,
	0x03: KeyLeft,
	0x04: KeyRight,
	0x09: KeyBack,   // Root menu
	0x0d: KeyBack,   // Exit
	0x2b: KeySelect, // Enter
	0x44: KeySelect, // Play
	0x91: KeyBack,   // Back, on some newer remotes (an F-key range code)
}

// keyPressed matches cec-client's debug log line for a press, e.g.
// "DEBUG:   [ 9877]  key pressed: up (1) current(ff) duration(0)".
var keyPressed = regexp.MustCompile(`key pressed: (.+?) \(([0-9a-fA-F]+)\)`)

// ParseLine returns the press in one line of cec-client output, if any.
// Buttons BiscuitTin has no use for come back with KeyNone.
func ParseLine(line string) (Press, bool) {
	m := keyPressed.FindStringSubmatch(line)
	if m == nil {
		return Press{}, false
	}
	code, err := strconv.ParseInt(m[2], 16, 32)
	if err != nil {
		return Press{}, false
	}
	return Press{Name: m[1], Key: keyCodes[int(code)]}, true
}

// retryAfter is how long to wait before starting cec-client again after it
// exits, e.g. while the adapter is unplugged.
var retryAfter = 30 * time.Second

// Reader runs cec-client until stopped, restarting it when it exits.
type Reader struct {
	Presses chan Press
	command func() *exec.Cmd
	stop    chan struct{}
	status  string
	mu      sync.Mutex
}

// Start runs command (built fresh for each attempt) in the background.
func Start(command func() *exec.Cmd) *Reader {
	r := &Reader{
		Presses: make(chan Press, 16),
		command: command,
		stop:    make(chan struct{}),
		status:  "Starting",
	}
	go r.run()
	return r
}

// Stop ends cec-client. A nil Reader is fine.
func (r *Reader) Stop() {
	if r == nil {
		return
	}
	close(r.stop)
}

// Status is shown on App diagnostics. A nil Reader is off.
func (r *Reader) Status() string {
	if r == nil {
		return "Off"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Reader) setStatus(s string) {
	r.mu.Lock()
	r.status = s
	r.mu.Unlock()
}

func (r *Reader) run() {
	for {
		r.runOnce()
		select {
		case <-r.stop:
			return
		case <-time.After(retryAfter):
		}
	}
}

func (r *Reader) runOnce() {
	cmd := r.command()
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		r.setStatus("Not running: " + err.Error())
		lg().Warn("cec-client could not start", zap.Strings("argv", cmd.Args), zap.Error(err))
		return
	}
	r.setStatus("cec-client running")
	lg().Info("cec-client started", zap.Strings("argv", cmd.Args))
	done := make(chan struct{})
	go func() {
		select {
		case <-r.stop:
			_ = cmd.Process.Kill()
		case <-done:
		}
	}()
	r.read(out)
	err = cmd.Wait()
	close(done)
	r.setStatus("Exited, retrying")
	lg().Warn("cec-client exited", zap.Error(err))
}

func (r *Reader) read(out io.Reader) {
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		p, ok := ParseLine(sc.Text())
		if !ok {
			continue
		}
		select {
		case r.Presses <- p:
		default: // the UI is behind; drop rather than block cec-client
		}
	}
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("cec") }
