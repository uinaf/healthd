package daemon

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/uinaf/healthd/internal/config"
)

func TestRunLoopRejectsNonPositiveInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		interval string
	}{
		{name: "zero", interval: "0s"},
		{name: "negative", interval: "-1s"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{
				Interval: tc.interval,
				Timeout:  "1s",
				Checks: []config.CheckConfig{
					{Name: "ok", Command: "true"},
				},
			}

			err := RunLoop(context.Background(), cfg, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "schedule interval must be greater than zero") {
				t.Fatalf("expected non-positive interval error, got %v", err)
			}
		})
	}
}
