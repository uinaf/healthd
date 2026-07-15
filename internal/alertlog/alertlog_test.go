package alertlog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/uinaf/healthd/internal/alertlog"
)

func TestFormatLineProducesTUIParseableOutput(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 27, 13, 0, 0, 0, time.UTC)
	line := alertlog.FormatLine(ts, "recovered", "colima-running", "services", "ok")

	expected := "2026-02-27T13:00:00Z [recovered] colima-running (services) - ok"
	if line != expected {
		t.Fatalf("unexpected line:\nwant: %q\ngot:  %q", expected, line)
	}
}

func TestFormatLineCollapsesNewlinesInReason(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 27, 8, 37, 0, 0, time.UTC)
	line := alertlog.FormatLine(ts, "crit", "noisy", "host", "first line\nsecond line\r\nthird")

	if strings.ContainsAny(line, "\n\r") {
		t.Fatalf("line contains newlines: %q", line)
	}
	if !strings.HasSuffix(line, " - first line second line third") {
		t.Fatalf("reason not collapsed as expected: %q", line)
	}
}

func TestFormatLineCapsReasonLength(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 27, 8, 37, 0, 0, time.UTC)
	huge := strings.Repeat("x", 8*1024)
	line := alertlog.FormatLine(ts, "crit", "noisy", "host", huge)
	if !strings.HasSuffix(line, "…") {
		t.Fatalf("expected capped reason marker, got len=%d", len(line))
	}
	if len(line) > 5*1024 {
		t.Fatalf("expected formatted line to stay bounded, got %d", len(line))
	}
}

func TestAppendCreatesParentDirAndAppends(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "state", "alerts.log")

	ts1 := time.Date(2026, 2, 27, 8, 37, 0, 0, time.UTC)
	if err := alertlog.Append(path, ts1, "crit", "openclaw-up-to-date", "services", "expected exit_code=0, got 1"); err != nil {
		t.Fatalf("first append: %v", err)
	}

	ts2 := time.Date(2026, 2, 27, 13, 0, 0, 0, time.UTC)
	if err := alertlog.Append(path, ts2, "recovered", "colima-running", "services", "ok"); err != nil {
		t.Fatalf("second append: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), lines)
	}

	alerts, err := alertlog.LoadRecent(path, 10)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 parsed alerts, got %d", len(alerts))
	}
	if alerts[0].State != "crit" || alerts[0].CheckName != "openclaw-up-to-date" || alerts[0].Group != "services" {
		t.Fatalf("first parsed alert mismatch: %+v", alerts[0])
	}
	if alerts[1].State != "recovered" || alerts[1].CheckName != "colima-running" {
		t.Fatalf("second parsed alert mismatch: %+v", alerts[1])
	}
}

func TestDefaultPathUnderUserHome(t *testing.T) {
	t.Parallel()

	got, err := alertlog.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	expected := filepath.Join(home, ".local", "state", "healthd", "alerts.log")
	if got != expected {
		t.Fatalf("unexpected path: want %q got %q", expected, got)
	}
}

func TestParseLineRejectsBadInput(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"not an alert line",
		"2026-02-27T13:00:00Z [crit] name - missing group",
		"not-a-time [crit] name (group) - reason",
	}
	for _, raw := range cases {
		if _, ok := alertlog.ParseLine(raw); ok {
			t.Fatalf("expected ParseLine to reject %q", raw)
		}
	}

	line, ok := alertlog.ParseLine("2026-02-27T13:00:00Z [crit] api (svc) - boom")
	if !ok {
		t.Fatal("expected valid line to parse")
	}
	if line.State != "crit" || line.CheckName != "api" || line.Group != "svc" || line.Reason != "boom" {
		t.Fatalf("unexpected parsed line: %+v", line)
	}
}

func TestLoadRecentLimitAndMissing(t *testing.T) {
	t.Parallel()

	if alerts, err := alertlog.LoadRecent(filepath.Join(t.TempDir(), "missing.log"), 10); err != nil || len(alerts) != 0 {
		t.Fatalf("expected empty missing file result, got %v err=%v", alerts, err)
	}
	if alerts, err := alertlog.LoadRecent(filepath.Join(t.TempDir(), "x.log"), 0); err != nil || len(alerts) != 0 {
		t.Fatalf("expected empty for non-positive limit, got %v err=%v", alerts, err)
	}

	path := filepath.Join(t.TempDir(), "alerts.log")
	content := "bad\n" +
		"2026-02-27T08:00:00Z [crit] a (g) - one\n" +
		"2026-02-27T09:00:00Z [recovered] b (g) - two\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	alerts, err := alertlog.LoadRecent(path, 1)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(alerts) != 1 || alerts[0].CheckName != "b" {
		t.Fatalf("expected last alert only, got %+v", alerts)
	}
}

func TestLoadRecentKeepsRollingWindow(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "alerts.log")
	var b strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "2026-02-27T08:%02d:00Z [crit] c%d (g) - r%d\n", i, i, i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	alerts, err := alertlog.LoadRecent(path, 3)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(alerts) != 3 {
		t.Fatalf("expected 3 alerts, got %d", len(alerts))
	}
	if alerts[0].CheckName != "c17" || alerts[2].CheckName != "c19" {
		t.Fatalf("expected last three alerts, got %+v", alerts)
	}
}

func TestLoadRecentSkipsOversizedLineAndContinues(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "alerts.log")
	var b strings.Builder
	b.WriteString("2026-02-27T08:00:00Z [crit] early (g) - one\n")
	b.WriteString("2026-02-27T08:01:00Z [crit] huge (g) - ")
	b.WriteString(strings.Repeat("x", 256*1024+10))
	b.WriteString("\n")
	b.WriteString("2026-02-27T08:02:00Z [recovered] late (g) - two\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	alerts, err := alertlog.LoadRecent(path, 10)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts after skipping oversized line, got %+v", alerts)
	}
	if alerts[0].CheckName != "early" || alerts[1].CheckName != "late" {
		t.Fatalf("unexpected alerts after oversized skip: %+v", alerts)
	}
}

func TestFormatLineCapsReasonOnUTF8Boundary(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 27, 8, 37, 0, 0, time.UTC)
	// Fill almost to the cap, then add a multi-byte rune that would be split.
	prefix := strings.Repeat("a", 4*1024-1)
	reason := prefix + "⌘extra"
	line := alertlog.FormatLine(ts, "crit", "noisy", "host", reason)
	if !utf8.ValidString(line) {
		t.Fatalf("formatted line is invalid UTF-8: %q", line)
	}
	if !strings.HasSuffix(line, "…") {
		t.Fatalf("expected truncation marker, got %q", line)
	}
}

func TestValidateSafeIdentifier(t *testing.T) {
	t.Parallel()

	if err := alertlog.ValidateSafeIdentifier("name", "disk-root"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := alertlog.ValidateSafeIdentifier("name", "bad(name"); err == nil {
		t.Fatal("expected delimiter rejection")
	}
	if err := alertlog.ValidateSafeIdentifier("name", "bad\nname"); err == nil {
		t.Fatal("expected newline rejection")
	}
}
