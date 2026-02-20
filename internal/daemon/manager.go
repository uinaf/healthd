package daemon

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const LaunchAgentLabel = "com.uinaf.healthd"

type Paths struct {
	PlistPath  string
	StdoutPath string
	StderrPath string
}

type Status struct {
	Installed bool
	Running   bool
	PID       int
}

type LaunchCtl interface {
	Load(plistPath string) error
	Unload(plistPath string) error
	List(label string) (string, error)
}

type Manager struct {
	launchctl LaunchCtl
	homeDir   func() (string, error)
	execPath  func() (string, error)
	mkdirAll  func(string, os.FileMode) error
	writeFile func(string, []byte, os.FileMode) error
	remove    func(string) error
	stat      func(string) (os.FileInfo, error)
	readFile  func(string) ([]byte, error)
}

func NewManager() *Manager {
	return &Manager{
		launchctl: execLaunchCtl{},
		homeDir:   os.UserHomeDir,
		execPath:  os.Executable,
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
		remove:    os.Remove,
		stat:      os.Stat,
		readFile:  os.ReadFile,
	}
}

func DefaultPaths(home string) Paths {
	return Paths{
		PlistPath:  filepath.Join(home, "Library", "LaunchAgents", LaunchAgentLabel+".plist"),
		StdoutPath: filepath.Join(home, "Library", "Logs", "healthd", "stdout.log"),
		StderrPath: filepath.Join(home, "Library", "Logs", "healthd", "stderr.log"),
	}
}

func (m *Manager) Install(configPath string) (Paths, error) {
	home, err := m.homeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("determine user home: %w", err)
	}
	paths := DefaultPaths(home)

	executable, err := m.execPath()
	if err != nil {
		return Paths{}, fmt.Errorf("determine executable path: %w", err)
	}
	executable = filepath.Clean(executable)

	if err := m.mkdirAll(filepath.Dir(paths.PlistPath), 0o755); err != nil {
		return Paths{}, fmt.Errorf("create launchagent dir: %w", err)
	}
	if err := m.mkdirAll(filepath.Dir(paths.StdoutPath), 0o755); err != nil {
		return Paths{}, fmt.Errorf("create log dir: %w", err)
	}

	plist, err := RenderPlist(PlistSpec{
		Label:      LaunchAgentLabel,
		Executable: executable,
		ConfigPath: filepath.Clean(configPath),
		StdoutPath: paths.StdoutPath,
		StderrPath: paths.StderrPath,
	})
	if err != nil {
		return Paths{}, err
	}

	if err := m.writeFile(paths.PlistPath, []byte(plist), 0o644); err != nil {
		return Paths{}, fmt.Errorf("write launchagent plist: %w", err)
	}

	_ = m.launchctl.Unload(paths.PlistPath)

	if err := m.launchctl.Load(paths.PlistPath); err != nil {
		return Paths{}, fmt.Errorf("load launchagent: %w", err)
	}

	return paths, nil
}

func (m *Manager) Uninstall() (Paths, error) {
	home, err := m.homeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("determine user home: %w", err)
	}
	paths := DefaultPaths(home)

	_ = m.launchctl.Unload(paths.PlistPath)

	if err := m.remove(paths.PlistPath); err != nil && !os.IsNotExist(err) {
		return Paths{}, fmt.Errorf("remove launchagent plist: %w", err)
	}

	return paths, nil
}

func (m *Manager) Status() (Status, Paths, error) {
	home, err := m.homeDir()
	if err != nil {
		return Status{}, Paths{}, fmt.Errorf("determine user home: %w", err)
	}
	paths := DefaultPaths(home)

	if _, err := m.stat(paths.PlistPath); err != nil {
		if os.IsNotExist(err) {
			return Status{Installed: false, Running: false, PID: 0}, paths, nil
		}
		return Status{}, Paths{}, fmt.Errorf("read launchagent status: %w", err)
	}

	output, err := m.launchctl.List(LaunchAgentLabel)
	if err != nil {
		return Status{Installed: true, Running: false, PID: 0}, paths, nil
	}

	pid := ParsePID(output)
	return Status{Installed: true, Running: pid > 0, PID: pid}, paths, nil
}

func (m *Manager) ReadLogs(lines int) (string, string, error) {
	home, err := m.homeDir()
	if err != nil {
		return "", "", fmt.Errorf("determine user home: %w", err)
	}
	paths := DefaultPaths(home)

	stdout, err := m.tailFile(paths.StdoutPath, lines)
	if err != nil {
		return "", "", fmt.Errorf("read stdout log: %w", err)
	}
	stderr, err := m.tailFile(paths.StderrPath, lines)
	if err != nil {
		return "", "", fmt.Errorf("read stderr log: %w", err)
	}

	return stdout, stderr, nil
}

func (m *Manager) tailFile(path string, lines int) (string, error) {
	data, err := m.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return TailLines(data, lines), nil
}

func ParsePID(output string) int {
	re := regexp.MustCompile(`(?m)(?:\"PID\"|pid)\s*=\s*([0-9]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return 0
	}
	pid, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return pid
}

func TailLines(data []byte, lines int) string {
	if lines <= 0 {
		return ""
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	all := make([]string, 0)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if len(all) == 0 {
		return ""
	}
	if len(all) <= lines {
		return strings.Join(all, "\n")
	}
	return strings.Join(all[len(all)-lines:], "\n")
}

type execLaunchCtl struct{}

func (execLaunchCtl) Load(plistPath string) error {
	cmd := exec.Command("launchctl", "load", "-w", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (execLaunchCtl) Unload(plistPath string) error {
	cmd := exec.Command("launchctl", "unload", "-w", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl unload: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (execLaunchCtl) List(label string) (string, error) {
	cmd := exec.Command("launchctl", "list", label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("launchctl list: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
