package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"colosscious.com/github-sync/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIssue(t *testing.T) {
	// Mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 驗證請求
		assert.Equal(t, "/repos/owner/repo/issues", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 驗證 payload
		var req CreateIssueRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "Test Issue", req.Title)
		assert.Equal(t, "Test body", req.Body)
		assert.Equal(t, []string{"bug", "from-redmine"}, req.Labels)

		// 返回 mock issue
		response := Issue{
			Number:  123,
			Title:   req.Title,
			HTMLURL: "https://github.com/owner/repo/issues/123",
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		token:   "test-token",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	// 測試 CreateIssue
	req := CreateIssueRequest{
		Title:  "Test Issue",
		Body:   "Test body",
		Labels: []string{"bug", "from-redmine"},
	}

	issue, err := client.CreateIssue("owner/repo", req)
	require.NoError(t, err)
	assert.NotNil(t, issue)
	assert.Equal(t, 123, issue.Number)
	assert.Equal(t, "Test Issue", issue.Title)
	assert.Equal(t, "https://github.com/owner/repo/issues/123", issue.HTMLURL)
}

func TestCreateIssueError(t *testing.T) {
	// Mock server 返回錯誤
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	client := &Client{
		token:   "invalid-token",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	req := CreateIssueRequest{
		Title: "Test",
	}

	_, err := client.CreateIssue("owner/repo", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestUpdateIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo/issues/123", r.URL.Path)
		assert.Equal(t, "PATCH", r.Method)

		var req CreateIssueRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "Updated title", req.Title)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Issue{Number: 123})
	}))
	defer server.Close()

	client := &Client{
		token:   "test-token",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	req := CreateIssueRequest{
		Title: "Updated title",
	}

	err := client.UpdateIssue("owner/repo", 123, req)
	assert.NoError(t, err)
}

func TestCloseIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateIssueRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "closed", req.State)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Issue{})
	}))
	defer server.Close()

	client := &Client{
		token:   "test-token",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	err := client.CloseIssue("owner/repo", 123)
	assert.NoError(t, err)
}

func TestValidateRepo(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
		errorMsg   string
	}{
		{
			name:       "valid repo",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantError:  true,
			errorMsg:   "repository not found or no permission",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantError:  true,
			errorMsg:   "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/repos/owner/repo", r.URL.Path)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := &Client{
				token:   "test-token",
				baseURL: server.URL,
				client:  &http.Client{},
			}

			err := client.ValidateRepo("owner/repo")
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildIssueURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		repo        string
		issueNumber int
		want        string
	}{
		{
			name:        "standard github",
			baseURL:     "https://github.com",
			repo:        "owner/repo",
			issueNumber: 123,
			want:        "https://github.com/owner/repo/issues/123",
		},
		{
			name:        "base url with trailing slash",
			baseURL:     "https://github.com/",
			repo:        "owner/repo",
			issueNumber: 456,
			want:        "https://github.com/owner/repo/issues/456",
		},
		{
			name:        "github enterprise",
			baseURL:     "https://github.company.com",
			repo:        "org/project",
			issueNumber: 789,
			want:        "https://github.company.com/org/project/issues/789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildIssueURL(tt.baseURL, tt.repo, tt.issueNumber)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewClient(t *testing.T) {
	cfg := config.GitHubConfig{
		Token:   "test-token",
		BaseURL: "https://github.com",
	}

	client := NewClient(cfg)
	assert.NotNil(t, client)
	assert.Equal(t, "test-token", client.token)
	assert.Equal(t, "https://api.github.com", client.baseURL)
	assert.NotNil(t, client.client)
}
