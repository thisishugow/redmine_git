package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     time.Duration
		wantErr  bool
	}{
		{"5 seconds", "5s", 5 * time.Second, false},
		{"10 minutes", "10m", 10 * time.Minute, false},
		{"1 hour", "1h", 1 * time.Hour, false},
		{"2h30m", "2h30m", 2*time.Hour + 30*time.Minute, false},
		{"invalid", "invalid", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := RepoConfig{Interval: tt.interval}
			got, err := repo.ParseInterval()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInterval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:      "test-repo",
						URL:       "git@github.com:user/repo.git",
						LocalPath: "/repos/test.git",
						Interval:  "5m",
					},
				},
				HTTPPort: 8080,
			},
			wantErr: false,
		},
		{
			name: "no repos",
			config: Config{
				Repos:    []RepoConfig{},
				HTTPPort: 8080,
			},
			wantErr: true,
			errMsg:  "no repositories configured",
		},
		{
			name: "missing repo name",
			config: Config{
				Repos: []RepoConfig{
					{
						URL:       "git@github.com:user/repo.git",
						LocalPath: "/repos/test.git",
						Interval:  "5m",
					},
				},
				HTTPPort: 8080,
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing repo url",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:      "test",
						LocalPath: "/repos/test.git",
						Interval:  "5m",
					},
				},
				HTTPPort: 8080,
			},
			wantErr: true,
			errMsg:  "url is required",
		},
		{
			name: "missing local path",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:     "test",
						URL:      "git@github.com:user/repo.git",
						Interval: "5m",
					},
				},
				HTTPPort: 8080,
			},
			wantErr: true,
			errMsg:  "local_path is required",
		},
		{
			name: "invalid interval",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:      "test",
						URL:       "git@github.com:user/repo.git",
						LocalPath: "/repos/test.git",
						Interval:  "invalid",
					},
				},
				HTTPPort: 8080,
			},
			wantErr: true,
			errMsg:  "invalid interval",
		},
		{
			name: "invalid port - too high",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:      "test",
						URL:       "git@github.com:user/repo.git",
						LocalPath: "/repos/test.git",
						Interval:  "5m",
					},
				},
				HTTPPort: 99999,
			},
			wantErr: true,
			errMsg:  "invalid http_port",
		},
		{
			name: "invalid port - negative",
			config: Config{
				Repos: []RepoConfig{
					{
						Name:      "test",
						URL:       "git@github.com:user/repo.git",
						LocalPath: "/repos/test.git",
						Interval:  "5m",
					},
				},
				HTTPPort: -1,
			},
			wantErr: true,
			errMsg:  "invalid http_port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || err.Error() == "" {
					t.Errorf("Expected error message containing '%s', got nil", tt.errMsg)
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	validYAML := `
repos:
  - name: "test-repo"
    url: "git@github.com:user/repo.git"
    local_path: "/repos/test.git"
    interval: "5m"
  - name: "another-repo"
    url: "git@github.com:user/another.git"
    local_path: "/repos/another.git"
    interval: "1h"

ssh_key_path: "/root/.ssh/id_rsa"
http_port: 8080
log_path: "./logs"
`

	if err := os.WriteFile(configPath, []byte(validYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify loaded config
	if len(cfg.Repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(cfg.Repos))
	}

	if cfg.Repos[0].Name != "test-repo" {
		t.Errorf("Expected first repo name 'test-repo', got '%s'", cfg.Repos[0].Name)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("Expected HTTPPort 8080, got %d", cfg.HTTPPort)
	}

	if cfg.SSHKeyPath != "/root/.ssh/id_rsa" {
		t.Errorf("Expected SSHKeyPath '/root/.ssh/id_rsa', got '%s'", cfg.SSHKeyPath)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal-config.yaml")

	// Minimal config without optional fields
	minimalYAML := `
repos:
  - name: "test-repo"
    url: "git@github.com:user/repo.git"
    local_path: "/repos/test.git"
    interval: "5m"
`

	if err := os.WriteFile(configPath, []byte(minimalYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check defaults
	if cfg.HTTPPort != 8080 {
		t.Errorf("Expected default HTTPPort 8080, got %d", cfg.HTTPPort)
	}

	if cfg.LogPath != "./logs" {
		t.Errorf("Expected default LogPath './logs', got '%s'", cfg.LogPath)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file, got nil")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
repos:
  - name: "test
    invalid yaml here
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "save-test.yaml")

	cfg := &Config{
		Repos: []RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/repo.git",
				LocalPath: "/repos/test.git",
				Interval:  "5m",
			},
		},
		SSHKeyPath: "/root/.ssh/id_rsa",
		HTTPPort:   8080,
		LogPath:    "./logs",
	}

	// Save config
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load it back and verify
	loadedCfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if len(loadedCfg.Repos) != len(cfg.Repos) {
		t.Errorf("Expected %d repos, got %d", len(cfg.Repos), len(loadedCfg.Repos))
	}

	if loadedCfg.HTTPPort != cfg.HTTPPort {
		t.Errorf("Expected HTTPPort %d, got %d", cfg.HTTPPort, loadedCfg.HTTPPort)
	}
}

func TestSaveConfigInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-save.yaml")

	// Config with no repos (invalid)
	cfg := &Config{
		Repos:    []RepoConfig{},
		HTTPPort: 8080,
	}

	err := SaveConfig(configPath, cfg)
	if err == nil {
		t.Error("Expected error when saving invalid config, got nil")
	}
}
