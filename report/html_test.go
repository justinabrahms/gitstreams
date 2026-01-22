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
		{ActivityCreatedRepo, "📦"},
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
		{"generated time", "Jan 15, 2024 2:30 PM"},
		{"period", "Jan 8 - Jan 15, 2024"},
		{"total activities", "<strong>3</strong> interesting"},
		{"user count", "<strong>2</strong>"},
		{"octocat user", "octocat"},
		{"torvalds user", "torvalds"},
		{"golang/go repo", "golang/go"},
		{"linux repo", "torvalds/linux"},
		{"repo link", `href="https://github.com/golang/go"`},
		{"avatar", `src="https://github.com/octocat.png"`},
		{"star icon", "⭐"},
		{"created icon", "📦"},
		{"pushed icon", "📤"},
		{"details", "A new awesome project"},
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
	if !strings.Contains(html, "No interesting activity to report") {
		t.Error("Empty report should show empty state message")
	}
	if !strings.Contains(html, "<strong>0</strong> interesting") {
		t.Error("Empty report should show 0 activities")
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
	if !strings.Contains(html, "1</strong> interesting activity") {
		t.Error("Should use singular 'activity' for count of 1")
	}
	if !strings.Contains(html, "1</strong> user") {
		t.Error("Should use singular 'user' for count of 1")
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
