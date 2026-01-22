package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justinabrahms/gitstreams/diff"
	"github.com/justinabrahms/gitstreams/github"
	"github.com/justinabrahms/gitstreams/notify"
	"github.com/justinabrahms/gitstreams/report"
	"github.com/justinabrahms/gitstreams/storage"
)

// mockGitHubClient implements GitHubClient for testing.
type mockGitHubClient struct {
	followedErr   error
	starredRepos  map[string][]github.Repository
	ownedRepos    map[string][]github.Repository
	events        map[string][]github.Event
	starredErr    map[string]error
	ownedErr      map[string]error
	eventsErr     map[string]error
	followedUsers []github.User
}

func (m *mockGitHubClient) GetFollowedUsers(ctx context.Context) ([]github.User, error) {
	return m.followedUsers, m.followedErr
}

func (m *mockGitHubClient) GetStarredReposByUsername(ctx context.Context, username string) ([]github.Repository, error) {
	if m.starredErr != nil {
		if err, ok := m.starredErr[username]; ok {
			return nil, err
		}
	}
	return m.starredRepos[username], nil
}

func (m *mockGitHubClient) GetOwnedReposByUsername(ctx context.Context, username string) ([]github.Repository, error) {
	if m.ownedErr != nil {
		if err, ok := m.ownedErr[username]; ok {
			return nil, err
		}
	}
	return m.ownedRepos[username], nil
}

func (m *mockGitHubClient) GetRecentEvents(ctx context.Context, username string) ([]github.Event, error) {
	if m.eventsErr != nil {
		if err, ok := m.eventsErr[username]; ok {
			return nil, err
		}
	}
	return m.events[username], nil
}

// mockStore implements Store for testing.
type mockStore struct {
	saveErr       error
	getErr        error
	savedSnapshot *storage.Snapshot
	snapshots     []*storage.Snapshot
	savedCalled   bool
	closeCalled   bool
}

func (m *mockStore) Save(snapshot *storage.Snapshot) error {
	m.savedCalled = true
	m.savedSnapshot = snapshot
	return m.saveErr
}

func (m *mockStore) GetByUser(userID string, limit int) ([]*storage.Snapshot, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.snapshots, nil
}

func (m *mockStore) GetByTimeRange(userID string, start, end time.Time) ([]*storage.Snapshot, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	// For testing, return snapshots that fall within the time range
	var filtered []*storage.Snapshot
	for _, s := range m.snapshots {
		if !s.Timestamp.Before(start) && !s.Timestamp.After(end) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (m *mockStore) Close() error {
	m.closeCalled = true
	return nil
}

// mockNotifier implements Notifier for testing.
type mockNotifier struct {
	sentNotification *notify.Notification
	sendErr          error
}

func (m *mockNotifier) Send(n notify.Notification) error {
	m.sentNotification = &n
	return m.sendErr
}

// mockReportGenerator implements ReportGenerator for testing.
type mockReportGenerator struct {
	generatedReport *report.Report
	generateErr     error
}

func (m *mockReportGenerator) Generate(w io.Writer, r *report.Report) error {
	m.generatedReport = r
	if m.generateErr != nil {
		return m.generateErr
	}
	_, err := w.Write([]byte("<html>mock report</html>"))
	return err
}

func fixedTime() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

func TestRun_MissingToken(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Unset GITHUB_TOKEN for this test
	t.Setenv("GITHUB_TOKEN", "")

	deps := DefaultDependencies()
	result := run(&stdout, &stderr, []string{}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1, got %d", result)
	}
	if !strings.Contains(stderr.String(), "GITHUB_TOKEN") {
		t.Errorf("expected error about GITHUB_TOKEN, got: %s", stderr.String())
	}
}

func TestRun_SuccessfulRun_NoChanges(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "testuser", ID: 1},
		},
		starredRepos: map[string][]github.Repository{
			"testuser": {
				{Name: "repo1", Owner: github.User{Login: "owner1"}},
			},
		},
		ownedRepos: map[string][]github.Repository{},
		events:     map[string][]github.Event{},
	}

	// Create a previous snapshot with the same data (no changes)
	prevSnapshot := diff.NewSnapshot(fixedTime().Add(-24 * time.Hour))
	prevSnapshot.Users["testuser"] = diff.UserActivity{
		Username: "testuser",
		StarredRepos: []diff.Repo{
			{Owner: "owner1", Name: "repo1"},
		},
	}

	mockStoreInst := &mockStore{}
	// Convert prevSnapshot to storage format for the mock
	ss, _ := snapshotToStorage(prevSnapshot)
	mockStoreInst.snapshots = []*storage.Snapshot{ss}

	mockNotifierInst := &mockNotifier{}
	mockGenInst := &mockReportGenerator{}

	browserOpened := false
	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory:     func() Notifier { return mockNotifierInst },
		ReportGenerator:     func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:         func(url string) error { browserOpened = true; return nil },
		Now:                 fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-no-notify",
		"-no-open",
	}, deps)

	if result != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", result, stderr.String())
	}

	if !strings.Contains(stdout.String(), "No new activity") {
		t.Errorf("expected 'No new activity' message, got: %s", stdout.String())
	}

	if browserOpened {
		t.Error("browser should not have been opened when there are no changes")
	}
}

