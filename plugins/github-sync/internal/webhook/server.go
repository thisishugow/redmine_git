package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"colosscious.com/github-sync/internal/config"
)

// IssueChangedPayload webhook payload 結構
type IssueChangedPayload struct {
	IssueID           int    `json:"issue_id"`
	ProjectIdentifier string `json:"project_identifier"`
	TargetRepo        string `json:"target_repo"`
	Action            string `json:"action"`
	Timestamp         string `json:"timestamp"`
}

// SyncTrigger 同步觸發器（用於通知 syncer 執行同步）
type SyncTrigger interface {
	SyncSpecificIssue(issueID int, projectIdentifier string) error
}

// Server webhook HTTP server
type Server struct {
	config      *config.Config
	syncTrigger SyncTrigger
	secret      string
}

// NewServer 建立 webhook server
func NewServer(cfg *config.Config, syncTrigger SyncTrigger) *Server {
	return &Server{
		config:      cfg,
		syncTrigger: syncTrigger,
		secret:      cfg.Webhook.Secret,
	}
}

// Start 啟動 HTTP server
func (s *Server) Start(addr string) error {
	http.HandleFunc("/webhook/issue-changed", s.handleIssueChanged)
	http.HandleFunc("/health", s.handleHealth)

	log.Printf("Webhook server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleIssueChanged 處理 issue 變更 webhook
func (s *Server) handleIssueChanged(w http.ResponseWriter, r *http.Request) {
	// 只接受 POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 讀取 body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read webhook body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 驗證簽章（如果有設定 secret）
	if s.secret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if !s.verifySignature(body, signature) {
			log.Printf("Invalid webhook signature from %s", r.RemoteAddr)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// 解析 payload
	var payload IssueChangedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Failed to parse webhook payload: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received webhook: issue #%d in project %s (repo: %s, action: %s)",
		payload.IssueID, payload.ProjectIdentifier, payload.TargetRepo, payload.Action)

	// 非同步觸發同步（避免阻塞 HTTP 回應）
	go func() {
		if err := s.syncTrigger.SyncSpecificIssue(payload.IssueID, payload.ProjectIdentifier); err != nil {
			log.Printf("Failed to sync issue #%d: %v", payload.IssueID, err)
		}
	}()

	// 立即回應 200 OK
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"accepted"}`))
}

// handleHealth 健康檢查端點
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// verifySignature 驗證 HMAC-SHA256 簽章
func (s *Server) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// 移除 "sha256=" prefix
	signature = strings.TrimPrefix(signature, "sha256=")

	// 計算期望的簽章
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// 使用 constant-time 比較防止 timing attack
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ParseIssueID 從字串解析 issue ID（工具函數）
func ParseIssueID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid issue ID: %s", s)
	}
	return id, nil
}
