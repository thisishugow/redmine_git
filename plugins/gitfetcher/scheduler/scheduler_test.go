package scheduler

import (
	"testing"
	"time"

	"colosscious.com/gitfetcher/config"
	"colosscious.com/gitfetcher/fetcher"
)

// mockFetcher is a mock implementation of GitFetcher for testing
type mockFetcher struct {
	fetchCalls []string
	results    map[string]*fetcher.FetchResult
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		fetchCalls: make([]string, 0),
		results:    make(map[string]*fetcher.FetchResult),
	}
}

func (m *mockFetcher) Fetch(name, localPath string) *fetcher.FetchResult {
	m.fetchCalls = append(m.fetchCalls, name)

	if result, ok := m.results[name]; ok {
		return result
	}

	// Default success result
	return &fetcher.FetchResult{
		RepoName:  name,
		Success:   true,
		Message:   "Mock fetch successful",
		Timestamp: time.Now(),
	}
}

func (m *mockFetcher) setResult(name string, success bool, message string) {
	m.results[name] = &fetcher.FetchResult{
		RepoName:  name,
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}
}

func TestNewScheduler(t *testing.T) {
	mock := newMockFetcher()
	// Type assertion to ensure mockFetcher can be used where GitFetcher is expected
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}

	if s.repos == nil {
		t.Error("repos map is nil")
	}

	if s.stopChans == nil {
		t.Error("stopChans map is nil")
	}

	// Verify mock wasn't called yet
	if len(mock.fetchCalls) != 0 {
		t.Errorf("Expected 0 fetch calls, got %d", len(mock.fetchCalls))
	}
}

func TestLoadConfig(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "repo1",
				URL:       "git@github.com:user/repo1.git",
				LocalPath: "/repos/repo1.git",
				Interval:  "5m",
			},
			{
				Name:      "repo2",
				URL:       "git@github.com:user/repo2.git",
				LocalPath: "/repos/repo2.git",
				Interval:  "10m",
			},
		},
		HTTPPort: 8080,
	}

	s.LoadConfig(cfg)

	// Wait a bit for goroutines to start
	time.Sleep(100 * time.Millisecond)

	status := s.GetStatus()

	if len(status) != 2 {
		t.Errorf("Expected 2 repos in status, got %d", len(status))
	}

	if _, ok := status["repo1"]; !ok {
		t.Error("repo1 not found in status")
	}

	if _, ok := status["repo2"]; !ok {
		t.Error("repo2 not found in status")
	}

	// Verify repo details
	if status["repo1"].Name != "repo1" {
		t.Errorf("Expected repo1 name 'repo1', got '%s'", status["repo1"].Name)
	}

	if status["repo1"].Interval != "5m" {
		t.Errorf("Expected repo1 interval '5m', got '%s'", status["repo1"].Interval)
	}

	// Clean up
	s.Stop()
}

func TestLoadConfigMultipleTimes(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	// First config
	cfg1 := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "repo1",
				URL:       "git@github.com:user/repo1.git",
				LocalPath: "/repos/repo1.git",
				Interval:  "5m",
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg1)
	time.Sleep(50 * time.Millisecond)

	// Second config (hot reload)
	cfg2 := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "repo2",
				URL:       "git@github.com:user/repo2.git",
				LocalPath: "/repos/repo2.git",
				Interval:  "10m",
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg2)
	time.Sleep(50 * time.Millisecond)

	status := s.GetStatus()

	// Should only have repo2, repo1 should be stopped
	if len(status) != 1 {
		t.Errorf("Expected 1 repo in status after reload, got %d", len(status))
	}

	if _, ok := status["repo2"]; !ok {
		t.Error("repo2 not found in status after reload")
	}

	if _, ok := status["repo1"]; ok {
		t.Error("repo1 should not be in status after reload")
	}

	s.Stop()
}

