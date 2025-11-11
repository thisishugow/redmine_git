package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"colosscious.com/gitfetcher/config"
	"colosscious.com/gitfetcher/fetcher"
	"colosscious.com/gitfetcher/scheduler"
	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *scheduler.Scheduler, string) {
	gin.SetMode(gin.TestMode)

	gf := fetcher.NewGitFetcher("", "")
	sched := scheduler.NewScheduler(gf)

	// Create temporary config file
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	router := gin.New()
	handler := NewHandler(sched, configPath)
	handler.SetupRoutes(router)

	return router, sched, configPath
}

func TestNewHandler(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	sched := scheduler.NewScheduler(gf)
	handler := NewHandler(sched, "/tmp/test.yaml")

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.scheduler == nil {
		t.Error("handler.scheduler is nil")
	}
}

func TestHandleIndex(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}

	// Check for expected HTML content
	if !contains(body, "GitFetcher") {
		t.Error("Expected 'GitFetcher' in response body")
	}

	if !contains(body, "<!DOCTYPE html>") {
		t.Error("Expected HTML doctype in response")
	}
}

func TestHandleStatus(t *testing.T) {
	router, sched, _ := setupTestRouter()

	// Load test config
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "5m",
			},
		},
		HTTPPort: 8080,
	}
	sched.LoadConfig(cfg)

	// Wait for scheduler to initialize
	time.Sleep(100 * time.Millisecond)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got '%s'", contentType)
	}

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	repos, ok := response["repos"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'repos' field in response")
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repo in status, got %d", len(repos))
	}

	testRepo, ok := repos["test-repo"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'test-repo' in repos")
	}

	name, ok := testRepo["Name"].(string)
	if !ok || name != "test-repo" {
		t.Errorf("Expected repo Name 'test-repo', got '%v'", name)
	}

	sched.Stop()
}

func TestHandleStatusEmpty(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	repos, ok := response["repos"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'repos' field in response")
	}

	if len(repos) != 0 {
		t.Errorf("Expected 0 repos in empty status, got %d", len(repos))
	}
}

func TestHandleManualFetch(t *testing.T) {
	router, sched, _ := setupTestRouter()

	// Load test config
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "1h",
			},
		},
		HTTPPort: 8080,
	}
	sched.LoadConfig(cfg)
	time.Sleep(100 * time.Millisecond)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/fetch/test-repo", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	success, ok := response["success"].(bool)
	if !ok || !success {
		t.Error("Expected success=true in response")
	}

	message, ok := response["message"].(string)
	if !ok || message == "" {
		t.Error("Expected non-empty message in response")
	}

	sched.Stop()
}

func TestHandleManualFetchNonexistent(t *testing.T) {
	router, sched, _ := setupTestRouter()

	// Don't load any config, so repo won't exist

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/fetch/nonexistent-repo", nil)
	router.ServeHTTP(w, req)

	// Should still return 200 (ManualFetch doesn't return error for nonexistent repo)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	sched.Stop()
}

func TestHandleManualFetchEmptyName(t *testing.T) {
	router, sched, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/fetch/", nil)
	router.ServeHTTP(w, req)

	// Should return 404 because the route won't match
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for empty name, got %d", w.Code)
	}

	sched.Stop()
}

func TestSetupRoutes(t *testing.T) {
	router, sched, _ := setupTestRouter()

	// Test that all expected routes are registered
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/api/status"},
		{"POST", "/api/fetch/:name"},
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(route.method, route.path, nil)

		// For parameterized routes, use a concrete value
		if route.path == "/api/fetch/:name" {
			req, _ = http.NewRequest(route.method, "/api/fetch/test", nil)
		}

		router.ServeHTTP(w, req)

		// Should not return 404 (route exists)
		if w.Code == http.StatusNotFound {
			t.Errorf("Route %s %s not found", route.method, route.path)
		}
	}

	sched.Stop()
}

func TestHandlerConcurrentRequests(t *testing.T) {
	router, sched, _ := setupTestRouter()

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "1h",
			},
		},
		HTTPPort: 8080,
	}
	sched.LoadConfig(cfg)
	time.Sleep(100 * time.Millisecond)

	// Make concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/status", nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			done <- true
		}()
	}

	// Wait for all requests
	for i := 0; i < 10; i++ {
		<-done
	}

	sched.Stop()
}

