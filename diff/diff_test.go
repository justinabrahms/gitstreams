package diff

import (
	"slices"
	"testing"
	"time"
)

func TestRepoFullName(t *testing.T) {
	r := Repo{Owner: "octocat", Name: "hello-world"}
	want := "octocat/hello-world"
	if got := r.FullName(); got != want {
		t.Errorf("FullName() = %q, want %q", got, want)
	}
}

func TestNewSnapshot(t *testing.T) {
	now := time.Now()
	s := NewSnapshot(now)
	if s.CapturedAt != now {
		t.Errorf("CapturedAt = %v, want %v", s.CapturedAt, now)
	}
	if s.Users == nil {
		t.Error("Users map is nil")
	}
}

func TestResultIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{
			name:   "empty result",
			result: Result{},
			want:   true,
		},
		{
			name:   "has new stars",
			result: Result{NewStars: []RepoChange{{}}},
			want:   false,
		},
		{
			name:   "has new repos",
			result: Result{NewRepos: []RepoChange{{}}},
			want:   false,
		},
		{
			name:   "has new events",
			result: Result{NewEvents: []EventChange{{}}},
			want:   false,
		},
		{
			name:   "has new users",
			result: Result{NewUsers: []string{"alice"}},
			want:   false,
		},
		{
			name:   "has gone users",
			result: Result{GoneUsers: []string{"bob"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareEmptySnapshots(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	result := Compare(old, new)
	if !result.IsEmpty() {
		t.Errorf("expected empty result for two empty snapshots")
	}
}

func TestCompareDetectsNewUser(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	new.Users["alice"] = UserActivity{Username: "alice"}

	result := Compare(old, new)

	if len(result.NewUsers) != 1 || result.NewUsers[0] != "alice" {
		t.Errorf("NewUsers = %v, want [alice]", result.NewUsers)
	}
}

func TestCompareDetectsGoneUser(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	old.Users["bob"] = UserActivity{Username: "bob"}

	result := Compare(old, new)

	if len(result.GoneUsers) != 1 || result.GoneUsers[0] != "bob" {
		t.Errorf("GoneUsers = %v, want [bob]", result.GoneUsers)
	}
}

func TestCompareDetectsNewStars(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	existingRepo := Repo{Owner: "existing", Name: "repo"}
	newRepo := Repo{Owner: "cool", Name: "project", Language: "Go"}

	old.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{existingRepo},
	}
	new.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{existingRepo, newRepo},
	}

	result := Compare(old, new)

	if len(result.NewStars) != 1 {
		t.Fatalf("NewStars length = %d, want 1", len(result.NewStars))
	}
	if result.NewStars[0].Repo.FullName() != "cool/project" {
		t.Errorf("NewStars[0].Repo = %v, want cool/project", result.NewStars[0].Repo.FullName())
	}
	if result.NewStars[0].Username != "alice" {
		t.Errorf("NewStars[0].Username = %q, want alice", result.NewStars[0].Username)
	}
}

func TestCompareDetectsNewRepos(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	existingRepo := Repo{Owner: "alice", Name: "old-project"}
	newRepo := Repo{Owner: "alice", Name: "new-project", CreatedAt: time.Now()}

	old.Users["alice"] = UserActivity{
		Username:   "alice",
		OwnedRepos: []Repo{existingRepo},
	}
	new.Users["alice"] = UserActivity{
		Username:   "alice",
		OwnedRepos: []Repo{existingRepo, newRepo},
	}

	result := Compare(old, new)

	if len(result.NewRepos) != 1 {
		t.Fatalf("NewRepos length = %d, want 1", len(result.NewRepos))
	}
	if result.NewRepos[0].Repo.FullName() != "alice/new-project" {
		t.Errorf("NewRepos[0].Repo = %v, want alice/new-project", result.NewRepos[0].Repo.FullName())
	}
}

func TestCompareDetectsNewEvents(t *testing.T) {
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	old := NewSnapshot(baseTime.Add(-24 * time.Hour))
	new := NewSnapshot(baseTime)

	oldEvent := Event{
		Type:      "PushEvent",
		Actor:     "alice",
		Repo:      "alice/project",
		CreatedAt: baseTime.Add(-48 * time.Hour),
	}
	newEvent := Event{
		Type:      "CreateEvent",
		Actor:     "alice",
		Repo:      "alice/new-repo",
		CreatedAt: baseTime.Add(-12 * time.Hour),
	}

	old.Users["alice"] = UserActivity{
		Username: "alice",
		Events:   []Event{oldEvent},
	}
	new.Users["alice"] = UserActivity{
		Username: "alice",
		Events:   []Event{oldEvent, newEvent},
	}

	result := Compare(old, new)

	if len(result.NewEvents) != 1 {
		t.Fatalf("NewEvents length = %d, want 1", len(result.NewEvents))
	}
	if result.NewEvents[0].Event.Type != "CreateEvent" {
		t.Errorf("NewEvents[0].Event.Type = %q, want CreateEvent", result.NewEvents[0].Event.Type)
	}
}

func TestCompareNewUserAllActivityIsNew(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	repo := Repo{Owner: "newuser", Name: "project"}
	event := Event{Type: "PushEvent", Actor: "newuser", Repo: "newuser/project", CreatedAt: time.Now()}

	new.Users["newuser"] = UserActivity{
		Username:     "newuser",
		StarredRepos: []Repo{{Owner: "other", Name: "cool-lib"}},
		OwnedRepos:   []Repo{repo},
		Events:       []Event{event},
	}

	result := Compare(old, new)

	if len(result.NewUsers) != 1 {
		t.Errorf("NewUsers = %v, want [newuser]", result.NewUsers)
	}
	if len(result.NewStars) != 1 {
		t.Errorf("NewStars length = %d, want 1", len(result.NewStars))
	}
	if len(result.NewRepos) != 1 {
		t.Errorf("NewRepos length = %d, want 1", len(result.NewRepos))
	}
	if len(result.NewEvents) != 1 {
		t.Errorf("NewEvents length = %d, want 1", len(result.NewEvents))
	}
}

func TestCompareNoChangesForIdenticalSnapshots(t *testing.T) {
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	repo := Repo{Owner: "alice", Name: "project"}
	event := Event{Type: "PushEvent", Actor: "alice", Repo: "alice/project", CreatedAt: baseTime}

	old := NewSnapshot(baseTime.Add(-24 * time.Hour))
	old.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{repo},
		OwnedRepos:   []Repo{repo},
		Events:       []Event{event},
	}

	new := NewSnapshot(baseTime)
	new.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{repo},
		OwnedRepos:   []Repo{repo},
		Events:       []Event{event},
	}

	result := Compare(old, new)

	if !result.IsEmpty() {
		t.Errorf("expected empty result for identical snapshots, got: NewStars=%d, NewRepos=%d, NewEvents=%d",
			len(result.NewStars), len(result.NewRepos), len(result.NewEvents))
	}
}

