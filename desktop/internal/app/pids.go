package app

import "github.com/jamesread/biscuittin/internal/launch"

func livePIDs(pids []int) []int {
	var live []int
	for _, pid := range pids {
		if launch.Alive(pid) {
			live = append(live, pid)
		}
	}
	return live
}

func hasLivePID(pids []int) bool {
	return len(livePIDs(pids)) > 0
}

func removePID(pids []int, pid int) []int {
	var out []int
	for _, p := range pids {
		if p != pid {
			out = append(out, p)
		}
	}
	return out
}

func killPIDs(pids []int) error {
	var last error
	for _, pid := range pids {
		if err := launch.Kill(pid); err != nil {
			last = err
		}
	}
	return last
}
