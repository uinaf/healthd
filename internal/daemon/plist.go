package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type PlistSpec struct {
	Label      string
	Executable string
	ConfigPath string
	Interval   time.Duration
	StdoutPath string
	StderrPath string
}

var launchAgentTemplate = template.Must(template.New("launchagent").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{ .Label }}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .Executable }}</string>
    <string>daemon</string>
    <string>run</string>
    <string>--config</string>
    <string>{{ .ConfigPath }}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StartInterval</key>
  <integer>{{ .IntervalSeconds }}</integer>
  <key>StandardOutPath</key>
  <string>{{ .StdoutPath }}</string>
  <key>StandardErrorPath</key>
  <string>{{ .StderrPath }}</string>
</dict>
</plist>
`))

func RenderPlist(spec PlistSpec) (string, error) {
	if strings.TrimSpace(spec.Label) == "" {
		return "", errors.New("plist label is required")
	}
	if strings.TrimSpace(spec.Executable) == "" {
		return "", errors.New("plist executable is required")
	}
	if strings.TrimSpace(spec.ConfigPath) == "" {
		return "", errors.New("plist config path is required")
	}
	if spec.Interval <= 0 {
		return "", errors.New("plist interval must be greater than zero")
	}
	if strings.TrimSpace(spec.StdoutPath) == "" || strings.TrimSpace(spec.StderrPath) == "" {
		return "", errors.New("plist log paths are required")
	}

	data := struct {
		Label           string
		Executable      string
		ConfigPath      string
		IntervalSeconds int64
		StdoutPath      string
		StderrPath      string
	}{
		Label:           spec.Label,
		Executable:      filepath.Clean(spec.Executable),
		ConfigPath:      filepath.Clean(spec.ConfigPath),
		IntervalSeconds: int64(spec.Interval / time.Second),
		StdoutPath:      filepath.Clean(spec.StdoutPath),
		StderrPath:      filepath.Clean(spec.StderrPath),
	}

	var buf bytes.Buffer
	if err := launchAgentTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render plist: %w", err)
	}
	return buf.String(), nil
}