func TestRun_SuccessfulRun_WithChanges(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "testuser", ID: 1},
		},
		starredRepos: map[string][]github.Repository{
			"testuser": {
				{Name: "new-repo", Owner: github.User{Login: "owner1"}, Description: "A new repo"},
			},
		},
		ownedRepos: map[string][]github.Repository{},
		events:     map[string][]github.Event{},
	}

	// Empty previous snapshot (all current data is "new")
	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{},
	}

	mockNotifierInst := &mockNotifier{}
	mockGenInst := &mockReportGenerator{}

	browserOpened := false
	reportPath := filepath.Join(tmpDir, "report.html")

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory:     func() Notifier { return mockNotifierInst },
		ReportGenerator:     func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:         func(url string) error { browserOpened = true; return nil },
		Now:                 fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-report", reportPath,
	}, deps)

	if result != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", result, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Report written to") {
		t.Errorf("expected 'Report written to' message, got: %s", stdout.String())
	}

	if !browserOpened {
		t.Error("browser should have been opened")
	}

	if mockNotifierInst.sentNotification == nil {
		t.Error("notification should have been sent")
	} else {
		if mockNotifierInst.sentNotification.Title != "GitStreams" {
			t.Errorf("expected notification title 'GitStreams', got: %s", mockNotifierInst.sentNotification.Title)
		}
	}

	if mockGenInst.generatedReport == nil {
		t.Error("report should have been generated")
	}

	if !mockStoreInst.savedCalled {
		t.Error("snapshot should have been saved")
	}
}

func TestRun_GitHubAPIError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedErr: errors.New("GitHub API error"),
	}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return &mockStore{}, nil },
		NotifierFactory:     func() Notifier { return &mockNotifier{} },
		ReportGenerator:     func() (ReportGenerator, error) { return &mockReportGenerator{}, nil },
		OpenBrowser:         func(url string) error { return nil },
		Now:                 fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
	}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1, got %d", result)
	}

	if !strings.Contains(stderr.String(), "fetching activity") {
		t.Errorf("expected error about fetching activity, got: %s", stderr.String())
	}
}

