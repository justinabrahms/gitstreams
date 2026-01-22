// Package diff compares GitHub activity snapshots to detect changes.
package diff

import "time"

// Repo represents a GitHub repository.
type Repo struct {
	CreatedAt   time.Time
	Owner       string
	Name        string
	Description string
	Language    string
	Stars       int
}

// FullName returns the owner/name format.
func (r Repo) FullName() string {
	return r.Owner + "/" + r.Name
}

// Event represents a GitHub activity event.
type Event struct {
	CreatedAt time.Time
	Type      string // e.g., "PushEvent", "CreateEvent", "ForkEvent"
	Actor     string // username who performed the event
	Repo      string // full repo name
}

// UserActivity represents a single user's GitHub activity at a point in time.
type UserActivity struct {
	Username     string
	StarredRepos []Repo
	OwnedRepos   []Repo
	Events       []Event
}

// Snapshot represents the state of all followed users' activity at a point in time.
type Snapshot struct {
	CapturedAt time.Time
	Users      map[string]UserActivity // keyed by username
}

// NewSnapshot creates an empty snapshot with the given timestamp.
func NewSnapshot(capturedAt time.Time) *Snapshot {
	return &Snapshot{
		CapturedAt: capturedAt,
		Users:      make(map[string]UserActivity),
	}
}

// RepoChange represents a change in starred or owned repos.
type RepoChange struct {
	Username string
	Repo     Repo
}

// EventChange represents new events detected.
type EventChange struct {
	Event    Event
	Username string
}

// Result contains all detected changes between two snapshots.
type Result struct {
	OldCapturedAt time.Time
	NewCapturedAt time.Time

	// New stars: repos that weren't starred before but are now
	NewStars []RepoChange

	// New repos: repos created by followed users
	NewRepos []RepoChange

	// New events: activity events not seen in the previous snapshot
	NewEvents []EventChange

	// New users: users that appear in new snapshot but not old
	NewUsers []string

	// Gone users: users that were in old snapshot but not new
	GoneUsers []string
}

// IsEmpty returns true if no changes were detected.
func (r *Result) IsEmpty() bool {
	return len(r.NewStars) == 0 &&
		len(r.NewRepos) == 0 &&
		len(r.NewEvents) == 0 &&
		len(r.NewUsers) == 0 &&
		len(r.GoneUsers) == 0
}

// Compare compares two snapshots and returns the detected changes.
// The old snapshot represents the previous state, new represents current state.
func Compare(old, new *Snapshot) *Result {
	result := &Result{
		OldCapturedAt: old.CapturedAt,
		NewCapturedAt: new.CapturedAt,
	}

	// Find new and gone users
	for username := range new.Users {
		if _, exists := old.Users[username]; !exists {
			result.NewUsers = append(result.NewUsers, username)
		}
	}
	for username := range old.Users {
		if _, exists := new.Users[username]; !exists {
			result.GoneUsers = append(result.GoneUsers, username)
		}
	}

	// Compare activity for users present in both snapshots
	for username, newActivity := range new.Users {
		oldActivity, exists := old.Users[username]
		if !exists {
			// New user - all their activity is "new"
			for _, repo := range newActivity.StarredRepos {
				result.NewStars = append(result.NewStars, RepoChange{
					Username: username,
					Repo:     repo,
				})
			}
			for _, repo := range newActivity.OwnedRepos {
				result.NewRepos = append(result.NewRepos, RepoChange{
					Username: username,
					Repo:     repo,
				})
			}
			for _, event := range newActivity.Events {
				result.NewEvents = append(result.NewEvents, EventChange{
					Username: username,
					Event:    event,
				})
			}
			continue
		}

		// Find new stars
		oldStars := repoSet(oldActivity.StarredRepos)
		for _, repo := range newActivity.StarredRepos {
			if !oldStars[repo.FullName()] {
				result.NewStars = append(result.NewStars, RepoChange{
					Username: username,
					Repo:     repo,
				})
			}
		}

		// Find new owned repos
		oldOwned := repoSet(oldActivity.OwnedRepos)
		for _, repo := range newActivity.OwnedRepos {
			if !oldOwned[repo.FullName()] {
				result.NewRepos = append(result.NewRepos, RepoChange{
					Username: username,
					Repo:     repo,
				})
			}
		}

		// Find new events (by type+repo+time combination)
		oldEvents := eventSet(oldActivity.Events)
		for _, event := range newActivity.Events {
			key := eventKey(event)
			if !oldEvents[key] {
				result.NewEvents = append(result.NewEvents, EventChange{
					Username: username,
					Event:    event,
				})
			}
		}
	}

	return result
}

// repoSet creates a set of repo full names for quick lookup.
func repoSet(repos []Repo) map[string]bool {
	set := make(map[string]bool, len(repos))
	for _, r := range repos {
		set[r.FullName()] = true
	}
	return set
}

// eventKey creates a unique key for an event.
func eventKey(e Event) string {
	return e.Type + "|" + e.Actor + "|" + e.Repo + "|" + e.CreatedAt.Format(time.RFC3339)
}

// eventSet creates a set of event keys for quick lookup.
func eventSet(events []Event) map[string]bool {
	set := make(map[string]bool, len(events))
	for _, e := range events {
		set[eventKey(e)] = true
	}
	return set
}
