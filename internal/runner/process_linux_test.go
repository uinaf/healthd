package runner

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func assertProcessStopped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err == nil {
			_, state, ok := strings.Cut(string(data), ") ")
			if ok && strings.HasPrefix(state, "Z ") {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("child %d still live", pid)
}

func awaitZombie(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			t.Fatal(err)
		}
		_, state, ok := strings.Cut(string(data), ") ")
		if ok && strings.HasPrefix(state, "Z ") {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("released leader did not become an unreaped zombie")
}
