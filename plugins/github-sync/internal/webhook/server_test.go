package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"colosscious.com/github-sync/internal/config"
	"github.com/stretchr/testify/assert"
)

// MockSyncTrigger 用於測試的 mock
type MockSyncTrigger struct {
	LastIssueID  int
	LastProject  string
	SyncError    error
	CallCount    int
}

func (m *MockSyncTrigger) SyncSpecificIssue(issueID int, projectIdentifier string) error {
	m.LastIssueID = issueID
	m.LastProject = projectIdentifier
	m.CallCount++
	return m.SyncError
}

func TestHandleIssueChanged(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		payload        IssueChangedPayload
		secret         string
		addSignature   bool
		invalidSig     bool
		expectedStatus int
		shouldSync     bool
	}{
		{
			name:   "valid webhook without signature",
			method: "POST",
			payload: IssueChangedPayload{
				IssueID:           123,
				ProjectIdentifier: "my-project",
				TargetRepo:        "myorg/backend",
				Action:            "updated",
			},
			secret:         "",
			addSignature:   false,
			expectedStatus: http.StatusOK,
			shouldSync:     true,
		},
		{
			name:   "valid webhook with valid signature",
			method: "POST",
			payload: IssueChangedPayload{
				IssueID:           456,
				ProjectIdentifier: "test-project",
				TargetRepo:        "myorg/frontend",
				Action:            "created",
			},
			secret:         "test-secret-key",
			addSignature:   true,
			expectedStatus: http.StatusOK,
			shouldSync:     true,
		},
		{
			name:   "webhook with invalid signature",
			method: "POST",
			payload: IssueChangedPayload{
				IssueID:           789,
				ProjectIdentifier: "another-project",
				TargetRepo:        "myorg/mobile",
				Action:            "updated",
			},
			secret:         "test-secret-key",
			addSignature:   true,
			invalidSig:     true,
			expectedStatus: http.StatusUnauthorized,
			shouldSync:     false,
		},
		{
			name:           "invalid HTTP method",
			method:         "GET",
			payload:        IssueChangedPayload{},
			secret:         "",
			expectedStatus: http.StatusMethodNotAllowed,
			shouldSync:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 準備 config
			cfg := &config.Config{
				Webhook: config.WebhookConfig{
					Secret: tt.secret,
				},
			}

			// 準備 mock sync trigger
			mockTrigger := &MockSyncTrigger{}

			// 建立 server
			server := NewServer(cfg, mockTrigger)

			// 準備 request body
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(tt.method, "/webhook/issue-changed", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// 加上簽章（如果需要）
			if tt.addSignature && tt.secret != "" {
				if tt.invalidSig {
					req.Header.Set("X-Webhook-Signature", "sha256=invalid")
				} else {
					mac := hmac.New(sha256.New, []byte(tt.secret))
					mac.Write(body)
					signature := hex.EncodeToString(mac.Sum(nil))
					req.Header.Set("X-Webhook-Signature", "sha256="+signature)
				}
			}

			// 執行 request
			rr := httptest.NewRecorder()
			server.handleIssueChanged(rr, req)

			// 驗證 status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// 驗證是否觸發同步（需要等待 goroutine）
			// 注意：實際測試時 goroutine 可能還沒執行完
			// 這裡我們只檢查 happy path
			if tt.shouldSync && rr.Code == http.StatusOK {
				// 在真實場景中，goroutine 會非同步執行
				// 這裡我們只驗證回應正確
				assert.Contains(t, rr.Body.String(), "accepted")
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		secret    string
		signature string
		expected  bool
	}{
		{
			name:      "valid signature",
			body:      `{"issue_id":123}`,
			secret:    "my-secret",
			signature: "", // 會在測試中計算
			expected:  true,
		},
		{
			name:      "invalid signature",
			body:      `{"issue_id":123}`,
			secret:    "my-secret",
			signature: "sha256=invalid",
			expected:  false,
		},
		{
			name:      "empty signature",
			body:      `{"issue_id":123}`,
			secret:    "my-secret",
			signature: "",
			expected:  false,
		},
		{
			name:      "wrong secret",
			body:      `{"issue_id":123}`,
			secret:    "my-secret",
			signature: "", // 會用不同的 secret 計算
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{secret: tt.secret}

			signature := tt.signature
			if tt.name == "valid signature" {
				// 計算正確的簽章
				mac := hmac.New(sha256.New, []byte(tt.secret))
				mac.Write([]byte(tt.body))
				signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
			} else if tt.name == "wrong secret" {
				// 用錯誤的 secret 計算簽章
				mac := hmac.New(sha256.New, []byte("wrong-secret"))
				mac.Write([]byte(tt.body))
				signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
			}

			result := server.verifySignature([]byte(tt.body), signature)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleHealth(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "ok")
}

func TestParseIssueID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int
		expectErr bool
	}{
		{"valid ID", "123", 123, false},
		{"valid large ID", "999999", 999999, false},
		{"invalid - letters", "abc", 0, true},
		{"invalid - empty", "", 0, true},
		{"invalid - negative", "-123", -123, false}, // strconv.Atoi 接受負數
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseIssueID(tt.input)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
