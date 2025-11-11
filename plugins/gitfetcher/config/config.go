package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type RepoConfig struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	LocalPath string `yaml:"local_path"`
	Interval  string `yaml:"interval"`
}

type Config struct {
	Repos      []RepoConfig `yaml:"repos"`
	SSHKeyPath string       `yaml:"ssh_key_path"`
	HTTPPort   int          `yaml:"http_port"`
	LogPath    string       `yaml:"log_path"`
}

// ParseInterval converts interval string (e.g., "5s", "10m", "1h") to time.Duration
func (r *RepoConfig) ParseInterval() (time.Duration, error) {
	return time.ParseDuration(r.Interval)
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if len(c.Repos) == 0 {
		return fmt.Errorf("no repositories configured")
	}

	for i, repo := range c.Repos {
		if repo.Name == "" {
			return fmt.Errorf("repo[%d]: name is required", i)
		}
		if repo.URL == "" {
			return fmt.Errorf("repo[%d]: url is required", i)
		}
		if repo.LocalPath == "" {
			return fmt.Errorf("repo[%d]: local_path is required", i)
		}
		if _, err := repo.ParseInterval(); err != nil {
			return fmt.Errorf("repo[%d]: invalid interval '%s': %w", i, repo.Interval, err)
		}
	}

	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("invalid http_port: %d", c.HTTPPort)
	}

	return nil
}

// LoadConfig reads and parses the YAML config file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}
	if cfg.LogPath == "" {
		cfg.LogPath = "./logs"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig writes the config back to file
func SaveConfig(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
