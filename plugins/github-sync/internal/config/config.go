package config

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config 全域配置結構
type Config struct {
	mu       sync.RWMutex
	Redmine  RedmineConfig  `mapstructure:"redmine"`
	GitHub   GitHubConfig   `mapstructure:"github"`
	Sync     SyncConfig     `mapstructure:"sync"`
	Database DatabaseConfig `mapstructure:"database"`
}

// RedmineConfig Redmine 配置
type RedmineConfig struct {
	URL      string          `mapstructure:"url"`
	APIKey   string          `mapstructure:"api_key"`
	Projects []ProjectConfig `mapstructure:"projects"`
}

// ProjectConfig 專案配置
type ProjectConfig struct {
	Identifier   string              `mapstructure:"identifier"`
	CustomFields CustomFieldsMapping `mapstructure:"custom_fields"`
}

// CustomFieldsMapping Custom Fields 對應
type CustomFieldsMapping struct {
	TargetRepoID      int `mapstructure:"target_repo_id"`
	GitHubIssueURLID  int `mapstructure:"github_issue_url_id"`
}

// GitHubConfig GitHub 配置
type GitHubConfig struct {
	Token   string `mapstructure:"token"`
	BaseURL string `mapstructure:"base_url"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	Interval    string      `mapstructure:"interval"`
	TitleFormat string      `mapstructure:"title_format"`
	OnError     ErrorConfig `mapstructure:"on_error"`
}

// ErrorConfig 錯誤處理配置
type ErrorConfig struct {
	Log             bool   `mapstructure:"log"`
	AddRedmineNote  bool   `mapstructure:"add_redmine_note"`
	LogFile         string `mapstructure:"log_file"`
}

// DatabaseConfig 資料庫配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Schema   string `mapstructure:"schema"`
	SSLMode  string `mapstructure:"sslmode"`
}

var (
	globalConfig *Config
	configMu     sync.RWMutex
	reloadChan   = make(chan struct{}, 1)
)

// LoadConfig 載入配置檔
func LoadConfig(configPath string) (*Config, error) {
	// 從環境變數或配置檔路徑
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "./config.yaml"
		}
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 自動讀取環境變數（用於覆蓋 DB 設定）
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GITHUB_SYNC")

	// 綁定環境變數到配置
	bindEnvVars()

	// 讀取配置檔
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 驗證配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	globalConfig = cfg

	// 啟動熱更新監聽
	watchConfig()

	log.Printf("Configuration loaded from %s", configPath)
	return cfg, nil
}

// bindEnvVars 綁定環境變數
func bindEnvVars() {
	// 資料庫配置可以從環境變數覆蓋
	viper.BindEnv("database.host", "POSTGRES_HOST")
	viper.BindEnv("database.port", "POSTGRES_PORT")
	viper.BindEnv("database.name", "POSTGRES_DB")
	viper.BindEnv("database.user", "POSTGRES_USER")
	viper.BindEnv("database.password", "POSTGRES_PASSWORD")
	viper.BindEnv("database.schema", "POSTGRES_SCHEMA")
	viper.BindEnv("database.sslmode", "POSTGRES_SSLMODE")
}

// watchConfig 監聽配置檔變更
func watchConfig() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s", e.Name)

		configMu.Lock()
		defer configMu.Unlock()

		newCfg := &Config{}
		if err := viper.Unmarshal(newCfg); err != nil {
			log.Printf("Error reloading config: %v", err)
			return
		}

		if err := newCfg.Validate(); err != nil {
			log.Printf("Invalid config after reload: %v", err)
			return
		}

		globalConfig.mu.Lock()
		globalConfig.Redmine = newCfg.Redmine
		globalConfig.GitHub = newCfg.GitHub
		globalConfig.Sync = newCfg.Sync
		// 注意：不更新 Database 配置，因為需要重新連線
		globalConfig.mu.Unlock()

		log.Println("Config reloaded successfully")

		// 通知配置已重新載入
		select {
		case reloadChan <- struct{}{}:
		default:
		}
	})
}

// GetConfig 取得當前配置（thread-safe）
func GetConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// GetReloadChannel 取得配置重載通知 channel
func GetReloadChannel() <-chan struct{} {
	return reloadChan
}

// Validate 驗證配置
func (c *Config) Validate() error {
	if c.Redmine.URL == "" {
		return fmt.Errorf("redmine.url is required")
	}
	if c.Redmine.APIKey == "" {
		return fmt.Errorf("redmine.api_key is required")
	}
	if len(c.Redmine.Projects) == 0 {
		return fmt.Errorf("at least one project is required")
	}

	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	if c.GitHub.BaseURL == "" {
		c.GitHub.BaseURL = "https://github.com"
	}

	if c.Sync.Interval == "" {
		c.Sync.Interval = "5m"
	}
	if c.Sync.TitleFormat == "" {
		c.Sync.TitleFormat = "[Redmine #%d] %s"
	}

	if c.Database.Schema == "" {
		c.Database.Schema = "redmine_github_sync"
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}

	return nil
}

// GetSyncInterval 取得同步間隔
func (c *Config) GetSyncInterval() (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.ParseDuration(c.Sync.Interval)
}
