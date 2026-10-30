// Package instance keeps BiscuitTin to one copy per user with a pid file.
// A second start signals the running copy to come to the foreground.
package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
)

// RaiseSignal asks the running instance to show its window.
const RaiseSignal = syscall.SIGUSR1

const fileName = "biscuittin.pid"

// ErrRunning is returned by Acquire when another instance holds the pid file.
var ErrRunning = errors.New("already running")

// Path is $XDG_RUNTIME_DIR/biscuittin/biscuittin.pid, or
// $TMPDIR/biscuittin-$UID/biscuittin.pid when XDG_RUNTIME_DIR is unset,
// beside the debug socket.
func Path() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("biscuittin-%d", os.Getuid()))
	} else {
		dir = filepath.Join(dir, "biscuittin")
	}
	return filepath.Join(dir, fileName)
}

// Lock is this process's pid file. Release deletes it.
type Lock struct {
	path string
}

// Acquire writes this process's pid to path. If a live BiscuitTin already
// holds it, Acquire returns that pid and ErrRunning. A pid file left by a
// crash, or naming a pid now used by another program, is replaced.
func Acquire(path string) (*Lock, int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, 0, err
	}
	for range 2 {
		err := create(path)
		if err == nil {
			return &Lock{path: path}, 0, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, 0, err
		}
		if pid, ok := holder(path); ok {
			return nil, pid, ErrRunning
		}
		lg().Info("removed stale pid file", zap.String("path", path))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("could not create %s", path)
}

// create makes the pid file only if it does not exist, so two instances
// starting together cannot both win.
func create(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(f, "%d\n", os.Getpid())
	cerr := f.Close()
	return errors.Join(werr, cerr)
}

// holder returns the pid in path if that process is alive and is a
// BiscuitTin, judged by its executable name matching ours.
func holder(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 || pid == os.Getpid() {
		return 0, false
	}
	if syscall.Kill(pid, 0) != nil {
		return 0, false
	}
	return pid, sameProgram(pid)
}

func sameProgram(pid int) bool {
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(comm)) == ownComm()
}

// ownComm is this process's name as the kernel reports it (at most 15
// bytes), for comparing against other processes.
func ownComm() string {
	if comm, err := os.ReadFile("/proc/self/comm"); err == nil {
		return strings.TrimSpace(string(comm))
	}
	return filepath.Base(os.Args[0])
}

// Raise signals pid to bring its window to the foreground.
func Raise(pid int) error {
	return syscall.Kill(pid, RaiseSignal)
}

// Release deletes the pid file if it is still ours.
func (l *Lock) Release() {
	if l == nil {
		return
	}
	raw, err := os.ReadFile(l.path)
	if err != nil || strings.TrimSpace(string(raw)) != strconv.Itoa(os.Getpid()) {
		return
	}
	if err := os.Remove(l.path); err != nil {
		lg().Warn("remove pid file", zap.String("path", l.path), zap.Error(err))
	}
}

func lg() *zap.Logger { return zap.L().Named("instance") }
