package sync

import (
	"fmt"
	"log"
	"strings"

	"colosscious.com/github-sync/internal/config"
	"colosscious.com/github-sync/internal/github"
	"colosscious.com/github-sync/internal/redmine"
	"colosscious.com/github-sync/internal/storage"
)

// Syncer 同步器
type Syncer struct {
	config  *config.Config
	redmine *redmine.Client
	github  *github.Client
	storage *storage.PostgresDB
}

// NewSyncer 建立同步器
func NewSyncer(cfg *config.Config, db *storage.PostgresDB) *Syncer {
	return &Syncer{
		config:  cfg,
		redmine: redmine.NewClient(cfg.Redmine),
		github:  github.NewClient(cfg.GitHub),
		storage: db,
	}
}

// Run 執行一次同步
func (s *Syncer) Run() error {
	log.Println("Starting sync run...")

	totalSynced := 0
	totalErrors := 0

	// 遍歷所有配置的專案
	for _, project := range s.config.Redmine.Projects {
		synced, errors := s.syncProject(project)
		totalSynced += synced
		totalErrors += errors
	}

	log.Printf("Sync completed: %d issues synced, %d errors", totalSynced, totalErrors)

	// 印出統計資訊
	stats, err := s.storage.GetStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		log.Printf("Stats: Total synced=%d, Today=%d, Unresolved errors=%d",
			stats["total_synced"], stats["today_synced"], stats["unresolved_errors"])
	}

	return nil
}

// syncProject 同步單一專案
func (s *Syncer) syncProject(project config.ProjectConfig) (int, int) {
	log.Printf("Syncing project: %s", project.Identifier)

	// 取得需要同步的 issues
	issues, err := s.redmine.GetNewIssues(
		project.Identifier,
		project.CustomFields.TargetRepoID,
		project.CustomFields.GitHubIssueURLID,
	)

	if err != nil {
		log.Printf("Failed to get issues for project %s: %v", project.Identifier, err)
		return 0, 1
	}

	if len(issues) == 0 {
		log.Printf("No new issues to sync for project %s", project.Identifier)
		return 0, 0
	}

	log.Printf("Found %d issues to sync", len(issues))

	synced := 0
	errors := 0

	for _, issue := range issues {
		if err := s.syncIssue(issue, project); err != nil {
			log.Printf("Failed to sync issue #%d: %v", issue.ID, err)
			errors++
		} else {
			synced++
		}
	}

	return synced, errors
}

// syncIssue 同步單一 issue
func (s *Syncer) syncIssue(issue redmine.Issue, project config.ProjectConfig) error {
	// 1. 檢查是否已同步（double check）
	isSynced, err := s.storage.IsSynced(issue.ID)
	if err != nil {
		return fmt.Errorf("failed to check sync status: %w", err)
	}
	if isSynced {
		log.Printf("Issue #%d already synced, skipping", issue.ID)
		return nil
	}

	// 2. 取得目標 GitHub repo
	targetRepo := issue.GetCustomFieldValue(project.CustomFields.TargetRepoID)
	if targetRepo == "" {
		log.Printf("Issue #%d has no target repo, skipping", issue.ID)
		return nil
	}

	// 驗證 repo 格式（必須是 owner/repo）
	if !strings.Contains(targetRepo, "/") {
		errMsg := fmt.Sprintf("Invalid repo format '%s', expected 'owner/repo'", targetRepo)
		log.Printf("Issue #%d: %s", issue.ID, errMsg)
		s.handleError(issue.ID, errMsg)
		return fmt.Errorf("invalid repo format: %s", targetRepo)
	}

	log.Printf("Syncing issue #%d to GitHub repo: %s", issue.ID, targetRepo)

	// 3. 建立 GitHub issue title
	title := fmt.Sprintf(s.config.Sync.TitleFormat, issue.ID, issue.Subject)

	// 4. 準備 GitHub issue body
	body := s.buildGitHubIssueBody(issue)

	// 5. 建立 GitHub issue
	ghIssue, err := s.github.CreateIssue(targetRepo, github.CreateIssueRequest{
		Title:  title,
		Body:   body,
		Labels: s.mapLabels(issue),
	})

	if err != nil {
		// 記錄錯誤
		s.handleError(issue.ID, fmt.Sprintf("Failed to create GitHub issue: %v", err))
		return fmt.Errorf("failed to create GitHub issue: %w", err)
	}

	log.Printf("Created GitHub issue: %s", ghIssue.HTMLURL)

	// 6. 回寫 GitHub URL 到 Redmine
	if err := s.redmine.UpdateCustomField(
		issue.ID,
		project.CustomFields.GitHubIssueURLID,
		ghIssue.HTMLURL,
	); err != nil {
		// GitHub issue 已建立，但更新 Redmine 失敗
		// 仍然記錄到 DB，避免重複建立
		log.Printf("Warning: Failed to update Redmine custom field: %v", err)
	}

	// 7. 記錄到資料庫
	if err := s.storage.RecordSync(storage.SyncRecord{
		RedmineIssueID:    issue.ID,
		RedmineProject:    project.Identifier,
		GitHubRepo:        targetRepo,
		GitHubIssueNumber: ghIssue.Number,
		GitHubIssueURL:    ghIssue.HTMLURL,
	}); err != nil {
		return fmt.Errorf("failed to record sync: %w", err)
	}

	log.Printf("✓ Successfully synced Redmine #%d -> GitHub %s#%d",
		issue.ID, targetRepo, ghIssue.Number)

	return nil
}