func TestRun_StoreError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			return &mockGitHubClient{followedUsers: []github.User{{Login: "test"}}}
		},
		StoreFactory: func(dbPath string) (Store, error) {
			return nil, errors.New("database error")
		},
		NotifierFactory: func() Notifier { return &mockNotifier{} },
		ReportGenerator: func() (ReportGenerator, error) { return &mockReportGenerator{}, nil },
		OpenBrowser:     func(url string) error { return nil },
		Now:             fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", "/some/path.db",
	}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1, got %d", result)
	}

	if !strings.Contains(stderr.String(), "opening database") {
		t.Errorf("expected error about database, got: %s", stderr.String())
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		check    func(*testing.T, *Config)
		name     string
		envToken string
		args     []string
		wantErr  bool
	}{
		{
			name:     "defaults with env token",
			args:     []string{},
			envToken: "env-token",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Token != "env-token" {
					t.Errorf("expected token from env, got: %s", cfg.Token)
				}
				if cfg.Verbose {
					t.Error("verbose should be false by default")
				}
			},
		},
		{
			name:     "explicit token overrides env",
			args:     []string{"-token", "explicit-token"},
			envToken: "env-token",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Token != "explicit-token" {
					t.Errorf("expected explicit token, got: %s", cfg.Token)
				}
			},
		},
		{
			name:     "verbose flag",
			args:     []string{"-v"},
			envToken: "token",
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("verbose should be true")
				}
			},
		},
		{
			name:     "no-notify flag",
			args:     []string{"-no-notify"},
			envToken: "token",
			check: func(t *testing.T, cfg *Config) {
				if !cfg.NoNotify {
					t.Error("NoNotify should be true")
				}
			},
		},
		{
			name:     "no-open flag",
			args:     []string{"-no-open"},
			envToken: "token",
			check: func(t *testing.T, cfg *Config) {
				if !cfg.NoOpen {
					t.Error("NoOpen should be true")
				}
			},
		},
		{
			name:     "custom db path",
			args:     []string{"-db", "/custom/path.db"},
			envToken: "token",
			check: func(t *testing.T, cfg *Config) {
				if cfg.DBPath != "/custom/path.db" {
					t.Errorf("expected custom db path, got: %s", cfg.DBPath)
				}
			},
		},
		{
			name:     "custom report path",
			args:     []string{"-report", "/custom/report.html"},
			envToken: "token",
			check: func(t *testing.T, cfg *Config) {
				if cfg.ReportPath != "/custom/report.html" {
					t.Errorf("expected custom report path, got: %s", cfg.ReportPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.envToken)

			cfg, err := parseFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConvertRepo(t *testing.T) {
	ghRepo := github.Repository{
		Name:        "test-repo",
		Description: "A test repository",
		Language:    "Go",
		StarCount:   100,
		Owner:       github.User{Login: "owner"},
	}

	diffRepo := convertRepo(ghRepo)

	if diffRepo.Name != "test-repo" {
		t.Errorf("expected name 'test-repo', got: %s", diffRepo.Name)
	}
	if diffRepo.Owner != "owner" {
		t.Errorf("expected owner 'owner', got: %s", diffRepo.Owner)
	}
	if diffRepo.Description != "A test repository" {
		t.Errorf("expected description, got: %s", diffRepo.Description)
	}
	if diffRepo.Language != "Go" {
		t.Errorf("expected language 'Go', got: %s", diffRepo.Language)
	}
	if diffRepo.Stars != 100 {
		t.Errorf("expected 100 stars, got: %d", diffRepo.Stars)
	}
}

func TestConvertEvent(t *testing.T) {
	eventTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	ghEvent := github.Event{
		Type:      "PushEvent",
		Actor:     github.User{Login: "actor"},
		Repo:      github.EventRepo{Name: "owner/repo"},
		CreatedAt: eventTime,
	}

	diffEvent := convertEvent(ghEvent)

	if diffEvent.Type != "PushEvent" {
		t.Errorf("expected type 'PushEvent', got: %s", diffEvent.Type)
	}
	if diffEvent.Actor != "actor" {
		t.Errorf("expected actor 'actor', got: %s", diffEvent.Actor)
	}
	if diffEvent.Repo != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got: %s", diffEvent.Repo)
	}
	if !diffEvent.CreatedAt.Equal(eventTime) {
		t.Errorf("expected time %v, got: %v", eventTime, diffEvent.CreatedAt)
	}
}

func TestEventTypeToActivityType(t *testing.T) {
	tests := []struct {
		input    string
		expected report.ActivityType
	}{
		{"WatchEvent", report.ActivityStarred},
		{"CreateEvent", report.ActivityCreatedRepo},
		{"ForkEvent", report.ActivityForked},
		{"PushEvent", report.ActivityPushed},
		{"PullRequestEvent", report.ActivityPR},
		{"IssuesEvent", report.ActivityIssue},
		{"UnknownEvent", report.ActivityType("UnknownEvent")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := eventTypeToActivityType(tt.input)
			if result != tt.expected {
				t.Errorf("eventTypeToActivityType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatNotificationMessage(t *testing.T) {
	tests := []struct {
		name     string
		result   *diff.Result
		expected string
	}{
		{
			name:     "empty result",
			result:   &diff.Result{},
			expected: "New activity detected",
		},
		{
			name: "only stars",
			result: &diff.Result{
				NewStars: []diff.RepoChange{{}, {}},
			},
			expected: "2 new stars",
		},
		{
			name: "stars and repos",
			result: &diff.Result{
				NewStars: []diff.RepoChange{{}},
				NewRepos: []diff.RepoChange{{}, {}, {}},
			},
			expected: "1 new stars and 3 new repos",
		},
		{
			name: "all types",
			result: &diff.Result{
				NewStars:  []diff.RepoChange{{}},
				NewRepos:  []diff.RepoChange{{}, {}},
				NewEvents: []diff.EventChange{{}, {}, {}, {}},
				NewUsers:  []string{"user1"},
			},
			expected: "1 new stars, 2 new repos, 4 events and 1 new users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNotificationMessage(tt.result)
			if result != tt.expected {
				t.Errorf("formatNotificationMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	original := diff.NewSnapshot(fixedTime())
	original.Users["user1"] = diff.UserActivity{
		Username: "user1",
		StarredRepos: []diff.Repo{
			{Owner: "owner", Name: "repo1", Description: "desc", Language: "Go", Stars: 10},
		},
		OwnedRepos: []diff.Repo{
			{Owner: "user1", Name: "myrepo"},
		},
		Events: []diff.Event{
			{Type: "PushEvent", Actor: "user1", Repo: "user1/myrepo", CreatedAt: fixedTime()},
		},
	}

	// Convert to storage format
	stored, err := snapshotToStorage(original)
	if err != nil {
		t.Fatalf("snapshotToStorage failed: %v", err)
	}

	if stored.UserID != snapshotUserID {
		t.Errorf("expected userID %q, got %q", snapshotUserID, stored.UserID)
	}

	// Convert back
	restored, err := storageToSnapshot(stored)
	if err != nil {
		t.Fatalf("storageToSnapshot failed: %v", err)
	}

	// Verify data
	if len(restored.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(restored.Users))
	}

	user, ok := restored.Users["user1"]
	if !ok {
		t.Fatal("user1 not found in restored snapshot")
	}

	if user.Username != "user1" {
		t.Errorf("expected username 'user1', got %q", user.Username)
	}

	if len(user.StarredRepos) != 1 {
		t.Errorf("expected 1 starred repo, got %d", len(user.StarredRepos))
	}

	if len(user.OwnedRepos) != 1 {
		t.Errorf("expected 1 owned repo, got %d", len(user.OwnedRepos))
	}

	if len(user.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(user.Events))
	}
}

func TestBuildReport(t *testing.T) {
	result := &diff.Result{
		OldCapturedAt: fixedTime().Add(-24 * time.Hour),
		NewCapturedAt: fixedTime(),
		NewStars: []diff.RepoChange{
			{Username: "user1", Repo: diff.Repo{Owner: "owner", Name: "starred-repo", Description: "A repo"}},
		},
		NewRepos: []diff.RepoChange{
			{Username: "user1", Repo: diff.Repo{Owner: "user1", Name: "new-repo"}},
		},
		NewEvents: []diff.EventChange{
			{Username: "user2", Event: diff.Event{Type: "PushEvent", Repo: "user2/repo", CreatedAt: fixedTime()}},
		},
	}

	rpt := buildReport(result, result.OldCapturedAt, result.NewCapturedAt, fixedTime())

	if rpt.TotalActivities() != 3 {
		t.Errorf("expected 3 total activities, got %d", rpt.TotalActivities())
	}

	if len(rpt.UserActivities) != 2 {
		t.Errorf("expected 2 users with activities, got %d", len(rpt.UserActivities))
	}
}

func TestBuildReport_ManyUsersWithActivity(t *testing.T) {
	// Simulate 30 users, each with one event - should result in 30 UserActivities
	result := &diff.Result{
		OldCapturedAt: fixedTime().Add(-24 * time.Hour),
		NewCapturedAt: fixedTime(),
	}

	// Create 30 users with events
	for i := 0; i < 30; i++ {
		username := fmt.Sprintf("user%d", i)
		result.NewEvents = append(result.NewEvents, diff.EventChange{
			Username: username,
			Event:    diff.Event{Type: "PushEvent", Repo: username + "/repo", CreatedAt: fixedTime()},
		})
	}

	rpt := buildReport(result, result.OldCapturedAt, result.NewCapturedAt, fixedTime())

	if len(rpt.UserActivities) != 30 {
		t.Errorf("expected 30 users with activities, got %d", len(rpt.UserActivities))
	}

	if rpt.TotalActivities() != 30 {
		t.Errorf("expected 30 total activities, got %d", rpt.TotalActivities())
	}
}

func TestDiffCompare_FirstRun_AllUsersNewWithActivity(t *testing.T) {
	// Simulate first run: empty previous snapshot, 30 users in current snapshot
	// Each user has some events - all should appear as NewEvents

	previousSnapshot := diff.NewSnapshot(fixedTime().Add(-24 * time.Hour))

	currentSnapshot := diff.NewSnapshot(fixedTime())
	for i := 0; i < 30; i++ {
		username := fmt.Sprintf("user%d", i)
		currentSnapshot.Users[username] = diff.UserActivity{
			Username: username,
			Events: []diff.Event{
				{Type: "PushEvent", Repo: username + "/repo", CreatedAt: fixedTime()},
			},
		}
	}

	result := diff.Compare(previousSnapshot, currentSnapshot)

	// All 30 users should be new
	if len(result.NewUsers) != 30 {
		t.Errorf("expected 30 new users, got %d", len(result.NewUsers))
	}

	// All 30 users' events should be new
	if len(result.NewEvents) != 30 {
		t.Errorf("expected 30 new events, got %d", len(result.NewEvents))
	}

	// Build report should have 30 users
	rpt := buildReport(result, previousSnapshot.CapturedAt, currentSnapshot.CapturedAt, fixedTime())
	if len(rpt.UserActivities) != 30 {
		t.Errorf("expected 30 users in report, got %d", len(rpt.UserActivities))
	}
}

func TestDiffCompare_FirstRun_UsersWithNoActivity(t *testing.T) {
	// Simulate first run: 30 users but only 1 has activity
	// BUG REPRODUCTION: 30 users fetched but only 1 shows

	previousSnapshot := diff.NewSnapshot(fixedTime().Add(-24 * time.Hour))

	currentSnapshot := diff.NewSnapshot(fixedTime())
	// 29 users with NO activity
	for i := 0; i < 29; i++ {
		username := fmt.Sprintf("user%d", i)
		currentSnapshot.Users[username] = diff.UserActivity{
			Username: username,
			// No events, no stars, no repos
		}
	}
	// 1 user WITH activity
	currentSnapshot.Users["active_user"] = diff.UserActivity{
		Username: "active_user",
		Events: []diff.Event{
			{Type: "PushEvent", Repo: "active_user/repo", CreatedAt: fixedTime()},
		},
	}

	result := diff.Compare(previousSnapshot, currentSnapshot)

	// All 30 users should be new
	if len(result.NewUsers) != 30 {
		t.Errorf("expected 30 new users, got %d", len(result.NewUsers))
	}

	// Only 1 event
	if len(result.NewEvents) != 1 {
		t.Errorf("expected 1 new event, got %d", len(result.NewEvents))
	}

	// Build report - should only show 1 user (the one with activity)
	// This is actually EXPECTED behavior - users without activity don't appear
	rpt := buildReport(result, previousSnapshot.CapturedAt, currentSnapshot.CapturedAt, fixedTime())
	t.Logf("NewUsers=%d, NewEvents=%d, UserActivities=%d",
		len(result.NewUsers), len(result.NewEvents), len(rpt.UserActivities))

	// The "bug" is that we have 30 NewUsers but only 1 UserActivity
	// If we want ALL users to appear, we need to change buildReport
	if len(rpt.UserActivities) != 1 {
		t.Errorf("expected 1 user with activity (current behavior), got %d", len(rpt.UserActivities))
	}
}

func TestFetchActivity_VerboseOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "user1"},
		},
		starredRepos: map[string][]github.Repository{},
		ownedRepos:   map[string][]github.Repository{},
		events:       map[string][]github.Event{},
	}

	ctx := context.Background()
	now := fixedTime()
	cutoff := now.AddDate(0, 0, -30) // 30 days ago
	_, err := fetchActivity(ctx, mockClient, now, cutoff, &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("fetchActivity failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Fetching activity for user1") {
		t.Errorf("expected verbose output about user1, got: %s", stdout.String())
	}
}

func TestFetchActivity_HandlesPartialErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "user1"},
		},
		starredRepos: map[string][]github.Repository{},
		ownedRepos:   map[string][]github.Repository{},
		events:       map[string][]github.Event{},
		starredErr: map[string]error{
			"user1": errors.New("rate limited"),
		},
	}

	ctx := context.Background()
	now := fixedTime()
	cutoff := now.AddDate(0, 0, -30) // 30 days ago
	snapshot, err := fetchActivity(ctx, mockClient, now, cutoff, &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("fetchActivity should not fail on partial errors: %v", err)
	}

	// User should still be in snapshot even if starred repos failed
	if _, ok := snapshot.Users["user1"]; !ok {
		t.Error("user1 should be in snapshot despite starred repos error")
	}

	if !strings.Contains(stdout.String(), "Warning") {
		t.Errorf("expected warning in output, got: %s", stdout.String())
	}
}

func TestLoadPreviousSnapshot_Empty(t *testing.T) {
	store := &mockStore{snapshots: []*storage.Snapshot{}}

	snapshot, err := loadPreviousSnapshot(store)
	if err != nil {
		t.Fatalf("loadPreviousSnapshot failed: %v", err)
	}

	if len(snapshot.Users) != 0 {
		t.Errorf("expected empty users map, got %d users", len(snapshot.Users))
	}
}

func TestFetchActivity_ProgressOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "user1"},
			{Login: "user2"},
			{Login: "user3"},
		},
		starredRepos: map[string][]github.Repository{},
		ownedRepos:   map[string][]github.Repository{},
		events:       map[string][]github.Event{},
	}

	ctx := context.Background()
	now := fixedTime()
	cutoff := now.AddDate(0, 0, -30) // 30 days ago
	_, err := fetchActivity(ctx, mockClient, now, cutoff, &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("fetchActivity failed: %v", err)
	}

	// Progress should be written to stderr, not stdout
	if strings.Contains(stdout.String(), "Fetching activity for user") {
		t.Errorf("progress should not be written to stdout when not verbose: %s", stdout.String())
	}

	stderrOutput := stderr.String()
	// Check that progress messages are in stderr
	if !strings.Contains(stderrOutput, "Fetching activity for user 1/3: user1...") {
		t.Errorf("expected progress message for user1 in stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "Fetching activity for user 2/3: user2...") {
		t.Errorf("expected progress message for user2 in stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "Fetching activity for user 3/3: user3...") {
		t.Errorf("expected progress message for user3 in stderr, got: %q", stderrOutput)
	}
}

func TestFetchActivity_FiltersOldData(t *testing.T) {
	var stdout, stderr bytes.Buffer

	now := fixedTime()               // 2024-01-15
	cutoff := now.AddDate(0, 0, -30) // 30 days ago = 2023-12-16

	// Create repos with different creation dates
	recentRepo := github.Repository{
		Name:      "recent-repo",
		Owner:     github.User{Login: "owner"},
		CreatedAt: now.AddDate(0, 0, -10), // 10 days ago - should be included
	}
	oldRepo := github.Repository{
		Name:      "old-repo",
		Owner:     github.User{Login: "owner"},
		CreatedAt: now.AddDate(-1, 0, 0), // 1 year ago - should be filtered out
	}

	// Create events with different dates
	recentEvent := github.Event{
		Type:      "PushEvent",
		Actor:     github.User{Login: "user1"},
		Repo:      github.EventRepo{Name: "owner/repo"},
		CreatedAt: now.AddDate(0, 0, -5), // 5 days ago - should be included
	}
	oldEvent := github.Event{
		Type:      "PushEvent",
		Actor:     github.User{Login: "user1"},
		Repo:      github.EventRepo{Name: "owner/old-repo"},
		CreatedAt: now.AddDate(0, -3, 0), // 3 months ago - should be filtered out
	}

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{
			{Login: "user1"},
		},
		starredRepos: map[string][]github.Repository{
			"user1": {recentRepo, oldRepo},
		},
		ownedRepos: map[string][]github.Repository{
			"user1": {recentRepo, oldRepo},
		},
		events: map[string][]github.Event{
			"user1": {recentEvent, oldEvent},
		},
	}

	ctx := context.Background()
	snapshot, err := fetchActivity(ctx, mockClient, now, cutoff, &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("fetchActivity failed: %v", err)
	}

	activity := snapshot.Users["user1"]

	// Check starred repos filtering
	if len(activity.StarredRepos) != 1 {
		t.Errorf("expected 1 starred repo (recent only), got %d", len(activity.StarredRepos))
	}
	if len(activity.StarredRepos) > 0 && activity.StarredRepos[0].Name != "recent-repo" {
		t.Errorf("expected recent-repo, got %s", activity.StarredRepos[0].Name)
	}

	// Check owned repos filtering
	if len(activity.OwnedRepos) != 1 {
		t.Errorf("expected 1 owned repo (recent only), got %d", len(activity.OwnedRepos))
	}
	if len(activity.OwnedRepos) > 0 && activity.OwnedRepos[0].Name != "recent-repo" {
		t.Errorf("expected recent-repo, got %s", activity.OwnedRepos[0].Name)
	}

	// Check events filtering
	if len(activity.Events) != 1 {
		t.Errorf("expected 1 event (recent only), got %d", len(activity.Events))
	}
	if len(activity.Events) > 0 && activity.Events[0].Repo != "owner/repo" {
		t.Errorf("expected owner/repo event, got %s", activity.Events[0].Repo)
	}
}

