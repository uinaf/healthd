//go:build linux

package runner

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

func waitForExit(ctx context.Context, pid int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOHANG|unix.WNOWAIT, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Signo != 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			// Observe once more after cancellation wins the select: exit may have
			// arrived since the prior poll. WNOWAIT still reserves the leader's PID.
			for {
				var final unix.Siginfo
				err := unix.Waitid(unix.P_PID, pid, &final, unix.WEXITED|unix.WNOHANG|unix.WNOWAIT, nil)
				if errors.Is(err, unix.EINTR) {
					continue
				}
				if err != nil {
					return err
				}
				if final.Signo != 0 {
					return nil
				}
				return ctx.Err()
			}
		case <-ticker.C:
		}
	}
}

func ignoreCleanupErrorAfterExit(_ int, _ error) bool {
	return false
}
