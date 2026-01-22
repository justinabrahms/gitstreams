package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestActivityIcon(t *testing.T) {
	tests := []struct {
		activityType ActivityType
		want         string
	}{
		{ActivityStarred, "⭐"},
		{ActivityCreatedRepo, "🆕"},
		{ActivityForked, "🔱"},
		{ActivityPushed, "📤"},
		{ActivityPR, "🔀"},
		{ActivityIssue, "🐛"},
		{ActivityType("unknown"), "📋"},
	}

	for _, tt := range tests {
		t.Run(string(tt.activityType), func(t *testing.T) {
			got := activityIcon(tt.activityType)
			if got != tt.want {
				t.Errorf("activityIcon(%q) = %q, want %q", tt.activityType, got, tt.want)
			}
		})
	}
}

func TestActivityVerb(t *testing.T) {
	tests := []struct {
		activityType ActivityType
		want         string
	}{
		{ActivityStarred, "starred"},
		{ActivityCreatedRepo, "created"},
		{ActivityForked, "forked"},
		{ActivityPushed, "pushed to"},
		{ActivityPR, "opened PR on"},
		{ActivityIssue, "opened issue on"},
		{ActivityType("unknown"), "acted on"},
	}

	for _, tt := range tests {
		t.Run(string(tt.activityType), func(t *testing.T) {
			got := activityVerb(tt.activityType)
			if got != tt.want {
				t.Errorf("activityVerb(%q) = %q, want %q", tt.activityType, got, tt.want)
			}
		})
	}
}

