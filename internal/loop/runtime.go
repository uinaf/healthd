package loop

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/uinaf/healthd/internal/alertlog"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/notify"
	"github.com/uinaf/healthd/internal/runner"
)

func Run(ctx context.Context, cfg config.Config, out io.Writer) error {
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return fmt.Errorf("parse schedule interval: %w", err)
	}
	if interval <= 0 {
		return fmt.Errorf("schedule interval must be greater than zero")
	}

	cooldown, err := notify.ParseCooldown(cfg.Notify.Cooldown)
	if err != nil {
		return err
	}
	tracker := notify.NewTracker(cooldown)

	notifiers := make([]notify.Notifier, 0)
	if len(cfg.Notify.Backends) > 0 {
		notifiers, err = notify.BuildNotifiers(cfg.Notify, nil)
		if err != nil {
			return err
		}
	}

	alertsPath, alertsPathErr := alertlog.DefaultPath()
	if alertsPathErr != nil {
		fmt.Fprintf(out, "alerts log disabled: %v\n", alertsPathErr)
	}

	runOnce := func() {
		results := runner.RunChecks(ctx, cfg.Checks, cfg.Timeout)
		for _, result := range results {
			event, ok := tracker.EventFor(result)
			if !ok {
				continue
			}
			if alertsPath != "" {
				if err := alertlog.Append(alertsPath, event.Timestamp, string(event.State), event.CheckName, event.Group, event.Reason); err != nil {
					fmt.Fprintf(out, "alerts log write error for %s: %v\n", result.Name, err)
				}
			}
			if len(notifiers) == 0 {
				continue
			}
			if dispatchErr := notify.Dispatch(ctx, event, notifiers); dispatchErr != nil {
				fmt.Fprintf(out, "notify dispatch error for %s: %v\n", result.Name, dispatchErr)
			}
		}
	}

	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runOnce()
		}
	}
}
