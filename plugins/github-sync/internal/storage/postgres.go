package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"colosscious.com/github-sync/internal/config"
)

// PostgresDB PostgreSQL 資料庫儲存
type PostgresDB struct {
	db     *sql.DB
	schema string
}

// SyncRecord 同步記錄
type SyncRecord struct {
	ID                int
	RedmineIssueID    int
	RedmineProject    string
	GitHubRepo        string
	GitHubIssueNumber int
	GitHubIssueURL    string
	SyncedAt          time.Time
}

// SyncError 同步錯誤記錄
type SyncError struct {
	ID             int
	RedmineIssueID int
	ErrorMessage   string
	OccurredAt     time.Time
	Resolved       bool
}

// NewPostgresDB 建立 PostgreSQL 連線
func NewPostgresDB(cfg config.DatabaseConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 測試連線
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 設定連線池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	p := &PostgresDB{
		db:     db,
		schema: cfg.Schema,
	}

	// 初始化 schema 和 tables
	if err := p.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return p, nil
}

// initSchema 初始化資料庫 schema
func (p *PostgresDB) initSchema() error {
	// 建立獨立 schema
	createSchemaSQL := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", p.schema)
	if _, err := p.db.Exec(createSchemaSQL); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// 建立 tables
	if err := p.migrate(); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}

// migrate 執行資料庫 migration
func (p *PostgresDB) migrate() error {
	migrations := []string{
		// sync_records table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.sync_records (
				id SERIAL PRIMARY KEY,
				redmine_issue_id INTEGER NOT NULL UNIQUE,
				redmine_project TEXT NOT NULL,
				github_repo TEXT NOT NULL,
				github_issue_number INTEGER NOT NULL,
				github_issue_url TEXT NOT NULL,
				synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`, p.schema),

		// sync_errors table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.sync_errors (
				id SERIAL PRIMARY KEY,
				redmine_issue_id INTEGER NOT NULL,
				error_message TEXT NOT NULL,
				occurred_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				resolved BOOLEAN DEFAULT FALSE
			)
		`, p.schema),

		// indexes
		fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_redmine_issue
			ON %s.sync_records(redmine_issue_id)
		`, p.schema),

		fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_github_repo
			ON %s.sync_records(github_repo)
		`, p.schema),

		fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_unresolved_errors
			ON %s.sync_errors(redmine_issue_id, resolved)
			WHERE resolved = FALSE
		`, p.schema),
	}

	for _, migration := range migrations {
		if _, err := p.db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	return nil
}

// IsSynced 檢查 issue 是否已同步
func (p *PostgresDB) IsSynced(redmineIssueID int) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.sync_records
			WHERE redmine_issue_id = $1
		)
	`, p.schema)

	var exists bool
	err := p.db.QueryRow(query, redmineIssueID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check sync status: %w", err)
	}

	return exists, nil
}

// RecordSync 記錄同步結果
func (p *PostgresDB) RecordSync(record SyncRecord) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.sync_records
		(redmine_issue_id, redmine_project, github_repo, github_issue_number, github_issue_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (redmine_issue_id) DO UPDATE SET
			github_repo = EXCLUDED.github_repo,
			github_issue_number = EXCLUDED.github_issue_number,
			github_issue_url = EXCLUDED.github_issue_url,
			synced_at = CURRENT_TIMESTAMP
	`, p.schema)

	_, err := p.db.Exec(query,
		record.RedmineIssueID,
		record.RedmineProject,
		record.GitHubRepo,
		record.GitHubIssueNumber,
		record.GitHubIssueURL,
	)

	if err != nil {
		return fmt.Errorf("failed to record sync: %w", err)
	}

	return nil
}

// RecordError 記錄同步錯誤
func (p *PostgresDB) RecordError(redmineIssueID int, errorMsg string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.sync_errors
		(redmine_issue_id, error_message)
		VALUES ($1, $2)
	`, p.schema)

	_, err := p.db.Exec(query, redmineIssueID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to record error: %w", err)
	}

	return nil
}

// GetSyncRecord 取得同步記錄
func (p *PostgresDB) GetSyncRecord(redmineIssueID int) (*SyncRecord, error) {
	query := fmt.Sprintf(`
		SELECT id, redmine_issue_id, redmine_project, github_repo,
		       github_issue_number, github_issue_url, synced_at
		FROM %s.sync_records
		WHERE redmine_issue_id = $1
	`, p.schema)

	record := &SyncRecord{}
	err := p.db.QueryRow(query, redmineIssueID).Scan(
		&record.ID,
		&record.RedmineIssueID,
		&record.RedmineProject,
		&record.GitHubRepo,
		&record.GitHubIssueNumber,
		&record.GitHubIssueURL,
		&record.SyncedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sync record: %w", err)
	}

	return record, nil
}

// GetUnresolvedErrors 取得未解決的錯誤
func (p *PostgresDB) GetUnresolvedErrors() ([]SyncError, error) {
	query := fmt.Sprintf(`
		SELECT id, redmine_issue_id, error_message, occurred_at, resolved
		FROM %s.sync_errors
		WHERE resolved = FALSE
		ORDER BY occurred_at DESC
	`, p.schema)

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query errors: %w", err)
	}
	defer rows.Close()

	var errors []SyncError
	for rows.Next() {
		var e SyncError
		if err := rows.Scan(&e.ID, &e.RedmineIssueID, &e.ErrorMessage, &e.OccurredAt, &e.Resolved); err != nil {
			return nil, fmt.Errorf("failed to scan error: %w", err)
		}
		errors = append(errors, e)
	}

	return errors, nil
}

// ResolveError 標記錯誤為已解決
func (p *PostgresDB) ResolveError(errorID int) error {
	query := fmt.Sprintf(`
		UPDATE %s.sync_errors
		SET resolved = TRUE
		WHERE id = $1
	`, p.schema)

	_, err := p.db.Exec(query, errorID)
	if err != nil {
		return fmt.Errorf("failed to resolve error: %w", err)
	}

	return nil
}

// GetStats 取得統計資訊
func (p *PostgresDB) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	// 總同步數
	var totalSynced int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.sync_records", p.schema)
	if err := p.db.QueryRow(query).Scan(&totalSynced); err != nil {
		return nil, err
	}
	stats["total_synced"] = totalSynced

	// 未解決錯誤數
	var unresolvedErrors int
	query = fmt.Sprintf("SELECT COUNT(*) FROM %s.sync_errors WHERE resolved = FALSE", p.schema)
	if err := p.db.QueryRow(query).Scan(&unresolvedErrors); err != nil {
		return nil, err
	}
	stats["unresolved_errors"] = unresolvedErrors

	// 今日同步數
	var todaySynced int
	query = fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.sync_records
		WHERE synced_at >= CURRENT_DATE
	`, p.schema)
	if err := p.db.QueryRow(query).Scan(&todaySynced); err != nil {
		return nil, err
	}
	stats["today_synced"] = todaySynced

	return stats, nil
}

// Close 關閉資料庫連線
func (p *PostgresDB) Close() error {
	return p.db.Close()
}
