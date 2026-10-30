package launch

import (
	"errors"
	"os/exec"
	"strings"
)

// ProbeArgs is the host command that exits 0 when an entry's command is
// installed: flatpak info for `flatpak run <id>`, otherwise the shell's
// command -v, which finds names on the host's PATH and checks paths are
// executable. nil means the command can't be checked.
func ProbeArgs(cmd []string) []string {
	if len(cmd) == 0 {
		return nil
	}
	if cmd[0] == "flatpak" && len(cmd) > 1 && cmd[1] == "run" {
		for _, a := range cmd[2:] {
			if !strings.HasPrefix(a, "-") {
				return []string{"flatpak", "info", a}
			}
		}
		return nil
	}
	return []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", cmd[0]}
}

// runProbe runs a probe command. Tests replace this.
var runProbe = func(argv []string) error {
	return Host(argv).Run()
}

// Installed reports whether cmd's program is installed on the host. known
// is false when that couldn't be checked (no probe, or the probe itself
// failed to run), so callers don't warn on a guess.
func Installed(cmd []string) (installed, known bool) {
	argv := ProbeArgs(expandArgs(cmd))
	if argv == nil {
		return false, false
	}
	err := runProbe(argv)
	if err == nil {
		return true, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return false, true
	}
	return false, false
}
