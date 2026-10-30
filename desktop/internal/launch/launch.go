// Package launch starts launcher entries as detached processes.
package launch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
)

// Exit reports that a launched entry's process has exited.
type Exit struct {
	Err   error
	Index int
	PID   int
}

// Start starts an entry fully detached from BiscuitTin: its own session
// (no SIGHUP or terminal signals from us), no parent-death signal, stdin from
// /dev/null and output to a per-entry log file. If BiscuitTin dies the child is
// reparented and keeps running. A goroutine reaps it so no zombies are left
// while BiscuitTin is alive. Inside the Flatpak sandbox the command runs on
// the host instead (see command).
func expandArgs(cmd []string) []string {
	args := make([]string, len(cmd))
	for i, a := range cmd {
		args[i] = config.ExpandPath(a)
	}
	return args
}

func expandEnv(in map[string]string) map[string]string {
	env := map[string]string{}
	for k, v := range in {
		env[k] = os.ExpandEnv(v)
	}
	return env
}

func startCmd(cfg *config.Config, index int) (*exec.Cmd, *os.File, error) {
	e := cfg.Entries[index]
	args := expandArgs(e.Command)
	cmd := command(args, config.ExpandPath(e.Workdir), expandEnv(e.Env))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := os.MkdirAll(cfg.LogDir, 0o750); err != nil {
		return nil, nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(cfg.LogDir, slug(e.Title)+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	_, _ = fmt.Fprintf(logFile, "\n=== %s biscuittin launching %q\n", time.Now().Format(time.RFC3339), args)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	return cmd, logFile, nil
}

func waitLaunch(cmd *exec.Cmd, index int, exits chan<- Exit) (int, error) {
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go func() {
		err := cmd.Wait()
		exits <- Exit{Index: index, PID: pid, Err: err}
	}()
	return pid, nil
}

func Start(cfg *config.Config, index int, exits chan<- Exit) (int, error) {
	cmd, logFile, err := startCmd(cfg, index)
	if err != nil {
		return 0, err
	}
	// The child holds its own copy of the descriptor after Start.
	defer func() { _ = logFile.Close() }()
	return waitLaunch(cmd, index, exits)
}

// Alive is true when pid is still a live process.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// Kill sends SIGTERM to the process group started by Start (Setsid), or to
// the process itself if it is not a group leader.
func Kill(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// ForceKill is Kill with SIGKILL, for a process that ignored SIGTERM.
func ForceKill(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// Host builds a command that runs on the host. Inside the Flatpak sandbox
// that is flatpak-spawn --host, the same as launched entries.
func Host(args []string) *exec.Cmd {
	return command(args, "", nil)
}

// flatpakInfo exists only inside a Flatpak sandbox.
var flatpakInfo = "/.flatpak-info"

// command builds the process for an entry. Inside the Flatpak sandbox the
// host's programs aren't visible, so the command is run on the host with
// flatpak-spawn, which forwards output, the exit status and signals. The
// host command gets the host session's environment plus the entry's env, not
// the sandbox's environment.
func envKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hostCommand(args []string, dir string, env map[string]string, keys []string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, k := range keys {
		cmd.Env = append(cmd.Env, k+"="+env[k])
	}
	return cmd
}

func spawnCommand(args []string, dir string, env map[string]string, keys []string) *exec.Cmd {
	spawn := []string{"--host"}
	if dir != "" {
		spawn = append(spawn, "--directory="+dir)
	}
	for _, k := range keys {
		spawn = append(spawn, "--env="+k+"="+env[k])
	}
	return exec.CommandContext(context.Background(), "flatpak-spawn", append(spawn, args...)...)
}

func command(args []string, dir string, env map[string]string) *exec.Cmd {
	keys := envKeys(env)
	if _, err := os.Stat(flatpakInfo); err != nil {
		return hostCommand(args, dir, env, keys)
	}
	return spawnCommand(args, dir, env, keys)
}

func isSlugChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	return r >= '0' && r <= '9'
}

func slugRune(r rune) rune {
	if isSlugChar(r) {
		return r
	}
	return '-'
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(slugRune(r))
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entry"
	}
	return out
}