func TestFetchActivity_FiltersBoundaryDates(t *testing.T) {
	var stdout, stderr bytes.Buffer

	now := fixedTime()               // 2024-01-15
	cutoff := now.AddDate(0, 0, -30) // exactly 30 days ago

	// Repo created exactly at cutoff should be included (not before)
	boundaryRepo := github.Repository{
		Name:      "boundary-repo",
		Owner:     github.User{Login: "owner"},
		CreatedAt: cutoff, // exactly at cutoff - should be included
	}
	justBeforeRepo := github.Repository{
		Name:      "just-before-repo",
		Owner:     github.User{Login: "owner"},
		CreatedAt: cutoff.Add(-1 * time.Second), // 1 second before cutoff - should be filtered
	}

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{{Login: "user1"}},
		starredRepos: map[string][]github.Repository{
			"user1": {boundaryRepo, justBeforeRepo},
		},
		ownedRepos: map[string][]github.Repository{},
		events:     map[string][]github.Event{},
	}

	ctx := context.Background()
	snapshot, err := fetchActivity(ctx, mockClient, now, cutoff, &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("fetchActivity failed: %v", err)
	}

	activity := snapshot.Users["user1"]

	// Only boundary repo should be included
	if len(activity.StarredRepos) != 1 {
		t.Errorf("expected 1 starred repo (boundary only), got %d", len(activity.StarredRepos))
	}
	if len(activity.StarredRepos) > 0 && activity.StarredRepos[0].Name != "boundary-repo" {
		t.Errorf("expected boundary-repo, got %s", activity.StarredRepos[0].Name)
	}
}

