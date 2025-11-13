package redmine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"colosscious.com/github-sync/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNewIssues(t *testing.T) {
	// Mock Redmine API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 驗證請求
		assert.Equal(t, "/issues.json", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Redmine-API-Key"))

		// 檢查query parameters
		query := r.URL.Query()
		assert.Equal(t, "test-project", query.Get("project_id"))
		assert.Equal(t, "*", query.Get("status_id"))

		// 返回 mock 資料
		response := IssuesResponse{
			Issues: []Issue{
				{
					ID:      1,
					Subject: "Test Issue 1",
					Project: Project{ID: 1, Name: "Test Project"},
					CustomFields: []CustomField{
						{ID: 10, Name: "Target Repo", Value: "owner/repo1"},
						{ID: 11, Name: "GitHub URL", Value: ""},
					},
				},
				{
					ID:      2,
					Subject: "Test Issue 2",
					Project: Project{ID: 1, Name: "Test Project"},
					CustomFields: []CustomField{
						{ID: 10, Name: "Target Repo", Value: "owner/repo2"},
						{ID: 11, Name: "GitHub URL", Value: "https://github.com/owner/repo/issues/1"},
					},
				},
			},
			TotalCount: 2,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// 建立 client
	client := &Client{
		baseURL: server.URL,
		apiKey:  "test-api-key",
		client:  &http.Client{},
	}

	// 測試 GetNewIssues
	issues, err := client.GetNewIssues("test-project", 10, 11)
	require.NoError(t, err)

	// 應該只返回還沒同步的（GitHub URL 為空的）
	assert.Len(t, issues, 1)
	assert.Equal(t, 1, issues[0].ID)
	assert.Equal(t, "Test Issue 1", issues[0].Subject)
}

func TestUpdateCustomField(t *testing.T) {
	// Mock Redmine API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 驗證請求
		assert.Equal(t, "/issues/123.json", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Redmine-API-Key"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 驗證 payload
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		issue := payload["issue"].(map[string]interface{})
		customFields := issue["custom_fields"].([]interface{})
		field := customFields[0].(map[string]interface{})

		assert.Equal(t, float64(11), field["id"])
		assert.Equal(t, "https://github.com/owner/repo/issues/1", field["value"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "test-api-key",
		client:  &http.Client{},
	}

	// 測試 UpdateCustomField
	err := client.UpdateCustomField(123, 11, "https://github.com/owner/repo/issues/1")
	assert.NoError(t, err)
}

func TestAddNote(t *testing.T) {
	// Mock Redmine API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/issues/123.json", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		issue := payload["issue"].(map[string]interface{})
		assert.Equal(t, "Test note", issue["notes"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "test-api-key",
		client:  &http.Client{},
	}

	err := client.AddNote(123, "Test note")
	assert.NoError(t, err)
}

func TestGetCustomFieldValue(t *testing.T) {
	issue := Issue{
		CustomFields: []CustomField{
			{ID: 10, Name: "Repo", Value: "owner/repo"},
			{ID: 11, Name: "URL", Value: nil},
			{ID: 12, Name: "Count", Value: float64(42)},
		},
	}

	// 測試字串值
	assert.Equal(t, "owner/repo", issue.GetCustomFieldValue(10))

	// 測試 nil 值
	assert.Equal(t, "", issue.GetCustomFieldValue(11))

	// 測試數字值
	assert.Equal(t, "42", issue.GetCustomFieldValue(12))

	// 測試不存在的 field
	assert.Equal(t, "", issue.GetCustomFieldValue(99))
}

func TestNewClient(t *testing.T) {
	cfg := config.RedmineConfig{
		URL:    "https://redmine.example.com",
		APIKey: "test-key",
	}

	client := NewClient(cfg)
	assert.NotNil(t, client)
	assert.Equal(t, "https://redmine.example.com", client.baseURL)
	assert.Equal(t, "test-key", client.apiKey)
	assert.NotNil(t, client.client)
}
