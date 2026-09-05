package runner

import (
	"golang.org/x/sys/unix"
	"testing"
	"time"
)

func assertProcessStopped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		processes, err := unix.SysctlKinfoProcSlice("kern.proc.pid", pid)
		if err == unix.ESRCH || (err == nil && (len(processes) == 0 || containsOnlyZombieLeader(processes, pid))) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("child %d still live", pid)
}
