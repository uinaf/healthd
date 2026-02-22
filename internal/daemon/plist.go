package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

type PlistSpec struct {
	Label      string
	Executable string
	ConfigPath string
	StdoutPath string
	StderrPath string
	Path       string
}

var defaultLaunchAgentPathEntries = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
	"/bin",
	"/usr/sbin",
	"/sbin",
}

func DefaultLaunchAgentPath() string {
	return strings.Join(defaultLaunchAgentPathEntries, ":")
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
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>{{ .Path }}</string>
  </dict>
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
	if strings.TrimSpace(spec.StdoutPath) == "" || strings.TrimSpace(spec.StderrPath) == "" {
		return "", errors.New("plist log paths are required")
	}

	path := strings.TrimSpace(spec.Path)
	if path == "" {
		path = DefaultLaunchAgentPath()
	}

	data := struct {
		Label      string
		Executable string
		ConfigPath string
		StdoutPath string
		StderrPath string
		Path       string
	}{
		Label:      spec.Label,
		Executable: filepath.Clean(spec.Executable),
		ConfigPath: filepath.Clean(spec.ConfigPath),
		StdoutPath: filepath.Clean(spec.StdoutPath),
		StderrPath: filepath.Clean(spec.StderrPath),
		Path:       path,
	}

	var buf bytes.Buffer
	if err := launchAgentTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render plist: %w", err)
	}
	return buf.String(), nil
}