func TestRun_DaysFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{},
	}
	mockStoreInst := &mockStore{snapshots: []*storage.Snapshot{}}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory:     func() Notifier { return &mockNotifier{} },
		ReportGenerator:     func() (ReportGenerator, error) { return &mockReportGenerator{}, nil },
		OpenBrowser:         func(url string) error { return nil },
		Now:                 fixedTime,
	}

	// Test with custom days flag
	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-days", "7",
		"-no-open",
		"-no-notify",
	}, deps)

	if result != 0 {
		t.Errorf("expected exit code 0, got %d", result)
	}
}

func TestRun_DaysFlagInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer

	deps := DefaultDependencies()

	// Test with invalid days (too low)
	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-days", "0",
	}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1 for invalid days, got %d", result)
	}
	if !strings.Contains(stderr.String(), "days must be between 1 and 365") {
		t.Errorf("expected error about days range, got: %s", stderr.String())
	}

	// Test with invalid days (too high)
	stdout.Reset()
	stderr.Reset()
	result = run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-days", "400",
	}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1 for invalid days, got %d", result)
	}
	if !strings.Contains(stderr.String(), "days must be between 1 and 365") {
		t.Errorf("expected error about days range, got: %s", stderr.String())
	}
}

func TestRun_NotificationError_DoesNotFail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{{Login: "testuser"}},
		starredRepos: map[string][]github.Repository{
			"testuser": {{Name: "repo", Owner: github.User{Login: "owner"}}},
		},
	}

	mockStoreInst := &mockStore{snapshots: []*storage.Snapshot{}}
	mockNotifierInst := &mockNotifier{sendErr: errors.New("notification failed")}
	mockGenInst := &mockReportGenerator{}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory:     func() Notifier { return mockNotifierInst },
		ReportGenerator:     func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:         func(url string) error { return nil },
		Now:                 fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-report", filepath.Join(tmpDir, "report.html"),
		"-no-open",
	}, deps)

	// Should still succeed even if notification fails
	if result != 0 {
		t.Errorf("expected exit code 0 despite notification error, got %d", result)
	}

	if !strings.Contains(stderr.String(), "could not send notification") {
		t.Errorf("expected warning about notification, got: %s", stderr.String())
	}
}

