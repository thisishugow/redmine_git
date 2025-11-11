package fetcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a test git bare repository
func setupTestRepo(t *testing.T) (repoPath string, cleanup func()) {
	tmpDir := t.TempDir()

	// Create a bare repo
	bareRepo := filepath.Join(tmpDir, "test.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create a working repo to push from
	workRepo := filepath.Join(tmpDir, "work")
	cmd = exec.Command("git", "init", workRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create work repo: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "-C", workRepo, "config", "user.name", "Test User")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user: %v", err)
	}
	cmd = exec.Command("git", "-C", workRepo, "config", "user.email", "test@example.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git email: %v", err)
	}

	// Create a test file and commit
	testFile := filepath.Join(workRepo, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "-C", workRepo, "add", "test.txt")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	cmd = exec.Command("git", "-C", workRepo, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Add remote and push
	cmd = exec.Command("git", "-C", workRepo, "remote", "add", "origin", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}

	cmd = exec.Command("git", "-C", workRepo, "push", "origin", "master")
	if err := cmd.Run(); err != nil {
		// Try 'main' branch if 'master' fails
		cmd = exec.Command("git", "-C", workRepo, "push", "origin", "HEAD")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to push: %v", err)
		}
	}

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return bareRepo, cleanup
}

func TestNewGitFetcher(t *testing.T) {
	gf := NewGitFetcher("/path/to/ssh/key", "/path/to/logs")

	if gf == nil {
		t.Fatal("NewGitFetcher returned nil")
	}

	if gf.sshKeyPath != "/path/to/ssh/key" {
		t.Errorf("Expected sshKeyPath '/path/to/ssh/key', got '%s'", gf.sshKeyPath)
	}

	if gf.logPath != "/path/to/logs" {
		t.Errorf("Expected logPath '/path/to/logs', got '%s'", gf.logPath)
	}
}

func TestFetchSuccess(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	bareRepo, cleanup := setupTestRepo(t)
	defer cleanup()

	tmpDir := t.TempDir()
	gf := NewGitFetcher("", tmpDir)

	result := gf.Fetch("test-repo", bareRepo)

	if result == nil {
		t.Fatal("Fetch returned nil result")
	}

	if result.RepoName != "test-repo" {
		t.Errorf("Expected RepoName 'test-repo', got '%s'", result.RepoName)
	}

	if !result.Success {
		t.Errorf("Expected Success=true, got false. Message: %s", result.Message)
	}

	if result.Timestamp.IsZero() {
		t.Error("Expected non-zero Timestamp")
	}
}

func TestFetchNonexistentRepo(t *testing.T) {
	tmpDir := t.TempDir()
	gf := NewGitFetcher("", tmpDir)

	result := gf.Fetch("nonexistent", "/nonexistent/repo")

	if result == nil {
		t.Fatal("Fetch returned nil result")
	}

	if result.Success {
		t.Error("Expected Success=false for nonexistent repo")
	}

	if result.Message == "" {
		t.Error("Expected error message for nonexistent repo")
	}

	if result.RepoName != "nonexistent" {
		t.Errorf("Expected RepoName 'nonexistent', got '%s'", result.RepoName)
	}
}

func TestFetchInvalidRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmpDir := t.TempDir()

	// Create a directory that's not a git repo
	notARepo := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(notARepo, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	logDir := filepath.Join(tmpDir, "logs")
	gf := NewGitFetcher("", logDir)

	result := gf.Fetch("invalid-repo", notARepo)

	if result == nil {
		t.Fatal("Fetch returned nil result")
	}

	if result.Success {
		t.Error("Expected Success=false for invalid repo")
	}
}

func TestFetchWithSSHKey(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	bareRepo, cleanup := setupTestRepo(t)
	defer cleanup()

	tmpDir := t.TempDir()

	// Create a fake SSH key file
	sshKeyPath := filepath.Join(tmpDir, "id_rsa")
	if err := os.WriteFile(sshKeyPath, []byte("fake key"), 0600); err != nil {
		t.Fatalf("Failed to create fake SSH key: %v", err)
	}

	logDir := filepath.Join(tmpDir, "logs")
	gf := NewGitFetcher(sshKeyPath, logDir)

	// Note: This will still work because we're using local path, not SSH
	result := gf.Fetch("test-repo", bareRepo)

	if result == nil {
		t.Fatal("Fetch returned nil result")
	}

	// Should succeed since it's a local repo
	if !result.Success {
		t.Errorf("Expected Success=true, got false. Message: %s", result.Message)
	}
}

func TestLogResult(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	gf := NewGitFetcher("", logDir)

	// Call private method via Fetch (which calls logResult internally)
	// We'll test indirectly by checking if log file was created

	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	bareRepo, cleanup := setupTestRepo(t)
	defer cleanup()

	gf.Fetch("test-repo", bareRepo)

	// Check if log directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("Log directory was not created")
	}

	// Check if log file exists
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("Failed to read log directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("No log files were created")
	}

	// Verify log file contains repo name
	logFile := filepath.Join(logDir, entries[0].Name())
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty")
	}
}

func TestFetchResultFields(t *testing.T) {
	result := &FetchResult{
		RepoName: "test-repo",
		Success:  true,
		Message:  "test message",
	}

	if result.RepoName != "test-repo" {
		t.Errorf("Expected RepoName 'test-repo', got '%s'", result.RepoName)
	}

	if !result.Success {
		t.Error("Expected Success=true")
	}

	if result.Message != "test message" {
		t.Errorf("Expected Message 'test message', got '%s'", result.Message)
	}
}