func TestReportTotalActivities(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   int
	}{
		{
			name:   "empty report",
			report: Report{},
			want:   0,
		},
		{
			name: "single user single activity",
			report: Report{
				UserActivities: []UserActivity{
					{User: "alice", Activities: []Activity{{Type: ActivityStarred}}},
				},
			},
			want: 1,
		},
		{
			name: "multiple users multiple activities",
			report: Report{
				UserActivities: []UserActivity{
					{User: "alice", Activities: []Activity{{Type: ActivityStarred}, {Type: ActivityForked}}},
					{User: "bob", Activities: []Activity{{Type: ActivityPR}}},
				},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.TotalActivities()
			if got != tt.want {
				t.Errorf("TotalActivities() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewHTMLGenerator(t *testing.T) {
	gen, err := NewHTMLGenerator()
	if err != nil {
		t.Fatalf("NewHTMLGenerator() error = %v", err)
	}
	if gen == nil {
		t.Fatal("NewHTMLGenerator() returned nil")
	}
}

func TestHTMLGeneratorGenerate(t *testing.T) {
	gen, err := NewHTMLGenerator()
	if err != nil {
		t.Fatalf("NewHTMLGenerator() error = %v", err)
	}

	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	periodStart := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	report := &Report{
		GeneratedAt: now,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		UserActivities: []UserActivity{
			{
				User:      "octocat",
				AvatarURL: "https://github.com/octocat.png",
				Activities: []Activity{
					{
						Type:      ActivityStarred,
						User:      "octocat",
						RepoName:  "golang/go",
						RepoURL:   "https://github.com/golang/go",
						Timestamp: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
						Details:   "",
					},
					{
						Type:      ActivityCreatedRepo,
						User:      "octocat",
						RepoName:  "octocat/awesome-project",
						RepoURL:   "https://github.com/octocat/awesome-project",
						Timestamp: time.Date(2024, 1, 13, 15, 30, 0, 0, time.UTC),
						Details:   "A new awesome project",
					},
				},
			},
			{
				User:      "torvalds",
				AvatarURL: "",
				Activities: []Activity{
					{
						Type:      ActivityPushed,
						User:      "torvalds",
						RepoName:  "torvalds/linux",
						RepoURL:   "https://github.com/torvalds/linux",
						Timestamp: time.Date(2024, 1, 12, 8, 0, 0, 0, time.UTC),
						Details:   "5 commits",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err = gen.Generate(&buf, report)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	html := buf.String()

	// Check for essential elements
	checks := []struct {
		name     string
		contains string
	}{
		{"DOCTYPE", "<!DOCTYPE html>"},
		{"title", "<title>GitStreams Activity Report</title>"},
		{"period", "Jan 8"},
		{"total activities", "<strong>3</strong>"},
		{"user count", "<strong>2</strong>"},
		{"octocat user", "octocat"},
		{"torvalds user", "torvalds"},
		{"golang/go repo", "golang/go"},
		{"linux repo", "torvalds/linux"},
		{"repo link", `href="https://github.com/golang/go"`},
		{"avatar", `src="https://github.com/octocat.png"`},
		{"star icon", "⭐"},
		{"created icon", "🆕"},
		{"pushed icon", "📤"},
		{"details", "A new awesome project"},
		{"highlight section", "Highlight of the Day"},
		{"hot badge on new repo", "🔥"},
		{"mvp badge", "MVP"},
		{"stats grid", "stats-grid"},
		{"category toggle button", `onclick="toggleView('category')"`},
		{"user toggle button", `onclick="toggleView('user')"`},
		{"category section", `class="category-section"`},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !strings.Contains(html, check.contains) {
				t.Errorf("HTML should contain %q", check.contains)
			}
		})
	}
}

func TestHTMLGeneratorGenerateEmptyReport(t *testing.T) {
	gen, err := NewHTMLGenerator()
	if err != nil {
		t.Fatalf("NewHTMLGenerator() error = %v", err)
	}

	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	report := &Report{
		GeneratedAt:    now,
		PeriodStart:    now.AddDate(0, 0, -7),
		PeriodEnd:      now,
		UserActivities: []UserActivity{},
	}

	var buf bytes.Buffer
	err = gen.Generate(&buf, report)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	html := buf.String()

	// Should show empty state
	if !strings.Contains(html, "Nothing to see here") {
		t.Error("Empty report should show empty state message")
	}
	if !strings.Contains(html, "<strong>0</strong>") {
		t.Error("Empty report should show 0 activities")
	}
	// Should have fun tagline for empty report
	if !strings.Contains(html, "calm before the storm") {
		t.Error("Empty report should have calm tagline")
	}
}

func TestHTMLGeneratorGenerateSingular(t *testing.T) {
	gen, err := NewHTMLGenerator()
	if err != nil {
		t.Fatalf("NewHTMLGenerator() error = %v", err)
	}

	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	report := &Report{
		GeneratedAt: now,
		PeriodStart: now.AddDate(0, 0, -7),
		PeriodEnd:   now,
		UserActivities: []UserActivity{
			{
				User: "alice",
				Activities: []Activity{
					{Type: ActivityStarred, RepoName: "test/repo", RepoURL: "https://github.com/test/repo", Timestamp: now},
				},
			},
		},
	}

	var buf bytes.Buffer
	err = gen.Generate(&buf, report)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	html := buf.String()

	// Should use singular form
	if !strings.Contains(html, "1</strong> thing happened") {
		t.Error("Should use singular 'thing happened' for count of 1")
	}
	if !strings.Contains(html, "1</strong> developer") {
		t.Error("Should use singular 'developer' for count of 1")
	}
}

func TestActivityTypes(t *testing.T) {
	// Verify all defined activity types have icons and verbs
	types := []ActivityType{
		ActivityStarred,
		ActivityCreatedRepo,
		ActivityForked,
		ActivityPushed,
		ActivityPR,
		ActivityIssue,
	}

	for _, at := range types {
		t.Run(string(at), func(t *testing.T) {
			icon := activityIcon(at)
			if icon == "" || icon == "📋" {
				t.Errorf("activityIcon(%q) should return a specific icon", at)
			}

			verb := activityVerb(at)
			if verb == "" || verb == "acted on" {
				t.Errorf("activityVerb(%q) should return a specific verb", at)
			}
		})
	}
}

func TestCategoryName(t *testing.T) {
	tests := []struct {
		activityType ActivityType
		want         string
	}{
		{ActivityStarred, "New Stars"},
		{ActivityCreatedRepo, "Repos Created"},
		{ActivityForked, "Forks"},
		{ActivityPushed, "Recent Pushes"},
		{ActivityPR, "Pull Requests"},
		{ActivityIssue, "Issues Opened"},
		{ActivityType("unknown"), "Other Activity"},
	}

	for _, tt := range tests {
		t.Run(string(tt.activityType), func(t *testing.T) {
			got := categoryName(tt.activityType)
			if got != tt.want {
				t.Errorf("categoryName(%q) = %q, want %q", tt.activityType, got, tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "unknown time",
		},
		{
			name:     "just now (30 seconds ago)",
			input:    now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			input:    now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			input:    now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			input:    now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "3 hours ago",
			input:    now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "yesterday",
			input:    now.Add(-25 * time.Hour),
			expected: "yesterday",
		},
		{
			name:     "3 days ago",
			input:    now.Add(-3 * 24 * time.Hour),
			expected: "3 days ago",
		},
		{
			name:     "last week",
			input:    now.Add(-10 * 24 * time.Hour),
			expected: "last week",
		},
		{
			name:     "3 weeks ago",
			input:    now.Add(-21 * 24 * time.Hour),
			expected: "3 weeks ago",
		},
		{
			name:     "last month",
			input:    now.Add(-45 * 24 * time.Hour),
			expected: "last month",
		},
		{
			name:     "3 months ago",
			input:    now.Add(-90 * 24 * time.Hour),
			expected: "3 months ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.input)
			if got != tt.expected {
				t.Errorf("relativeTime() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReportActivitiesByCategory(t *testing.T) {
	now := time.Now()
	report := &Report{
		GeneratedAt: now,
		PeriodStart: now.AddDate(0, 0, -7),
		PeriodEnd:   now,
		UserActivities: []UserActivity{
			{
				User: "alice",
				Activities: []Activity{
					{Type: ActivityStarred, User: "alice", RepoName: "repo1"},
					{Type: ActivityStarred, User: "alice", RepoName: "repo2"},
					{Type: ActivityPushed, User: "alice", RepoName: "repo3"},
				},
			},
			{
				User: "bob",
				Activities: []Activity{
					{Type: ActivityStarred, User: "bob", RepoName: "repo4"},
					{Type: ActivityCreatedRepo, User: "bob", RepoName: "repo5"},
				},
			},
		},
	}

	categories := report.ActivitiesByCategory()

	// Should have 3 categories: starred, created_repo, pushed
	if len(categories) != 3 {
		t.Errorf("ActivitiesByCategory() returned %d categories, want 3", len(categories))
	}

	// First category should be starred (order defined in ActivitiesByCategory)
	if categories[0].Type != ActivityStarred {
		t.Errorf("First category type = %q, want %q", categories[0].Type, ActivityStarred)
	}
	if len(categories[0].Activities) != 3 {
		t.Errorf("Starred activities count = %d, want 3", len(categories[0].Activities))
	}

	// Second should be created_repo
	if categories[1].Type != ActivityCreatedRepo {
		t.Errorf("Second category type = %q, want %q", categories[1].Type, ActivityCreatedRepo)
	}
	if len(categories[1].Activities) != 1 {
		t.Errorf("CreatedRepo activities count = %d, want 1", len(categories[1].Activities))
	}

	// Third should be pushed
	if categories[2].Type != ActivityPushed {
		t.Errorf("Third category type = %q, want %q", categories[2].Type, ActivityPushed)
	}
	if len(categories[2].Activities) != 1 {
		t.Errorf("Pushed activities count = %d, want 1", len(categories[2].Activities))
	}
}

func TestReportActivitiesByCategoryEmpty(t *testing.T) {
	report := &Report{}
	categories := report.ActivitiesByCategory()

	if len(categories) != 0 {
		t.Errorf("ActivitiesByCategory() on empty report returned %d categories, want 0", len(categories))
	}
}

func TestGetStats(t *testing.T) {
	now := time.Now()
	report := &Report{
		UserActivities: []UserActivity{
			{
				User: "alice",
				Activities: []Activity{
					{Type: ActivityStarred, Timestamp: now},
					{Type: ActivityStarred, Timestamp: now},
					{Type: ActivityCreatedRepo, Timestamp: now},
				},
			},
			{
				User: "bob",
				Activities: []Activity{
					{Type: ActivityPR, Timestamp: now},
					{Type: ActivityIssue, Timestamp: now},
					{Type: ActivityForked, Timestamp: now},
				},
			},
		},
	}

	stats := report.GetStats()

	if stats.Stars != 2 {
		t.Errorf("GetStats().Stars = %d, want 2", stats.Stars)
	}
	if stats.Repos != 1 {
		t.Errorf("GetStats().Repos = %d, want 1", stats.Repos)
	}
	if stats.PRs != 1 {
		t.Errorf("GetStats().PRs = %d, want 1", stats.PRs)
	}
	if stats.Issues != 1 {
		t.Errorf("GetStats().Issues = %d, want 1", stats.Issues)
	}
	if stats.Forks != 1 {
		t.Errorf("GetStats().Forks = %d, want 1", stats.Forks)
	}
}

func TestGetHighlight(t *testing.T) {
	now := time.Now()

	tests := []struct {
		report   *Report
		name     string
		wantType ActivityType
		wantUser string
		wantNil  bool
	}{
		{
			name:    "empty report",
			report:  &Report{},
			wantNil: true,
		},
		{
			name: "prefers new repos",
			report: &Report{
				UserActivities: []UserActivity{
					{
						User: "alice",
						Activities: []Activity{
							{Type: ActivityStarred, Timestamp: now},
							{Type: ActivityCreatedRepo, Timestamp: now},
						},
					},
				},
			},
			wantType: ActivityCreatedRepo,
			wantUser: "alice",
		},
		{
			name: "prefers PRs over stars",
			report: &Report{
				UserActivities: []UserActivity{
					{
						User: "bob",
						Activities: []Activity{
							{Type: ActivityStarred, Timestamp: now},
							{Type: ActivityPR, Timestamp: now},
						},
					},
				},
			},
			wantType: ActivityPR,
			wantUser: "bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			highlight := tt.report.GetHighlight()
			if tt.wantNil {
				if highlight != nil {
					t.Error("GetHighlight() should return nil for empty report")
				}
				return
			}
			if highlight == nil {
				t.Fatal("GetHighlight() should not return nil")
			}
			if highlight.Activity.Type != tt.wantType {
				t.Errorf("GetHighlight().Activity.Type = %v, want %v", highlight.Activity.Type, tt.wantType)
			}
			if highlight.User != tt.wantUser {
				t.Errorf("GetHighlight().User = %v, want %v", highlight.User, tt.wantUser)
			}
			if highlight.Reason == "" {
				t.Error("GetHighlight().Reason should not be empty")
			}
		})
	}
}

func TestMostActiveUser(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		report *Report
		want   string
	}{
		{
			name:   "empty report",
			report: &Report{},
			want:   "",
		},
		{
			name: "single user",
			report: &Report{
				UserActivities: []UserActivity{
					{User: "alice", Activities: []Activity{{Type: ActivityStarred, Timestamp: now}}},
				},
			},
			want: "alice",
		},
		{
			name: "multiple users - most active wins",
			report: &Report{
				UserActivities: []UserActivity{
					{User: "alice", Activities: []Activity{{Type: ActivityStarred, Timestamp: now}}},
					{User: "bob", Activities: []Activity{
						{Type: ActivityStarred, Timestamp: now},
						{Type: ActivityPR, Timestamp: now},
						{Type: ActivityCreatedRepo, Timestamp: now},
					}},
					{User: "carol", Activities: []Activity{
						{Type: ActivityStarred, Timestamp: now},
						{Type: ActivityPR, Timestamp: now},
					}},
				},
			},
			want: "bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.MostActiveUser()
			if got != tt.want {
				t.Errorf("MostActiveUser() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsHotActivity(t *testing.T) {
	tests := []struct {
		activityType ActivityType
		want         bool
	}{
		{ActivityCreatedRepo, true},
		{ActivityPR, true},
		{ActivityStarred, false},
		{ActivityForked, false},
		{ActivityPushed, false},
		{ActivityIssue, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.activityType), func(t *testing.T) {
			got := IsHotActivity(tt.activityType)
			if got != tt.want {
				t.Errorf("IsHotActivity(%q) = %v, want %v", tt.activityType, got, tt.want)
			}
		})
	}
}

func TestTagline(t *testing.T) {
	tests := []struct {
		contains string
		count    int
	}{
		{"calm", 0},
		{"quiet", 2},
		{"busy", 5},
		{"action", 15},
		{"FIRE", 50},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			got := tagline(tt.count)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("tagline(%d) = %q, should contain %q", tt.count, got, tt.contains)
			}
		})
	}
}

func TestHTMLGeneratorGenerateCategoryView(t *testing.T) {
	gen, err := NewHTMLGenerator()
	if err != nil {
		t.Fatalf("NewHTMLGenerator() error = %v", err)
	}

	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	report := &Report{
		GeneratedAt: now,
		PeriodStart: now.AddDate(0, 0, -7),
		PeriodEnd:   now,
		UserActivities: []UserActivity{
			{
				User: "alice",
				Activities: []Activity{
					{Type: ActivityStarred, User: "alice", RepoName: "repo1", RepoURL: "https://github.com/repo1", Timestamp: now},
					{Type: ActivityPushed, User: "alice", RepoName: "repo2", RepoURL: "https://github.com/repo2", Timestamp: now},
				},
			},
		},
	}

	var buf bytes.Buffer
	err = gen.Generate(&buf, report)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	html := buf.String()

	// Check for category view elements
	checks := []struct {
		name     string
		contains string
	}{
		{"category toggle button", `onclick="toggleView('category')"`},
		{"user toggle button", `onclick="toggleView('user')"`},
		{"category section", `class="category-section"`},
		{"category title stars", "New Stars"},
		{"category title pushes", "Recent Pushes"},
		{"collapsible details", "<details open>"},
		{"summary element", "<summary>"},
		{"view toggle script", "function toggleView"},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !strings.Contains(html, check.contains) {
				t.Errorf("HTML should contain %q", check.contains)
			}
		})
	}
}

func TestRelativeTimeOldDates(t *testing.T) {
	// Test dates more than a year old - should show absolute date
	oldDate := time.Date(2020, 6, 15, 10, 30, 0, 0, time.UTC)
	got := relativeTime(oldDate)

	// Should contain the year
	if !strings.Contains(got, "2020") {
		t.Errorf("relativeTime() for old date should show year, got %q", got)
	}
}