func TestParseSinceDate(t *testing.T) {
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		expected time.Time
		name     string
		input    string
		wantErr  bool
	}{
		{
			name:     "relative days",
			input:    "7d",
			expected: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "relative weeks",
			input:    "2w",
			expected: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "relative months",
			input:    "1m",
			expected: time.Date(2025, 12, 22, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "absolute date",
			input:    "2026-01-15",
			expected: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "absolute date with slashes",
			input:    "2026/01/15",
			expected: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSinceDate(tt.input, now)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRun_HistoricalMode(t *testing.T) {
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC)
	sevenDaysAgo := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create historical snapshot
	oldActivity := diff.UserActivity{
		Username: "alice",
		StarredRepos: []diff.Repo{
			{Name: "old-repo", Owner: "alice", CreatedAt: sevenDaysAgo},
		},
	}
	oldSnapshot := &diff.Snapshot{
		CapturedAt: sevenDaysAgo,
		Users:      map[string]diff.UserActivity{"alice": oldActivity},
	}

	// Create current snapshot
	newActivity := diff.UserActivity{
		Username: "alice",
		StarredRepos: []diff.Repo{
			{Name: "old-repo", Owner: "alice", CreatedAt: sevenDaysAgo},
			{Name: "new-repo", Owner: "alice", CreatedAt: now},
		},
	}
	newSnapshot := &diff.Snapshot{
		CapturedAt: now,
		Users:      map[string]diff.UserActivity{"alice": newActivity},
	}

	// Convert to storage format
	oldSS, _ := snapshotToStorage(oldSnapshot)
	newSS, _ := snapshotToStorage(newSnapshot)

	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{oldSS, newSS},
	}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			return &mockGitHubClient{}
		},
		StoreFactory: func(dbPath string) (Store, error) {
			return mockStoreInst, nil
		},
		NotifierFactory: func() Notifier {
			return &mockNotifier{}
		},
		ReportGenerator: func() (ReportGenerator, error) {
			return &mockReportGenerator{}, nil
		},
		OpenBrowser: func(url string) error { return nil },
		Now:         func() time.Time { return now },
	}

	var stdout, stderr bytes.Buffer
	args := []string{"-since", "7d", "-token", "test-token", "-no-notify", "-no-open", "-v"}

	exitCode := run(&stdout, &stderr, args, deps)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Historical mode") {
		t.Errorf("expected 'Historical mode' in output, got: %s", output)
	}
}

func TestRun_OfflineMode(t *testing.T) {
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC)
	sevenDaysAgo := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create historical snapshot
	oldActivity := diff.UserActivity{
		Username: "alice",
		StarredRepos: []diff.Repo{
			{Name: "old-repo", Owner: "alice", CreatedAt: sevenDaysAgo},
		},
	}
	oldSnapshot := &diff.Snapshot{
		CapturedAt: sevenDaysAgo,
		Users:      map[string]diff.UserActivity{"alice": oldActivity},
	}

	// Create current snapshot (most recent cached)
	newActivity := diff.UserActivity{
		Username: "alice",
		StarredRepos: []diff.Repo{
			{Name: "old-repo", Owner: "alice", CreatedAt: sevenDaysAgo},
			{Name: "new-repo", Owner: "alice", CreatedAt: now},
		},
	}
	newSnapshot := &diff.Snapshot{
		CapturedAt: now,
		Users:      map[string]diff.UserActivity{"alice": newActivity},
	}

	// Convert to storage format
	oldSS, _ := snapshotToStorage(oldSnapshot)
	oldSS.Timestamp = sevenDaysAgo
	newSS, _ := snapshotToStorage(newSnapshot)
	newSS.Timestamp = now

	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{newSS, oldSS}, // Most recent first
	}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			t.Error("should not call GitHub API in offline mode")
			return &mockGitHubClient{}
		},
		StoreFactory: func(dbPath string) (Store, error) {
			return mockStoreInst, nil
		},
		NotifierFactory: func() Notifier {
			return &mockNotifier{}
		},
		ReportGenerator: func() (ReportGenerator, error) {
			return &mockReportGenerator{}, nil
		},
		OpenBrowser: func(url string) error { return nil },
		Now:         func() time.Time { return now },
	}

	var stdout, stderr bytes.Buffer
	args := []string{"-since", "7d", "-offline", "-no-notify", "-no-open", "-v"}

	exitCode := run(&stdout, &stderr, args, deps)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Historical mode") {
		t.Errorf("expected 'Historical mode' in output, got: %s", output)
	}
	if !strings.Contains(output, "Using cached snapshot") {
		t.Errorf("expected 'Using cached snapshot' in output, got: %s", output)
	}
}

