package scheduler

import (
	"log"
	"sync"
	"time"

	"colosscious.com/gitfetcher/config"
	"colosscious.com/gitfetcher/fetcher"
)

type RepoStatus struct {
	Name         string
	URL          string
	LocalPath    string
	Interval     string
	LastFetch    time.Time
	LastResult   string
	LastSuccess  bool
	NextFetch    time.Time
	IsRunning    bool
	FetchCount   int
	SuccessCount int
	FailCount    int
}

type Scheduler struct {
	fetcher    *fetcher.GitFetcher
	repos      map[string]*RepoStatus
	stopChans  map[string]chan bool
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

func NewScheduler(gf *fetcher.GitFetcher) *Scheduler {
	return &Scheduler{
		fetcher:   gf,
		repos:     make(map[string]*RepoStatus),
		stopChans: make(map[string]chan bool),
	}
}

// LoadConfig loads repositories from config and starts schedulers
func (s *Scheduler) LoadConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop all existing schedulers
	for name, stopChan := range s.stopChans {
		close(stopChan)
		delete(s.stopChans, name)
	}

	// Clear old repos
	s.repos = make(map[string]*RepoStatus)

	// Start new schedulers
	for _, repo := range cfg.Repos {
		interval, _ := repo.ParseInterval()

		status := &RepoStatus{
			Name:      repo.Name,
			URL:       repo.URL,
			LocalPath: repo.LocalPath,
			Interval:  repo.Interval,
			NextFetch: time.Now(),
		}

		s.repos[repo.Name] = status
		stopChan := make(chan bool)
		s.stopChans[repo.Name] = stopChan

		s.wg.Add(1)
		go s.runScheduler(repo.Name, repo.LocalPath, interval, stopChan)
	}

	log.Printf("Loaded %d repositories", len(cfg.Repos))
}

// runScheduler is the main loop for each repository
func (s *Scheduler) runScheduler(name, localPath string, interval time.Duration, stopChan chan bool) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	s.executeFetch(name, localPath)

	for {
		select {
		case <-ticker.C:
			s.executeFetch(name, localPath)
		case <-stopChan:
			log.Printf("Stopping scheduler for %s", name)
			return
		}
	}
}

// executeFetch runs git fetch and updates status
func (s *Scheduler) executeFetch(name, localPath string) {
	s.mu.Lock()
	status, exists := s.repos[name]
	if !exists {
		s.mu.Unlock()
		return
	}
	status.IsRunning = true
	s.mu.Unlock()

	log.Printf("Fetching %s...", name)
	result := s.fetcher.Fetch(name, localPath)

	s.mu.Lock()
	status.IsRunning = false
	status.LastFetch = result.Timestamp
	status.LastResult = result.Message
	status.LastSuccess = result.Success
	status.FetchCount++

	if result.Success {
		status.SuccessCount++
	} else {
		status.FailCount++
	}

	// Calculate next fetch time
	if repoConfig, ok := s.getRepoConfig(name); ok {
		if interval, err := repoConfig.ParseInterval(); err == nil {
			status.NextFetch = time.Now().Add(interval)
		}
	}
	s.mu.Unlock()

	if result.Success {
		log.Printf("Fetch %s completed: %s", name, result.Message)
	} else {
		log.Printf("Fetch %s failed: %s", name, result.Message)
	}
}

// getRepoConfig is a helper to get interval from current config
func (s *Scheduler) getRepoConfig(name string) (*config.RepoConfig, bool) {
	status, exists := s.repos[name]
	if !exists {
		return nil, false
	}

	// Return a temporary config object for interval parsing
	return &config.RepoConfig{
		Name:      status.Name,
		Interval:  status.Interval,
		LocalPath: status.LocalPath,
		URL:       status.URL,
	}, true
}

// GetStatus returns current status of all repositories
func (s *Scheduler) GetStatus() map[string]*RepoStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*RepoStatus)
	for k, v := range s.repos {
		// Create a copy to avoid race conditions
		statusCopy := *v
		result[k] = &statusCopy
	}
	return result
}

// ManualFetch triggers an immediate fetch for a specific repository
func (s *Scheduler) ManualFetch(name string) error {
	s.mu.RLock()
	status, exists := s.repos[name]
	if !exists {
		s.mu.RUnlock()
		return nil
	}
	localPath := status.LocalPath
	s.mu.RUnlock()

	go s.executeFetch(name, localPath)
	return nil
}

// Stop gracefully stops all schedulers
func (s *Scheduler) Stop() {
	s.mu.Lock()
	for _, stopChan := range s.stopChans {
		close(stopChan)
	}
	s.mu.Unlock()

	s.wg.Wait()
	log.Println("All schedulers stopped")
}
