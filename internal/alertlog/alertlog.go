// Package alertlog is the canonical writer and parser for
// ~/.local/state/healthd/alerts.log.
package alertlog

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// maxReasonBytes caps persisted/parsed reason text so a single alerts.log
	// line cannot overwhelm the TUI scanner.
	maxReasonBytes = 4 * 1024
	// maxAlertLineBytes is the scanner token limit for one alerts.log line.
	maxAlertLineBytes = 256 * 1024
)

var linePattern = regexp.MustCompile(`^(\S+) \[([^\]]+)\] ([^(]+) \(([^)]*)\) - (.*)$`)

// Line is one parsed alerts.log entry.
type Line struct {
	Time      time.Time
	State     string
	CheckName string
	Group     string
	Reason    string
}

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

// ParseLine parses one alerts.log line. Returns false when the line does not
// match the canonical format.
func ParseLine(raw string) (Line, bool) {
	matches := linePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 6 {
		return Line{}, false
	}

	ts, err := time.Parse(time.RFC3339, matches[1])
	if err != nil {
		return Line{}, false
	}

	return Line{
		Time:      ts,
		State:     strings.TrimSpace(matches[2]),
		CheckName: strings.TrimSpace(matches[3]),
		Group:     strings.TrimSpace(matches[4]),
		Reason:    strings.TrimSpace(matches[5]),
	}, true
}

// LoadRecent returns the last limit parsed lines from path. Missing files yield
// an empty slice without error. Scanning keeps only a rolling window of
// `limit` entries so watch-mode refreshes stay O(limit) in memory.
func LoadRecent(path string, limit int) ([]Line, error) {
	if limit <= 0 {
		return []Line{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Line{}, nil
		}
		return nil, fmt.Errorf("open alerts log %q: %w", path, err)
	}
	defer file.Close()

	buf := make([]Line, limit)
	write := 0
	count := 0

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxAlertLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, ok := ParseLine(line)
		if !ok {
			continue
		}
		buf[write] = entry
		write = (write + 1) % limit
		if count < limit {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		// Oversized historical lines should not blank the whole alerts panel.
		if !errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("read alerts log %q: %w", path, err)
		}
	}

	out := make([]Line, count)
	start := 0
	if count == limit {
		start = write
	}
	for i := 0; i < count; i++ {
		out[i] = buf[(start+i)%limit]
	}
	return out, nil
}

// ValidateSafeIdentifier rejects characters that would break the alerts.log
// line format delimiters used by FormatLine/ParseLine.
func ValidateSafeIdentifier(field, value string) error {
	for _, r := range value {
		switch r {
		case '(', ')', '[', ']':
			return fmt.Errorf("%s %q must not contain '%c' (would break alerts.log parsing)", field, value, r)
		case '\n', '\r':
			return fmt.Errorf("%s %q must not contain newlines", field, value)
		}
	}
	return nil
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
	collapsed = strings.TrimSpace(collapsed)
	if len(collapsed) > maxReasonBytes {
		return collapsed[:maxReasonBytes] + "…"
	}
	return collapsed
}