// buildGitHubIssueBody 建立 GitHub issue 的 body
func (s *Syncer) buildGitHubIssueBody(issue redmine.Issue) string {
	body := fmt.Sprintf("**From Redmine Issue #%d**\n\n", issue.ID)
	body += fmt.Sprintf("**Project**: %s\n", issue.Project.Name)
	body += fmt.Sprintf("**Tracker**: %s\n", issue.Tracker.Name)
	body += fmt.Sprintf("**Priority**: %s\n", issue.Priority.Name)
	body += fmt.Sprintf("**Author**: %s\n", issue.Author.Name)
	body += fmt.Sprintf("**Created**: %s\n\n", issue.CreatedOn)
	body += "---\n\n"

	if issue.Description != "" {
		body += issue.Description
	} else {
		body += "*No description*"
	}

	body += fmt.Sprintf("\n\n---\n*Synced from Redmine: %s/issues/%d*",
		s.config.Redmine.URL, issue.ID)

	return body
}

// mapLabels 將 Redmine 的 tracker/priority 對應到 GitHub labels
func (s *Syncer) mapLabels(issue redmine.Issue) []string {
	var labels []string

	// 可以根據需求對應，這裡提供基本範例
	// 未來可以在 config 加入 label mapping

	// Tracker 對應
	switch issue.Tracker.Name {
	case "Bug":
		labels = append(labels, "bug")
	case "Feature":
		labels = append(labels, "enhancement")
	case "Support":
		labels = append(labels, "question")
	}

	// Priority 對應
	switch issue.Priority.Name {
	case "Urgent", "Immediate":
		labels = append(labels, "priority:high")
	case "High":
		labels = append(labels, "priority:medium")
	}

	// 加上來源標籤
	labels = append(labels, "from-redmine")

	return labels
}

// handleError 處理同步錯誤
func (s *Syncer) handleError(issueID int, errorMsg string) {
	// 1. 記錄到 log
	if s.config.Sync.OnError.Log {
		log.Printf("Error syncing issue #%d: %s", issueID, errorMsg)
	}

	// 2. 記錄到資料庫
	if err := s.storage.RecordError(issueID, errorMsg); err != nil {
		log.Printf("Failed to record error to DB: %v", err)
	}

	// 3. 在 Redmine 加註解
	if s.config.Sync.OnError.AddRedmineNote {
		note := fmt.Sprintf("⚠️ GitHub 同步失敗\n\n錯誤訊息：%s", errorMsg)
		if err := s.redmine.AddNote(issueID, note); err != nil {
			log.Printf("Failed to add Redmine note: %v", err)
		}
	}
}

// UpdateConfig 更新配置（用於熱更新）
func (s *Syncer) UpdateConfig(cfg *config.Config) {
	s.config = cfg
	s.redmine = redmine.NewClient(cfg.Redmine)
	s.github = github.NewClient(cfg.GitHub)
	log.Println("Syncer config updated")
}