func TestHandleStatusResponseFormat(t *testing.T) {
	router, sched, _ := setupTestRouter()

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "5m",
			},
		},
		HTTPPort: 8080,
	}
	sched.LoadConfig(cfg)
	time.Sleep(200 * time.Millisecond)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/status", nil)
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	repos := response["repos"].(map[string]interface{})
	testRepo := repos["test-repo"].(map[string]interface{})

	// Verify expected fields exist
	expectedFields := []string{"Name", "URL", "LocalPath", "Interval", "LastFetch", "NextFetch", "FetchCount"}
	for _, field := range expectedFields {
		if _, ok := testRepo[field]; !ok {
			t.Errorf("Expected field '%s' in repo status", field)
		}
	}

	sched.Stop()
}

func TestIndexHTMLEmbedded(t *testing.T) {
	// Test that the embedded HTML is not empty
	if indexHTML == "" {
		t.Error("indexHTML is empty, embedding failed")
	}

	// Check for key HTML elements
	if !contains(indexHTML, "<html") {
		t.Error("indexHTML missing <html tag")
	}

	if !contains(indexHTML, "GitFetcher") {
		t.Error("indexHTML missing GitFetcher title")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleGetConfig(t *testing.T) {
	router, _, configPath := setupTestRouter()

	// Create a test config file
	testConfig := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "5m",
			},
		},
		SSHKeyPath: "/root/.ssh/id_rsa",
		HTTPPort:   8080,
		LogPath:    "./logs",
	}

	if err := config.SaveConfig(configPath, testConfig); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	success, ok := response["success"].(bool)
	if !ok || !success {
		t.Error("Expected success=true in response")
	}

	configData, ok := response["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected config object in response")
	}

	if httpPort, ok := configData["http_port"].(float64); ok {
		if httpPort != 8080 {
			t.Errorf("Expected http_port 8080, got %v", httpPort)
		}
	} else {
		t.Errorf("http_port field missing or wrong type: %v", configData["http_port"])
	}
}

func TestHandleGetConfigFileNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gf := fetcher.NewGitFetcher("", "")
	sched := scheduler.NewScheduler(gf)

	// Use a non-existent config path
	nonExistentPath := "/nonexistent/path/config.yaml"

	router := gin.New()
	handler := NewHandler(sched, nonExistentPath)
	handler.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for missing config, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	success, _ := response["success"].(bool)
	if success {
		t.Error("Expected success=false for missing config")
	}
}

func TestHandleUpdateConfig(t *testing.T) {
	router, _, configPath := setupTestRouter()

	newConfig := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "new-repo",
				URL:       "git@github.com:user/new.git",
				LocalPath: "/repos/new.git",
				Interval:  "10m",
			},
		},
		SSHKeyPath: "/root/.ssh/id_rsa",
		HTTPPort:   9090,
		LogPath:    "./new-logs",
	}

	jsonData, err := json.Marshal(newConfig)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/config", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	success, ok := response["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success=true, got %v. Error: %v", success, response["error"])
	}

	// Verify config was saved
	savedConfig, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if len(savedConfig.Repos) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(savedConfig.Repos))
	}

	if savedConfig.Repos[0].Name != "new-repo" {
		t.Errorf("Expected repo name 'new-repo', got '%s'", savedConfig.Repos[0].Name)
	}

	if savedConfig.HTTPPort != 9090 {
		t.Errorf("Expected HTTPPort 9090, got %d", savedConfig.HTTPPort)
	}
}

func TestHandleUpdateConfigInvalidJSON(t *testing.T) {
	router, _, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/config", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	success, _ := response["success"].(bool)
	if success {
		t.Error("Expected success=false for invalid JSON")
	}
}

func TestHandleUpdateConfigInvalidConfig(t *testing.T) {
	router, _, _ := setupTestRouter()

	// Config with no repos (invalid)
	invalidConfig := &config.Config{
		Repos:    []config.RepoConfig{},
		HTTPPort: 8080,
	}

	jsonData, _ := json.Marshal(invalidConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/config", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid config, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	success, _ := response["success"].(bool)
	if success {
		t.Error("Expected success=false for invalid config")
	}

	errorMsg, ok := response["error"].(string)
	if !ok || errorMsg == "" {
		t.Error("Expected error message in response")
	}
}