func TestCompareMultipleUsers(t *testing.T) {
	old := NewSnapshot(time.Now().Add(-24 * time.Hour))
	new := NewSnapshot(time.Now())

	// Alice: existing user with new star
	aliceOldStar := Repo{Owner: "old", Name: "repo"}
	aliceNewStar := Repo{Owner: "new", Name: "repo"}
	old.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{aliceOldStar},
	}
	new.Users["alice"] = UserActivity{
		Username:     "alice",
		StarredRepos: []Repo{aliceOldStar, aliceNewStar},
	}

	// Bob: existing user with no changes
	bobRepo := Repo{Owner: "bob", Name: "stable"}
	old.Users["bob"] = UserActivity{
		Username:   "bob",
		OwnedRepos: []Repo{bobRepo},
	}
	new.Users["bob"] = UserActivity{
		Username:   "bob",
		OwnedRepos: []Repo{bobRepo},
	}

	// Carol: new user
	new.Users["carol"] = UserActivity{
		Username:     "carol",
		StarredRepos: []Repo{{Owner: "popular", Name: "lib"}},
	}

	result := Compare(old, new)

	// Alice's new star
	aliceStars := 0
	for _, s := range result.NewStars {
		if s.Username == "alice" {
			aliceStars++
		}
	}
	if aliceStars != 1 {
		t.Errorf("alice new stars = %d, want 1", aliceStars)
	}

	// Carol as new user (with her star)
	carolStars := 0
	for _, s := range result.NewStars {
		if s.Username == "carol" {
			carolStars++
		}
	}
	if carolStars != 1 {
		t.Errorf("carol new stars = %d, want 1", carolStars)
	}

	// Carol in new users
	if !slices.Contains(result.NewUsers, "carol") {
		t.Error("carol not found in NewUsers")
	}
}

func TestCompareTimestamps(t *testing.T) {
	oldTime := time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	old := NewSnapshot(oldTime)
	new := NewSnapshot(newTime)

	result := Compare(old, new)

	if result.OldCapturedAt != oldTime {
		t.Errorf("OldCapturedAt = %v, want %v", result.OldCapturedAt, oldTime)
	}
	if result.NewCapturedAt != newTime {
		t.Errorf("NewCapturedAt = %v, want %v", result.NewCapturedAt, newTime)
	}
}
