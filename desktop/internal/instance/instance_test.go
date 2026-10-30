package instance

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readPID(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func TestAcquireAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", fileName)
	l, _, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := readPID(t, path); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pid file = %q", got)
	}
	l.Release()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pid file left after Release: %v", err)
	}
}

func TestAcquireReplacesStale(t *testing.T) {
	for name, content := range map[string]string{
		"garbage": "not a pid\n",
		"dead":    "2147483646\n",
		"other":   "", // filled with a live non-BiscuitTin pid below
	} {
		t.Run(name, func(t *testing.T) {
			if name == "other" {
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Skip("no sleep:", err)
				}
				t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
				content = strconv.Itoa(cmd.Process.Pid)
			}
			path := filepath.Join(t.TempDir(), fileName)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			l, _, err := Acquire(path)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Release()
			if got := readPID(t, path); got != strconv.Itoa(os.Getpid()) {
				t.Fatalf("pid file = %q", got)
			}
		})
	}
}

// TestHelperProcess stands in for a running BiscuitTin: it is this test
// binary, so its process name matches.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("BISCUITTIN_INSTANCE_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
}

func TestAcquireRefusesLiveInstance(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "BISCUITTIN_INSTANCE_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(path, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, pid, err := Acquire(path)
	if !errors.Is(err, ErrRunning) || pid != cmd.Process.Pid {
		t.Fatalf("Acquire = pid %d, %v; want %d, ErrRunning", pid, err, cmd.Process.Pid)
	}
	if got := readPID(t, path); got != strconv.Itoa(cmd.Process.Pid) {
		t.Fatalf("pid file overwritten: %q", got)
	}
}

func TestReleaseKeepsOthersFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	(&Lock{path: path}).Release()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("Release removed a pid file it did not own")
	}
}