// TestRun_OfflineWithoutSince - --offline can be used standalone or with --since
// Standalone --offline mode uses cached data for a quick report without GitHub API calls
// This test is removed as --offline is now supported both with and without --since

func TestRun_BrowserError_DoesNotFail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockClient := &mockGitHubClient{
		followedUsers: []github.User{{Login: "testuser"}},
		starredRepos: map[string][]github.Repository{
			"testuser": {{Name: "repo", Owner: github.User{Login: "owner"}}},
		},
	}

	mockStoreInst := &mockStore{snapshots: []*storage.Snapshot{}}
	mockGenInst := &mockReportGenerator{}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient { return mockClient },
		StoreFactory:        func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory:     func() Notifier { return &mockNotifier{} },
		ReportGenerator:     func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:         func(url string) error { return errors.New("browser failed") },
		Now:                 fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-token", "test-token",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-report", filepath.Join(tmpDir, "report.html"),
		"-no-notify",
	}, deps)

	// Should still succeed even if browser fails
	if result != 0 {
		t.Errorf("expected exit code 0 despite browser error, got %d", result)
	}

	if !strings.Contains(stderr.String(), "could not open browser") {
		t.Errorf("expected warning about browser, got: %s", stderr.String())
	}
}

func TestRun_OfflineMode_WithCachedData(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	// Create a cached snapshot
	cachedSnapshot := diff.NewSnapshot(fixedTime().Add(-24 * time.Hour))
	cachedSnapshot.Users["testuser"] = diff.UserActivity{
		Username: "testuser",
		StarredRepos: []diff.Repo{
			{Owner: "owner1", Name: "cached-repo", Description: "A cached repo"},
		},
	}

	ss, _ := snapshotToStorage(cachedSnapshot)
	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{ss},
	}

	mockGenInst := &mockReportGenerator{}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			t.Error("GitHubClient should not be created in offline mode")
			return nil
		},
		StoreFactory:    func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory: func() Notifier { return &mockNotifier{} },
		ReportGenerator: func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:     func(url string) error { return nil },
		Now:             fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-offline",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-report", filepath.Join(tmpDir, "report.html"),
		"-no-notify",
		"-no-open",
	}, deps)

	if result != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", result, stderr.String())
	}

	// Check that we got a warning about cached data
	if !strings.Contains(stdout.String(), "Using cached data") {
		t.Errorf("expected 'Using cached data' message, got: %s", stdout.String())
	}

	if !strings.Contains(stdout.String(), "may be stale") {
		t.Errorf("expected 'may be stale' warning, got: %s", stdout.String())
	}

	// Verify that the snapshot was NOT saved (offline mode should not save)
	if mockStoreInst.savedCalled {
		t.Error("expected snapshot not to be saved in offline mode")
	}

	// Verify report was generated
	if mockGenInst.generatedReport == nil {
		t.Error("expected report to be generated")
	}
}

func TestRun_OfflineMode_NoCachedData(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{}, // No cached data
	}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			t.Error("GitHubClient should not be created in offline mode")
			return nil
		},
		StoreFactory:    func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory: func() Notifier { return &mockNotifier{} },
		ReportGenerator: func() (ReportGenerator, error) { return &mockReportGenerator{}, nil },
		OpenBrowser:     func(url string) error { return nil },
		Now:             fixedTime,
	}

	result := run(&stdout, &stderr, []string{
		"-offline",
		"-db", filepath.Join(tmpDir, "test.db"),
	}, deps)

	if result != 1 {
		t.Errorf("expected exit code 1, got %d", result)
	}

	// Check that we got an error about missing cached data
	if !strings.Contains(stderr.String(), "No cached data available") {
		t.Errorf("expected error about no cached data, got: %s", stderr.String())
	}

	if !strings.Contains(stderr.String(), "Run without --offline first") {
		t.Errorf("expected suggestion to run without --offline, got: %s", stderr.String())
	}
}

func TestRun_OfflineMode_NoTokenRequired(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()

	// Create a cached snapshot
	cachedSnapshot := diff.NewSnapshot(fixedTime().Add(-24 * time.Hour))
	cachedSnapshot.Users["testuser"] = diff.UserActivity{
		Username: "testuser",
		StarredRepos: []diff.Repo{
			{Owner: "owner1", Name: "cached-repo"},
		},
	}

	ss, _ := snapshotToStorage(cachedSnapshot)
	mockStoreInst := &mockStore{
		snapshots: []*storage.Snapshot{ss},
	}

	mockGenInst := &mockReportGenerator{}

	deps := &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			t.Error("GitHubClient should not be created in offline mode")
			return nil
		},
		StoreFactory:    func(dbPath string) (Store, error) { return mockStoreInst, nil },
		NotifierFactory: func() Notifier { return &mockNotifier{} },
		ReportGenerator: func() (ReportGenerator, error) { return mockGenInst, nil },
		OpenBrowser:     func(url string) error { return nil },
		Now:             fixedTime,
	}

	// Run without -token flag and without GITHUB_TOKEN env var
	result := run(&stdout, &stderr, []string{
		"-offline",
		"-db", filepath.Join(tmpDir, "test.db"),
		"-report", filepath.Join(tmpDir, "report.html"),
		"-no-notify",
		"-no-open",
	}, deps)

	if result != 0 {
		t.Errorf("expected exit code 0 in offline mode without token, got %d. stderr: %s", result, stderr.String())
	}

	// Should not have error about missing token
	if strings.Contains(stderr.String(), "GITHUB_TOKEN") {
		t.Errorf("should not require GITHUB_TOKEN in offline mode, got: %s", stderr.String())
	}
}

