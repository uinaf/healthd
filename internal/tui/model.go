package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

type runChecksFunc func(context.Context, []config.CheckConfig, string) []runner.CheckResult

type checksMsg struct {
	results []runner.CheckResult
}

type tickMsg time.Time

type Model struct {
	cfg        config.Config
	checks     []config.CheckConfig
	watch      bool
	interval   time.Duration
	results    []runner.CheckResult
	alertsPath string
	alerts     []AlertLine
	alertsErr  error
	styles     styles
	runChecks  runChecksFunc
}

func NewModel(cfg config.Config, checks []config.CheckConfig, watch bool) Model {
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil || interval <= 0 {
		interval = 60 * time.Second
	}

	alertsPath, alertPathErr := DefaultAlertsLogPath()
	m := Model{
		cfg:       cfg,
		checks:    checks,
		watch:     watch,
		interval:  interval,
		styles:    defaultStyles(),
		runChecks: runner.RunChecks,
	}
	if alertPathErr != nil {
		m.alertsErr = alertPathErr
	} else {
		m.alertsPath = alertsPath
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return m.runChecksCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case checksMsg:
		m.results = msg.results
		m.loadAlerts()
		if !m.watch {
			return m, tea.Quit
		}
		return m, m.tickCmd()
	case tickMsg:
		return m, m.runChecksCmd()
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.results) == 0 {
		return "loading checks...\n"
	}

	passed := 0
	for _, result := range m.results {
		if result.Passed {
			passed++
		}
	}
	failed := len(m.results) - passed

	header := fmt.Sprintf("healthd - %d checks - %d ok - %d fail", len(m.results), passed, failed)
	if m.watch {
		header += fmt.Sprintf(" - refreshing every %s", m.interval)
	}

	b := &strings.Builder{}
	b.WriteString(m.styles.Header.Render(header))
	b.WriteByte('\n')
	b.WriteString(m.styles.Divider.Render(strings.Repeat("-", 61)))
	b.WriteByte('\n')
	b.WriteString(m.renderGroups())
	b.WriteByte('\n')
	b.WriteString(m.styles.AlertHeader.Render("Recent Alerts "))
	b.WriteString(m.styles.Divider.Render(strings.Repeat("-", 47)))
	b.WriteByte('\n')
	b.WriteString(m.renderAlerts())
	if m.watch {
		b.WriteString("\n\npress q to quit")
	}
	b.WriteByte('\n')
	return b.String()
}

func (m Model) runChecksCmd() tea.Cmd {
	checks := make([]config.CheckConfig, len(m.checks))
	copy(checks, m.checks)
	defaultTimeout := m.cfg.Timeout
	return func() tea.Msg {
		results := m.runChecks(context.Background(), checks, defaultTimeout)
		return checksMsg{results: results}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) loadAlerts() {
	if m.alertsPath == "" {
		return
	}
	alerts, err := LoadRecentAlerts(m.alertsPath, 10)
	if err != nil {
		m.alertsErr = err
		return
	}
	m.alerts = alerts
	m.alertsErr = nil
}

func (m Model) renderGroups() string {
	grouped := map[string][]runner.CheckResult{}
	for _, result := range m.results {
		grouped[result.Group] = append(grouped[result.Group], result)
	}

	groups := make([]string, 0, len(grouped))
	for group := range grouped {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		left := strings.TrimSpace(groups[i])
		right := strings.TrimSpace(groups[j])
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		return left < right
	})

	b := &strings.Builder{}
	for idx, group := range groups {
		title := strings.TrimSpace(group)
		if title == "" {
			title = "ungrouped"
		}
		b.WriteString(m.styles.GroupHeader.Render(title))
		b.WriteByte('\n')

		entries := grouped[group]
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name < entries[j].Name
		})

		for _, result := range entries {
			indicator, statusStyle := m.statusStyle(result)
			line := fmt.Sprintf("  %s %-30s %-18s %8s", indicator, result.Name, result.Reason, formatDuration(result.Duration))
			b.WriteString(statusStyle.Render(line))
			b.WriteByte('\n')
		}
		if idx < len(groups)-1 {
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderAlerts() string {
	if m.alertsErr != nil {
		return m.styles.Error.Render(m.alertsErr.Error())
	}
	if len(m.alerts) == 0 {
		return m.styles.Empty.Render("no recent alerts")
	}

	b := &strings.Builder{}
	for i := len(m.alerts) - 1; i >= 0; i-- {
		alert := m.alerts[i]
		stateStyle := m.styles.Fail
		if alert.State == "recovered" {
			stateStyle = m.styles.Pass
		} else if alert.State == "warn" {
			stateStyle = m.styles.TimedOut
		}

		b.WriteString(" ")
		b.WriteString(m.styles.AlertTime.Render(alert.Time.Local().Format("15:04")))
		b.WriteString(" ")
		b.WriteString(stateStyle.Width(10).Render(alert.State))
		b.WriteString(" ")
		b.WriteString(m.styles.AlertCheck.Render(alert.CheckName))
		if i > 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m Model) statusStyle(result runner.CheckResult) (string, lipgloss.Style) {
	if result.TimedOut {
		return "!", m.styles.TimedOut
	}
	if result.Passed {
		return "v", m.styles.Pass
	}
	return "x", m.styles.Fail
}

func formatDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return duration.String()
	}
	return duration.Round(time.Millisecond).String()
}