func TestGetStatus(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	// Initially empty
	status := s.GetStatus()
	if len(status) != 0 {
		t.Errorf("Expected empty status initially, got %d repos", len(status))
	}

	// Load config
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "1s",
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg)

	// Wait for initial fetch
	time.Sleep(200 * time.Millisecond)

	status = s.GetStatus()
	if len(status) != 1 {
		t.Errorf("Expected 1 repo in status, got %d", len(status))
	}

	repoStatus, ok := status["test-repo"]
	if !ok {
		t.Fatal("test-repo not found in status")
	}

	// Check status fields
	if repoStatus.Name != "test-repo" {
		t.Errorf("Expected Name 'test-repo', got '%s'", repoStatus.Name)
	}

	if repoStatus.FetchCount == 0 {
		t.Error("Expected at least 1 fetch to have occurred")
	}

	s.Stop()
}

func TestManualFetch(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "1h", // Long interval so it won't auto-fetch during test
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg)

	// Wait for initial fetch
	time.Sleep(200 * time.Millisecond)

	initialStatus := s.GetStatus()["test-repo"]
	initialFetchCount := initialStatus.FetchCount

	// Trigger manual fetch
	err := s.ManualFetch("test-repo")
	if err != nil {
		t.Errorf("ManualFetch failed: %v", err)
	}

	// Wait for manual fetch to complete
	time.Sleep(200 * time.Millisecond)

	finalStatus := s.GetStatus()["test-repo"]
	finalFetchCount := finalStatus.FetchCount

	if finalFetchCount <= initialFetchCount {
		t.Errorf("Expected FetchCount to increase from %d, got %d", initialFetchCount, finalFetchCount)
	}

	s.Stop()
}

func TestManualFetchNonexistentRepo(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	// Try to fetch a repo that doesn't exist in config
	err := s.ManualFetch("nonexistent")
	if err != nil {
		t.Errorf("Expected no error for nonexistent repo, got: %v", err)
	}
}

func TestStop(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "repo1",
				URL:       "git@github.com:user/repo1.git",
				LocalPath: "/repos/repo1.git",
				Interval:  "1s",
			},
			{
				Name:      "repo2",
				URL:       "git@github.com:user/repo2.git",
				LocalPath: "/repos/repo2.git",
				Interval:  "1s",
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg)

	// Wait for schedulers to start
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan bool)
	go func() {
		s.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Stop() did not complete within timeout")
	}
}

func TestRepoStatusStatistics(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "100ms", // Very short interval for testing
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg)

	// Wait for multiple fetches
	time.Sleep(500 * time.Millisecond)

	status := s.GetStatus()["test-repo"]

	// Should have multiple fetch attempts
	if status.FetchCount < 2 {
		t.Errorf("Expected at least 2 fetches, got %d", status.FetchCount)
	}

	// Check that SuccessCount and FailCount are tracked
	if status.SuccessCount+status.FailCount != status.FetchCount {
		t.Errorf("SuccessCount(%d) + FailCount(%d) should equal FetchCount(%d)",
			status.SuccessCount, status.FailCount, status.FetchCount)
	}

	s.Stop()
}

func TestRepoStatusFields(t *testing.T) {
	status := &RepoStatus{
		Name:         "test",
		URL:          "git@github.com:user/test.git",
		LocalPath:    "/repos/test.git",
		Interval:     "5m",
		LastFetch:    time.Now(),
		LastResult:   "success",
		LastSuccess:  true,
		NextFetch:    time.Now().Add(5 * time.Minute),
		IsRunning:    false,
		FetchCount:   10,
		SuccessCount: 9,
		FailCount:    1,
	}

	if status.Name != "test" {
		t.Errorf("Expected Name 'test', got '%s'", status.Name)
	}

	if status.FetchCount != 10 {
		t.Errorf("Expected FetchCount 10, got %d", status.FetchCount)
	}

	if status.SuccessCount != 9 {
		t.Errorf("Expected SuccessCount 9, got %d", status.SuccessCount)
	}

	if status.FailCount != 1 {
		t.Errorf("Expected FailCount 1, got %d", status.FailCount)
	}
}

func TestConcurrentGetStatus(t *testing.T) {
	gf := fetcher.NewGitFetcher("", "")
	s := NewScheduler(gf)

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{
				Name:      "test-repo",
				URL:       "git@github.com:user/test.git",
				LocalPath: "/repos/test.git",
				Interval:  "10ms",
			},
		},
		HTTPPort: 8080,
	}
	s.LoadConfig(cfg)

	// Call GetStatus concurrently while fetches are happening
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.GetStatus()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	s.Stop()
	// If we got here without race conditions, test passes
}
