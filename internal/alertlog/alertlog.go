// Package alertlog is the canonical writer for ~/.local/state/healthd/alerts.log.
// The TUI reads this file in internal/tui/alerts.go; the line format here must
// match the parser there (alertLinePattern).
package alertlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".local", "state", "healthd", "alerts.log"), nil
}

func FormatLine(t time.Time, state, checkName, group, reason string) string {
	return fmt.Sprintf(
		"%s [%s] %s (%s) - %s",
		t.UTC().Format(time.RFC3339),
		state,
		strings.TrimSpace(checkName),
		strings.TrimSpace(group),
		sanitizeReason(reason),
	)
}

func Append(path string, t time.Time, state, checkName, group, reason string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create alerts log dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open alerts log: %w", err)
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, FormatLine(t, state, checkName, group, reason)); err != nil {
		return fmt.Errorf("write alerts log: %w", err)
	}
	return nil
}

func sanitizeReason(reason string) string {
	collapsed := strings.ReplaceAll(reason, "\r\n", " ")
	collapsed = strings.ReplaceAll(collapsed, "\n", " ")
	collapsed = strings.ReplaceAll(collapsed, "\r", " ")
	return strings.TrimSpace(collapsed)
}