func TestFilterResultBySinceDate(t *testing.T) {
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC)
	sinceDate := time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC)
	oldDate := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // 3 weeks ago

	result := &diff.Result{
		OldCapturedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NewCapturedAt: now,
		NewStars: []diff.RepoChange{
			{
				Username: "simonw",
				Repo: diff.Repo{
					Owner:     "simonw",
					Name:      "old-repo",
					CreatedAt: oldDate, // Should be filtered out
				},
			},
			{
				Username: "octocat",
				Repo: diff.Repo{
					Owner:     "octocat",
					Name:      "new-repo",
					CreatedAt: sinceDate.Add(time.Hour), // Should be included
				},
			},
		},
		NewRepos: []diff.RepoChange{
			{
				Username: "user1",
				Repo: diff.Repo{
					Owner:     "user1",
					Name:      "ancient-repo",
					CreatedAt: oldDate, // Should be filtered out
				},
			},
			{
				Username: "user2",
				Repo: diff.Repo{
					Owner:     "user2",
					Name:      "recent-repo",
					CreatedAt: now, // Should be included
				},
			},
		},
		NewEvents: []diff.EventChange{
			{
				Username: "simonw",
				Event: diff.Event{
					Type:      "PushEvent",
					Actor:     "simonw",
					Repo:      "simonw/old-project",
					CreatedAt: oldDate, // Should be filtered out
				},
			},
			{
				Username: "octocat",
				Event: diff.Event{
					Type:      "PushEvent",
					Actor:     "octocat",
					Repo:      "octocat/fresh-project",
					CreatedAt: sinceDate, // Exactly on since date - should be included
				},
			},
		},
		NewUsers:  []string{"newuser1", "newuser2"},
		GoneUsers: []string{"goneuser1"},
	}

	filtered := filterResultBySinceDate(result, sinceDate)

	// Check that timestamps are preserved
	if !filtered.OldCapturedAt.Equal(result.OldCapturedAt) {
		t.Errorf("OldCapturedAt mismatch: got %v, want %v", filtered.OldCapturedAt, result.OldCapturedAt)
	}
	if !filtered.NewCapturedAt.Equal(result.NewCapturedAt) {
		t.Errorf("NewCapturedAt mismatch: got %v, want %v", filtered.NewCapturedAt, result.NewCapturedAt)
	}

	// Check that user lists are preserved
	if len(filtered.NewUsers) != 2 || filtered.NewUsers[0] != "newuser1" {
		t.Errorf("NewUsers not preserved: got %v, want %v", filtered.NewUsers, result.NewUsers)
	}
	if len(filtered.GoneUsers) != 1 || filtered.GoneUsers[0] != "goneuser1" {
		t.Errorf("GoneUsers not preserved: got %v, want %v", filtered.GoneUsers, result.GoneUsers)
	}

	// Check that old stars are filtered out
	if len(filtered.NewStars) != 1 {
		t.Fatalf("expected 1 new star, got %d", len(filtered.NewStars))
	}
	if filtered.NewStars[0].Username != "octocat" {
		t.Errorf("wrong star kept: got %s, want octocat", filtered.NewStars[0].Username)
	}

	// Check that old repos are filtered out
	if len(filtered.NewRepos) != 1 {
		t.Fatalf("expected 1 new repo, got %d", len(filtered.NewRepos))
	}
	if filtered.NewRepos[0].Username != "user2" {
		t.Errorf("wrong repo kept: got %s, want user2", filtered.NewRepos[0].Username)
	}

	// Check that old events are filtered out
	if len(filtered.NewEvents) != 1 {
		t.Fatalf("expected 1 new event, got %d", len(filtered.NewEvents))
	}
	if filtered.NewEvents[0].Username != "octocat" {
		t.Errorf("wrong event kept: got %s, want octocat", filtered.NewEvents[0].Username)
	}
}

func TestFilterResultBySinceDate_BoundaryConditions(t *testing.T) {
	sinceDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		createdAt     time.Time
		name          string
		shouldInclude bool
	}{
		{
			name:          "before since date",
			createdAt:     sinceDate.Add(-24 * time.Hour),
			shouldInclude: false,
		},
		{
			name:          "exactly on since date",
			createdAt:     sinceDate,
			shouldInclude: true,
		},
		{
			name:          "after since date",
			createdAt:     sinceDate.Add(24 * time.Hour),
			shouldInclude: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &diff.Result{
				NewStars: []diff.RepoChange{
					{
						Username: "user",
						Repo: diff.Repo{
							Owner:     "user",
							Name:      "repo",
							CreatedAt: tt.createdAt,
						},
					},
				},
			}

			filtered := filterResultBySinceDate(result, sinceDate)

			expectedCount := 0
			if tt.shouldInclude {
				expectedCount = 1
			}

			if len(filtered.NewStars) != expectedCount {
				t.Errorf("expected %d stars, got %d", expectedCount, len(filtered.NewStars))
			}
		})
	}
}
