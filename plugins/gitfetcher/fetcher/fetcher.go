package fetcher

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FetchResult struct {
	RepoName  string
	Success   bool
	Message   string
	Timestamp time.Time
}

type GitFetcher struct {
	sshKeyPath string
	logPath    string
}

func NewGitFetcher(sshKeyPath, logPath string) *GitFetcher {
	return &GitFetcher{
		sshKeyPath: sshKeyPath,
		logPath:    logPath,
	}
}

// Clone executes git clone --mirror for a repository
func (gf *GitFetcher) Clone(name, url, localPath string) *FetchResult {
	result := &FetchResult{
		RepoName:  name,
		Timestamp: time.Now(),
	}

	log.Printf("Cloning %s from %s to %s...", name, url, localPath)

	// Prepare git clone --mirror command
	cmd := exec.Command("git", "clone", "--mirror", url, localPath)

	// Set SSH key if provided
	if gf.sshKeyPath != "" {
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", gf.sshKeyPath)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("clone failed: %v\nOutput: %s", err, string(output))
		gf.logResult(result)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully cloned as mirror repository")
	gf.logResult(result)
	return result
}

// Fetch executes git fetch for a repository, clones if not exists
func (gf *GitFetcher) Fetch(name, url, localPath string) *FetchResult {
	result := &FetchResult{
		RepoName:  name,
		Timestamp: time.Now(),
	}

	// Check if repository exists, clone if not
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		log.Printf("Repository %s does not exist, cloning...", name)
		return gf.Clone(name, url, localPath)
	}

	// Prepare git command
	cmd := exec.Command("git", "-C", localPath, "fetch", "--all", "--prune")

	// Set SSH key if provided
	if gf.sshKeyPath != "" {
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", gf.sshKeyPath)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("fetch failed: %v\nOutput: %s", err, string(output))
		gf.logResult(result)
		return result
	}

	result.Success = true
	result.Message = strings.TrimSpace(string(output))
	if result.Message == "" {
		result.Message = "Already up to date"
	}
	gf.logResult(result)
	return result
}

// logResult writes fetch result to log file
func (gf *GitFetcher) logResult(result *FetchResult) {
	if gf.logPath == "" {
		return
	}

	// Ensure log directory exists
	if err := os.MkdirAll(gf.logPath, 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
		return
	}

	// Create/append to daily log file
	logFile := filepath.Join(gf.logPath, fmt.Sprintf("fetch-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return
	}
	defer f.Close()

	status := "SUCCESS"
	if !result.Success {
		status = "FAILED"
	}

	logEntry := fmt.Sprintf("[%s] [%s] %s: %s\n",
		result.Timestamp.Format("2006-01-02 15:04:05"),
		status,
		result.RepoName,
		result.Message,
	)

	if _, err := f.WriteString(logEntry); err != nil {
		log.Printf("Failed to write log: %v", err)
	}
}
