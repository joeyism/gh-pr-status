package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultPollInterval = 30 * time.Second
	minimumPollInterval = 5 * time.Second
)

// Config describes the values that can be customized in the YAML file.
type Config struct {
	Orgs         []string `yaml:"orgs"`
	PollInterval string   `yaml:"poll_interval"`
}

// PollDuration returns the effective polling interval after applying defaults.
func (c Config) PollDuration() time.Duration {
	if c.PollInterval == "" {
		return defaultPollInterval
	}
	parsed, err := time.ParseDuration(c.PollInterval)
	if err != nil || parsed < minimumPollInterval {
		fmt.Fprintf(os.Stderr, "warning: invalid poll_interval %q, using default %s\n", c.PollInterval, defaultPollInterval)
		return defaultPollInterval
	}
	return parsed
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh-prs", "config.yaml")
}

// LoadConfig reads a YAML config file from the provided path (or default path when empty).
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
