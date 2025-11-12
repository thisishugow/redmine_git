package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"colosscious.com/github-sync/internal/config"
)

// Client GitHub API 客戶端
type Client struct {
	token   string
	baseURL string
	client  *http.Client
}

// Issue GitHub issue 結構
type Issue struct {
	Number  int      `json:"number"`
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	State   string   `json:"state"`
	HTMLURL string   `json:"html_url"`
	Labels  []string `json:"labels,omitempty"`
}

// CreateIssueRequest 建立 issue 的請求
type CreateIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	State  string   `json:"state,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// NewClient 建立 GitHub 客戶端
func NewClient(cfg config.GitHubConfig) *Client {
	return &Client{
		token:   cfg.Token,
		baseURL: "https://api.github.com",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateIssue 在指定 repo 建立 issue
func (c *Client) CreateIssue(repo string, req CreateIssueRequest) (*Issue, error) {
	// repo 格式：owner/repo (例如 mycompany/backend)
	endpoint := fmt.Sprintf("%s/repos/%s/issues", c.baseURL, repo)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &issue, nil
}

// UpdateIssue 更新 issue（用於未來擴充）
func (c *Client) UpdateIssue(repo string, issueNumber int, req CreateIssueRequest) error {
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, repo, issueNumber)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("PATCH", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CloseIssue 關閉 issue（用於未來擴充）
func (c *Client) CloseIssue(repo string, issueNumber int) error {
	return c.UpdateIssue(repo, issueNumber, CreateIssueRequest{
		State: "closed",
	})
}

// AddComment 在 issue 加上評論（用於未來擴充）
func (c *Client) AddComment(repo string, issueNumber int, comment string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.baseURL, repo, issueNumber)

	payload := map[string]string{
		"body": comment,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ValidateRepo 驗證 repo 是否存在且有權限
func (c *Client) ValidateRepo(repo string) error {
	endpoint := fmt.Sprintf("%s/repos/%s", c.baseURL, repo)

	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found or no permission: %s", repo)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetRateLimit 取得 API rate limit 資訊
func (c *Client) GetRateLimit() (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/rate_limit", c.baseURL)

	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// BuildIssueURL 建立 GitHub issue 的 URL
func BuildIssueURL(baseURL, repo string, issueNumber int) string {
	// baseURL: https://github.com
	// repo: mycompany/backend
	// issueNumber: 123
	// 結果: https://github.com/mycompany/backend/issues/123

	base := strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/%s/issues/%d", base, repo, issueNumber)
}
