package sync

import (
	"testing"

	"colosscious.com/github-sync/internal/config"
	"colosscious.com/github-sync/internal/redmine"
	"github.com/stretchr/testify/assert"
)

func TestBuildGitHubIssueBody(t *testing.T) {
	cfg := &config.Config{
		Redmine: config.RedmineConfig{
			URL:        "https://redmine.example.com",
			DisplayURL: "", // 測試 fallback 到 URL
		},
	}

	syncer := &Syncer{
		config: cfg,
	}

	issue := redmine.Issue{
		ID:      123,
		Subject: "Test Issue",
		Description: "This is a test issue\nwith multiple lines",
		Project: redmine.Project{
			ID:   1,
			Name: "Test Project",
		},
		Tracker: redmine.Tracker{
			ID:   2,
			Name: "Feature",
		},
		Priority: redmine.Priority{
			ID:   3,
			Name: "High",
		},
		Author: redmine.User{
			ID:   7,
			Name: "John Doe",
		},
		CreatedOn: "2025-11-13T10:00:00Z",
	}

	body := syncer.buildGitHubIssueBody(issue)

	// 驗證包含關鍵資訊
	assert.Contains(t, body, "**From Redmine Issue #123**")
	assert.Contains(t, body, "**Project**: Test Project")
	assert.Contains(t, body, "**Tracker**: Feature")
	assert.Contains(t, body, "**Priority**: High")
	assert.Contains(t, body, "**Author**: John Doe")
	assert.Contains(t, body, "This is a test issue")
	assert.Contains(t, body, "with multiple lines")
	assert.Contains(t, body, "https://redmine.example.com/issues/123")
}

func TestBuildGitHubIssueBodyWithDisplayURL(t *testing.T) {
	cfg := &config.Config{
		Redmine: config.RedmineConfig{
			URL:        "http://redmine:3000",
			DisplayURL: "http://192.168.1.100:3000",
		},
	}

	syncer := &Syncer{
		config: cfg,
	}

	issue := redmine.Issue{
		ID:          789,
		Subject:     "Test with display URL",
		Description: "Testing display URL",
		Project:     redmine.Project{Name: "Test"},
		Tracker:     redmine.Tracker{Name: "Bug"},
		Priority:    redmine.Priority{Name: "Normal"},
		Author:      redmine.User{Name: "User"},
		CreatedOn:   "2025-11-13T10:00:00Z",
	}

	body := syncer.buildGitHubIssueBody(issue)
	// 應該使用 display_url，而不是 url
	assert.Contains(t, body, "http://192.168.1.100:3000/issues/789")
	assert.NotContains(t, body, "http://redmine:3000")
}

func TestBuildGitHubIssueBodyEmptyDescription(t *testing.T) {
	cfg := &config.Config{
		Redmine: config.RedmineConfig{
			URL: "https://redmine.example.com",
		},
	}

	syncer := &Syncer{
		config: cfg,
	}

	issue := redmine.Issue{
		ID:          456,
		Subject:     "Issue without description",
		Description: "",
		Project:     redmine.Project{Name: "Test"},
		Tracker:     redmine.Tracker{Name: "Bug"},
		Priority:    redmine.Priority{Name: "Normal"},
		Author:      redmine.User{Name: "Jane"},
		CreatedOn:   "2025-11-13T10:00:00Z",
	}

	body := syncer.buildGitHubIssueBody(issue)
	assert.Contains(t, body, "*No description*")
}

func TestMapLabels(t *testing.T) {
	syncer := &Syncer{}

	tests := []struct {
		name     string
		issue    redmine.Issue
		expected []string
	}{
		{
			name: "bug tracker",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Bug"},
				Priority: redmine.Priority{Name: "Normal"},
			},
			expected: []string{"bug", "from-redmine"},
		},
		{
			name: "feature tracker",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Feature"},
				Priority: redmine.Priority{Name: "Normal"},
			},
			expected: []string{"enhancement", "from-redmine"},
		},
		{
			name: "support tracker",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Support"},
				Priority: redmine.Priority{Name: "Normal"},
			},
			expected: []string{"question", "from-redmine"},
		},
		{
			name: "urgent priority",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Task"},
				Priority: redmine.Priority{Name: "Urgent"},
			},
			expected: []string{"priority:high", "from-redmine"},
		},
		{
			name: "high priority",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Task"},
				Priority: redmine.Priority{Name: "High"},
			},
			expected: []string{"priority:medium", "from-redmine"},
		},
		{
			name: "bug with urgent priority",
			issue: redmine.Issue{
				Tracker: redmine.Tracker{Name: "Bug"},
				Priority: redmine.Priority{Name: "Urgent"},
			},
			expected: []string{"bug", "priority:high", "from-redmine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := syncer.mapLabels(tt.issue)
			assert.Equal(t, tt.expected, labels)
		})
	}
}

func TestMapLabelsUnknownTracker(t *testing.T) {
	syncer := &Syncer{}

	issue := redmine.Issue{
		Tracker: redmine.Tracker{Name: "Unknown Tracker"},
		Priority: redmine.Priority{Name: "Normal"},
	}

	labels := syncer.mapLabels(issue)
	// 應該只有 from-redmine
	assert.Equal(t, []string{"from-redmine"}, labels)
}
