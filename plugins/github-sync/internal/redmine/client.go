package redmine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"colosscious.com/github-sync/internal/config"
)

// Client Redmine API 客戶端
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// Issue Redmine issue 結構
type Issue struct {
	ID           int           `json:"id"`
	Project      Project       `json:"project"`
	Tracker      Tracker       `json:"tracker"`
	Status       Status        `json:"status"`
	Priority     Priority      `json:"priority"`
	Author       User          `json:"author"`
	Subject      string        `json:"subject"`
	Description  string        `json:"description"`
	CustomFields []CustomField `json:"custom_fields"`
	CreatedOn    string        `json:"created_on"`
	UpdatedOn    string        `json:"updated_on"`
}

// Project 專案資訊
type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Tracker issue 類型
type Tracker struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Status issue 狀態
type Status struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Priority 優先級
type Priority struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// User 使用者
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CustomField 自訂欄位
type CustomField struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Value    interface{} `json:"value"`
	Multiple bool        `json:"multiple,omitempty"`
}

// IssuesResponse API 回應
type IssuesResponse struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
}

// NewClient 建立 Redmine 客戶端
func NewClient(cfg config.RedmineConfig) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(cfg.URL, "/"),
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetNewIssues 取得需要同步的新 issues
func (c *Client) GetNewIssues(projectID string, targetRepoFieldID, githubURLFieldID int) ([]Issue, error) {
	// 查詢條件：
	// 1. 有填 target_repo_field (cf_X != "")
	// 2. 沒有填 github_url_field (cf_Y = "")
	params := url.Values{}
	params.Add("project_id", projectID)
	params.Add("status_id", "*") // 所有狀態
	params.Add(fmt.Sprintf("cf_%d", targetRepoFieldID), "*") // 有填目標 repo
	params.Add("limit", "100")
	params.Add("sort", "created_on:desc")

	issues, err := c.getIssues(params)
	if err != nil {
		return nil, err
	}

	// 過濾出還沒同步的（GitHub URL 欄位為空）
	var newIssues []Issue
	for _, issue := range issues {
		githubURL := issue.GetCustomFieldValue(githubURLFieldID)
		if githubURL == "" {
			newIssues = append(newIssues, issue)
		}
	}

	return newIssues, nil
}

// getIssues 通用的取得 issues 方法
func (c *Client) getIssues(params url.Values) ([]Issue, error) {
	endpoint := fmt.Sprintf("%s/issues.json?%s", c.baseURL, params.Encode())

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issuesResp IssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&issuesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return issuesResp.Issues, nil
}

// UpdateCustomField 更新 issue 的 custom field
func (c *Client) UpdateCustomField(issueID, fieldID int, value string) error {
	endpoint := fmt.Sprintf("%s/issues/%d.json", c.baseURL, issueID)

	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"custom_fields": []map[string]interface{}{
				{
					"id":    fieldID,
					"value": value,
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Redmine PUT 成功會回傳 204 No Content 或 200 OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddNote 在 issue 加上註解
func (c *Client) AddNote(issueID int, note string) error {
	endpoint := fmt.Sprintf("%s/issues/%d.json", c.baseURL, issueID)

	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"notes": note,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetCustomFieldValue 取得 custom field 的值
func (i *Issue) GetCustomFieldValue(fieldID int) string {
	for _, cf := range i.CustomFields {
		if cf.ID == fieldID {
			if cf.Value == nil {
				return ""
			}
			// 處理不同類型的值
			switch v := cf.Value.(type) {
			case string:
				return v
			case float64:
				return fmt.Sprintf("%.0f", v)
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

// GetCustomFieldName 取得 custom field 的名稱（用於 debug）
func (i *Issue) GetCustomFieldName(fieldID int) string {
	for _, cf := range i.CustomFields {
		if cf.ID == fieldID {
			return cf.Name
		}
	}
	return ""
}
