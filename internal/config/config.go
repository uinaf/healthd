package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
}

type CheckConfig struct {
	Name     string `toml:"name"`
	Command  string `toml:"command"`
	Interval string `toml:"interval"`
	Timeout  string `toml:"timeout"`
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
