package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// 建立測試配置檔
	configContent := `
redmine:
  url: "https://redmine.example.com"
  api_key: "test-api-key"
  projects:
    - identifier: "test-project"
      custom_fields:
        target_repo_id: 10
        github_issue_url_id: 11

github:
  token: "ghp_test_token"
  base_url: "https://github.com"

sync:
  interval: "5m"
  title_format: "[Redmine #%d] %s"
  on_error:
    log: true
    add_redmine_note: false
    log_file: "/tmp/test.log"

database:
  host: "localhost"
  port: 5432
  name: "testdb"
  user: "testuser"
  password: "testpass"
  schema: "test_schema"
  sslmode: "disable"
`

	// 建立臨時配置檔
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	// 測試載入配置
	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 驗證 Redmine 配置
	assert.Equal(t, "https://redmine.example.com", cfg.Redmine.URL)
	assert.Equal(t, "test-api-key", cfg.Redmine.APIKey)
	assert.Len(t, cfg.Redmine.Projects, 1)
	assert.Equal(t, "test-project", cfg.Redmine.Projects[0].Identifier)
	assert.Equal(t, 10, cfg.Redmine.Projects[0].CustomFields.TargetRepoID)
	assert.Equal(t, 11, cfg.Redmine.Projects[0].CustomFields.GitHubIssueURLID)

	// 驗證 GitHub 配置
	assert.Equal(t, "ghp_test_token", cfg.GitHub.Token)
	assert.Equal(t, "https://github.com", cfg.GitHub.BaseURL)

	// 驗證 Sync 配置
	assert.Equal(t, "5m", cfg.Sync.Interval)
	assert.Equal(t, "[Redmine #%d] %s", cfg.Sync.TitleFormat)
	assert.True(t, cfg.Sync.OnError.Log)
	assert.False(t, cfg.Sync.OnError.AddRedmineNote)

	// 驗證 Database 配置
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "test_schema", cfg.Database.Schema)
}

func TestLoadConfigWithEnvOverride(t *testing.T) {
	// 建立基本配置檔
	configContent := `
redmine:
  url: "https://redmine.example.com"
  api_key: "test-api-key"
  projects:
    - identifier: "test-project"
      custom_fields:
        target_repo_id: 10
        github_issue_url_id: 11

github:
  token: "ghp_test_token"

sync:
  interval: "5m"

database:
  host: "localhost"
  port: 5432
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	// 設定環境變數覆蓋
	os.Setenv("POSTGRES_HOST", "db.example.com")
	os.Setenv("POSTGRES_PORT", "5433")
	os.Setenv("POSTGRES_SCHEMA", "custom_schema")
	defer func() {
		os.Unsetenv("POSTGRES_HOST")
		os.Unsetenv("POSTGRES_PORT")
		os.Unsetenv("POSTGRES_SCHEMA")
	}()

	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// 驗證環境變數覆蓋
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "custom_schema", cfg.Database.Schema)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Redmine: RedmineConfig{
					URL:    "https://redmine.example.com",
					APIKey: "test-key",
					Projects: []ProjectConfig{
						{Identifier: "test"},
					},
				},
				GitHub: GitHubConfig{
					Token: "test-token",
				},
			},
			wantError: false,
		},
		{
			name: "missing redmine url",
			config: &Config{
				Redmine: RedmineConfig{
					APIKey: "test-key",
					Projects: []ProjectConfig{
						{Identifier: "test"},
					},
				},
				GitHub: GitHubConfig{
					Token: "test-token",
				},
			},
			wantError: true,
			errorMsg:  "redmine.url is required",
		},
		{
			name: "missing redmine api key",
			config: &Config{
				Redmine: RedmineConfig{
					URL: "https://redmine.example.com",
					Projects: []ProjectConfig{
						{Identifier: "test"},
					},
				},
				GitHub: GitHubConfig{
					Token: "test-token",
				},
			},
			wantError: true,
			errorMsg:  "redmine.api_key is required",
		},
		{
			name: "missing projects",
			config: &Config{
				Redmine: RedmineConfig{
					URL:      "https://redmine.example.com",
					APIKey:   "test-key",
					Projects: []ProjectConfig{},
				},
				GitHub: GitHubConfig{
					Token: "test-token",
				},
			},
			wantError: true,
			errorMsg:  "at least one project is required",
		},
		{
			name: "missing github token",
			config: &Config{
				Redmine: RedmineConfig{
					URL:    "https://redmine.example.com",
					APIKey: "test-key",
					Projects: []ProjectConfig{
						{Identifier: "test"},
					},
				},
				GitHub: GitHubConfig{},
			},
			wantError: true,
			errorMsg:  "github.token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSyncInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "valid 5 minutes",
			interval: "5m",
			want:     5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "valid 1 hour",
			interval: "1h",
			want:     time.Hour,
			wantErr:  false,
		},
		{
			name:     "invalid format",
			interval: "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Sync: SyncConfig{
					Interval: tt.interval,
				},
			}

			got, err := cfg.GetSyncInterval()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		Redmine: RedmineConfig{
			URL:    "https://redmine.example.com",
			APIKey: "test-key",
			Projects: []ProjectConfig{
				{Identifier: "test"},
			},
		},
		GitHub: GitHubConfig{
			Token: "test-token",
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	// 驗證預設值
	assert.Equal(t, "https://github.com", cfg.GitHub.BaseURL)
	assert.Equal(t, "5m", cfg.Sync.Interval)
	assert.Equal(t, "[Redmine #%d] %s", cfg.Sync.TitleFormat)
	assert.Equal(t, "redmine_github_sync", cfg.Database.Schema)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
}
