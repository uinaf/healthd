package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var alertLinePattern = regexp.MustCompile(`^(\S+) \[([^\]]+)\] ([^(]+) \(([^)]*)\) - (.*)$`)

type AlertLine struct {
	Time      time.Time
	State     string
	CheckName string
	Group     string
	Reason    string
}

func DefaultAlertsLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".local", "state", "healthd", "alerts.log"), nil
}

func LoadRecentAlerts(path string, limit int) ([]AlertLine, error) {
	if limit <= 0 {
		return []AlertLine{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []AlertLine{}, nil
		}
		return nil, fmt.Errorf("open alerts log %q: %w", path, err)
	}
	defer file.Close()

	parsed := make([]AlertLine, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, ok := parseAlertLine(line)
		if !ok {
			continue
		}
		parsed = append(parsed, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read alerts log %q: %w", path, err)
	}

	if len(parsed) <= limit {
		return parsed, nil
	}
	return parsed[len(parsed)-limit:], nil
}

func parseAlertLine(line string) (AlertLine, bool) {
	matches := alertLinePattern.FindStringSubmatch(line)
	if len(matches) != 6 {
		return AlertLine{}, false
	}

	ts, err := time.Parse(time.RFC3339, matches[1])
	if err != nil {
		return AlertLine{}, false
	}

	return AlertLine{
		Time:      ts,
		State:     strings.TrimSpace(matches[2]),
		CheckName: strings.TrimSpace(matches[3]),
		Group:     strings.TrimSpace(matches[4]),
		Reason:    strings.TrimSpace(matches[5]),
	}, true
}
