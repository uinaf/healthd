package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	Header      lipgloss.Style
	Divider     lipgloss.Style
	GroupHeader lipgloss.Style
	Pass        lipgloss.Style
	Fail        lipgloss.Style
	TimedOut    lipgloss.Style
	AlertHeader lipgloss.Style
	AlertTime   lipgloss.Style
	AlertCheck  lipgloss.Style
	Empty       lipgloss.Style
	Error       lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		Header:      lipgloss.NewStyle().Bold(true),
		Divider:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		GroupHeader: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		Pass:        lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		Fail:        lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		TimedOut:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		AlertHeader: lipgloss.NewStyle().Bold(true),
		AlertTime:   lipgloss.NewStyle().Width(6).Foreground(lipgloss.Color("245")),
		AlertCheck:  lipgloss.NewStyle().Width(30),
		Empty:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
}
