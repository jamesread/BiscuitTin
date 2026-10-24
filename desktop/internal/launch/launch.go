// Package launch starts launcher entries as detached processes.
package launch

import (
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
	Index int
	PID   int
	Err   error
}

// Start starts an entry fully detached from biscuittin: its own session
// (no SIGHUP or terminal signals from us), no parent-death signal, stdin from
// /dev/null and output to a per-entry log file. If biscuittin dies the child is
// reparented and keeps running. A goroutine reaps it so no zombies are left
// while biscuittin is alive. Inside the Flatpak sandbox the command runs on
// the host instead (see command).
func Start(cfg *config.Config, index int, exits chan<- Exit) (int, error) {
	e := cfg.Entries[index]

	args := make([]string, len(e.Command))
	for i, a := range e.Command {
		args[i] = config.ExpandPath(a)
	}

	env := map[string]string{}
	for k, v := range e.Env {
		env[k] = os.ExpandEnv(v)
	}
	cmd := command(args, config.ExpandPath(e.Workdir), env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return 0, err
	}
	logPath := filepath.Join(cfg.LogDir, slug(e.Title)+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	// The child holds its own copy of the descriptor after Start.
	defer logFile.Close()

	fmt.Fprintf(logFile, "\n=== %s biscuittin launching %q\n", time.Now().Format(time.RFC3339), args)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

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
func command(args []string, dir string, env map[string]string) *exec.Cmd {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if _, err := os.Stat(flatpakInfo); err != nil {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		for _, k := range keys {
			cmd.Env = append(cmd.Env, k+"="+env[k])
		}
		return cmd
	}

	spawn := []string{"--host"}
	if dir != "" {
		spawn = append(spawn, "--directory="+dir)
	}
	for _, k := range keys {
		spawn = append(spawn, "--env="+k+"="+env[k])
	}
	return exec.Command("flatpak-spawn", append(spawn, args...)...)
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entry"
	}
	return out
}
