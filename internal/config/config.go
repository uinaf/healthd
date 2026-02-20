package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	DefaultConfigPath = "~/.config/healthd/config.toml"
	EnvConfigPath     = "HEALTHD_CONFIG"
)

type Config struct {
	Interval string        `toml:"interval"`
	Timeout  string        `toml:"timeout"`
	Checks   []CheckConfig `toml:"check"`
	Notify   NotifyConfig  `toml:"notify"`
}

type CheckConfig struct {
	Name     string            `toml:"name"`
	Command  string            `toml:"command"`
	Group    string            `toml:"group"`
	Interval string            `toml:"interval"`
	Timeout  string            `toml:"timeout"`
	Env      map[string]string `toml:"env"`
	Expect   ExpectConfig      `toml:"expect"`
}

type ExpectConfig struct {
	ExitCode    *int     `toml:"exit_code"`
	Equals      *string  `toml:"equals"`
	Not         *string  `toml:"not"`
	Contains    *string  `toml:"contains"`
	NotContains *string  `toml:"not_contains"`
	Min         *float64 `toml:"min"`
	Max         *float64 `toml:"max"`
	Regex       *string  `toml:"regex"`
}

type NotifyConfig struct {
	Cooldown string                `toml:"cooldown"`
	Backends []NotifyBackendConfig `toml:"backend"`
}

type NotifyBackendConfig struct {
	Name    string `toml:"name"`
	Type    string `toml:"type"`
	URL     string `toml:"url"`
	Topic   string `toml:"topic"`
	Command string `toml:"command"`
	Timeout string `toml:"timeout"`
}

func DefaultConfig() Config {
	return Config{
		Interval: "60s",
		Timeout:  "10s",
	}
}

func ResolvePath(cliPath string) (string, error) {
	for _, candidate := range []string{cliPath, os.Getenv(EnvConfigPath), DefaultConfigPath} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		return expandPath(candidate)
	}

	return "", errors.New("unable to resolve config path")
}

func LoadFromPath(path string) (Config, error) {
	path, err := expandPath(path)
	if err != nil {
		return Config{}, err
	}

	cfg := DefaultConfig()
	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("failed to decode config %q: %w", path, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return Config{}, fmt.Errorf("unknown config fields: %s", joinKeys(undecoded))
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if err := validateDurationField("interval", c.Interval); err != nil {
		return err
	}

	if err := validateDurationField("timeout", c.Timeout); err != nil {
		return err
	}

	if len(c.Checks) == 0 {
		return errors.New("at least one [[check]] entry is required")
	}

	for i, check := range c.Checks {
		prefix := fmt.Sprintf("check[%d]", i)
		if strings.TrimSpace(check.Name) == "" {
			return fmt.Errorf("%s.name is required", prefix)
		}
		if strings.TrimSpace(check.Command) == "" {
			return fmt.Errorf("%s.command is required", prefix)
		}
		if check.Interval != "" {
			if err := validateDurationField(prefix+".interval", check.Interval); err != nil {
				return err
			}
		}
		if check.Timeout != "" {
			if err := validateDurationField(prefix+".timeout", check.Timeout); err != nil {
				return err
			}
		}
		if err := validateEnvField(prefix+".env", check.Env); err != nil {
			return err
		}
		if err := validateExpectField(prefix+".expect", check.Expect); err != nil {
			return err
		}
	}

	if err := validateNotifyConfig(c.Notify); err != nil {
		return err
	}

	return nil
}

func validateDurationField(name, value string) error {
	d, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if d <= 0 {
		return fmt.Errorf("%s must be greater than zero", name)
	}
	return nil
}

func validateEnvField(name string, env map[string]string) error {
	for key := range env {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			return fmt.Errorf("%s key must not be empty", name)
		}
		if strings.Contains(trimmed, "=") {
			return fmt.Errorf("%s key %q must not contain '='", name, key)
		}
	}
	return nil
}

func validateExpectField(name string, expect ExpectConfig) error {
	if expect.Min != nil && expect.Max != nil && *expect.Min > *expect.Max {
		return fmt.Errorf("%s min must be less than or equal to max", name)
	}
	if expect.Regex != nil {
		if _, err := regexp.Compile(*expect.Regex); err != nil {
			return fmt.Errorf("%s regex is invalid: %w", name, err)
		}
	}
	return nil
}

func validateNotifyConfig(notify NotifyConfig) error {
	if strings.TrimSpace(notify.Cooldown) != "" {
		if err := validateDurationField("notify.cooldown", notify.Cooldown); err != nil {
			return err
		}
	}

	names := map[string]struct{}{}
	for i, backend := range notify.Backends {
		prefix := fmt.Sprintf("notify.backend[%d]", i)
		if err := validateNotifyBackend(prefix, backend); err != nil {
			return err
		}

		name := strings.TrimSpace(backend.Name)
		if name == "" {
			name = strings.TrimSpace(backend.Type)
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("%s name %q must be unique", prefix, name)
		}
		names[name] = struct{}{}
	}

	return nil
}

func validateNotifyBackend(prefix string, backend NotifyBackendConfig) error {
	backendType := strings.TrimSpace(backend.Type)
	if backendType == "" {
		return fmt.Errorf("%s.type is required", prefix)
	}

	if strings.TrimSpace(backend.Timeout) != "" {
		if err := validateDurationField(prefix+".timeout", backend.Timeout); err != nil {
			return err
		}
	}

	switch backendType {
	case "ntfy":
		topic := strings.TrimSpace(backend.Topic)
		if topic == "" || strings.Trim(topic, "/") == "" {
			return fmt.Errorf("%s.topic is required for ntfy backend", prefix)
		}
	case "webhook":
		if strings.TrimSpace(backend.URL) == "" {
			return fmt.Errorf("%s.url is required for webhook backend", prefix)
		}
	case "command":
		if strings.TrimSpace(backend.Command) == "" {
			return fmt.Errorf("%s.command is required for command backend", prefix)
		}
	default:
		return fmt.Errorf("%s.type must be one of ntfy, webhook, command", prefix)
	}

	return nil
}

func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine user home: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}

	return filepath.Clean(path), nil
}

func joinKeys(keys []toml.Key) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key.String())
	}
	return strings.Join(parts, ", ")
}
