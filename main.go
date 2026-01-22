// Package main provides the CLI entry point for gitstreams.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/justinabrahms/gitstreams/diff"
	"github.com/justinabrahms/gitstreams/github"
	"github.com/justinabrahms/gitstreams/notify"
	"github.com/justinabrahms/gitstreams/progress"
	"github.com/justinabrahms/gitstreams/report"
	"github.com/justinabrahms/gitstreams/storage"
)

const (
	defaultDBName   = "gitstreams.db"
	snapshotUserID  = "followed_users"
	activityDataKey = "snapshot_data"
)

// Version info set via ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Config holds the runtime configuration for gitstreams.
type Config struct {
	DBPath     string
	Token      string
	ReportPath string
	Since      string // Generate report from this date (e.g., '2026-01-15' or '7d')
	Days       int    // How far back to fetch GitHub data (API sync lookback, default 30)
	NoNotify   bool
	NoOpen     bool
	Verbose    bool
	Offline    bool // Use only cached data, skip GitHub API calls
}

// Dependencies holds injectable dependencies for testing.
type Dependencies struct {
	GitHubClientFactory func(token string) GitHubClient
	StoreFactory        func(dbPath string) (Store, error)
	NotifierFactory     func() Notifier
	ReportGenerator     func() (ReportGenerator, error)
	OpenBrowser         func(url string) error
	Now                 func() time.Time
}

// GitHubClient defines the GitHub API operations we need.
type GitHubClient interface {
	GetFollowedUsers(ctx context.Context) ([]github.User, error)
	GetStarredReposByUsername(ctx context.Context, username string) ([]github.Repository, error)
	GetOwnedReposByUsername(ctx context.Context, username string) ([]github.Repository, error)
	GetRecentEvents(ctx context.Context, username string) ([]github.Event, error)
}

// Store defines the storage operations we need.
type Store interface {
	Save(snapshot *storage.Snapshot) error
	GetByUser(userID string, limit int) ([]*storage.Snapshot, error)
	GetByTimeRange(userID string, start, end time.Time) ([]*storage.Snapshot, error)
	Close() error
}

// Notifier defines notification operations.
type Notifier interface {
	Send(n notify.Notification) error
}

// ReportGenerator defines report generation operations.
type ReportGenerator interface {
	Generate(w io.Writer, r *report.Report) error
}

// DefaultDependencies returns production dependencies.
func DefaultDependencies() *Dependencies {
	return &Dependencies{
		GitHubClientFactory: func(token string) GitHubClient {
			return github.NewClient(token)
		},
		StoreFactory: func(dbPath string) (Store, error) {
			return storage.NewSQLiteStore(dbPath)
		},
		NotifierFactory: func() Notifier {
			return notify.NewMacNotifier()
		},
		ReportGenerator: func() (ReportGenerator, error) {
			return report.NewHTMLGenerator()
		},
		OpenBrowser: openBrowser,
		Now:         time.Now,
	}
}

func main() {
	// Handle "version" subcommand before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("gitstreams %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:], DefaultDependencies()))
}

func run(stdout, stderr io.Writer, args []string, deps *Dependencies) int {
	cfg, err := parseFlags(args)
	if err != nil {
		if err == errVersion {
			_, _ = fmt.Fprintf(stdout, "gitstreams %s (commit: %s, built: %s)\n", version, commit, date)
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// Open storage first to support both live and historical modes
	store, err := deps.StoreFactory(cfg.DBPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error opening database: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	var currentSnapshot, previousSnapshot *diff.Snapshot

	// Historical mode: generate report from cached data
	if cfg.Since != "" {
		var sinceDate time.Time
		sinceDate, err = parseSinceDate(cfg.Since, deps.Now())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error parsing --since date: %v\n", err)
			return 1
		}

		if cfg.Verbose {
			_, _ = fmt.Fprintf(stdout, "Historical mode: comparing data from %s to present\n", sinceDate.Format("2006-01-02"))
		}

		// Get snapshot from the "since" date
		var sinceSnapshots []*storage.Snapshot
		sinceSnapshots, err = store.GetByTimeRange(snapshotUserID, sinceDate.Add(-24*time.Hour), sinceDate.Add(24*time.Hour))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error querying snapshots for --since date: %v\n", err)
			return 1
		}
		if len(sinceSnapshots) == 0 {
			_, _ = fmt.Fprintf(stderr, "No cached snapshot found for date %s (try running without --since first to build cache)\n", sinceDate.Format("2006-01-02"))
			return 1
		}
		previousSnapshot, err = storageToSnapshot(sinceSnapshots[0])
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error loading historical snapshot: %v\n", err)
			return 1
		}

		// Get current snapshot: either from cache (offline) or from GitHub (live)
		if cfg.Offline {
			// Use most recent cached snapshot
			var recentSnapshots []*storage.Snapshot
			recentSnapshots, err = store.GetByUser(snapshotUserID, 1)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "Error loading most recent snapshot: %v\n", err)
				return 1
			}
			if len(recentSnapshots) == 0 {
				_, _ = fmt.Fprintf(stderr, "No cached snapshots available (run without --offline first)\n")
				return 1
			}
			currentSnapshot, err = storageToSnapshot(recentSnapshots[0])
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "Error loading current snapshot: %v\n", err)
				return 1
			}
			if cfg.Verbose {
				_, _ = fmt.Fprintf(stdout, "Using cached snapshot from %s\n", currentSnapshot.CapturedAt.Format("2006-01-02"))
			}
		} else {
			// Fetch fresh data from GitHub
			if cfg.Token == "" {
				_, _ = fmt.Fprintln(stderr, "Error: GITHUB_TOKEN environment variable is required (or use --offline)")
				return 1
			}
			ctx := context.Background()
			client := deps.GitHubClientFactory(cfg.Token)
			cutoff := deps.Now().AddDate(0, 0, -cfg.Days)
			currentSnapshot, err = fetchActivity(ctx, client, deps.Now(), cutoff, stdout, stderr, cfg.Verbose)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "Error fetching activity: %v\n", err)
				return 1
			}
			if cfg.Verbose {
				_, _ = fmt.Fprintf(stdout, "Fetched activity for %d users\n", len(currentSnapshot.Users))
			}
			// Don't save in historical mode
		}
	} else if cfg.Offline {
		// Standalone offline mode: use cached data without historical comparison
		var snapshots []*storage.Snapshot
		snapshots, err = store.GetByUser(snapshotUserID, 1)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error loading cached snapshot: %v\n", err)
			return 1
		}

		if len(snapshots) == 0 {
			_, _ = fmt.Fprintln(stderr, "Error: No cached data available. Run without --offline first to fetch data from GitHub.")
			return 1
		}

		currentSnapshot, err = storageToSnapshot(snapshots[0])
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error loading cached snapshot: %v\n", err)
			return 1
		}

		_, _ = fmt.Fprintf(stdout, "Using cached data from %s (may be stale)\n", currentSnapshot.CapturedAt.Format("2006-01-02 15:04:05"))

		if cfg.Verbose {
			_, _ = fmt.Fprintf(stdout, "Loaded cached activity for %d users\n", len(currentSnapshot.Users))
		}

		// In standalone offline mode, use an empty snapshot as "previous" so the report shows all activities in the cached snapshot
		previousSnapshot = diff.NewSnapshot(time.Time{})
	} else {
		// Normal mode: fetch current data and compare with most recent snapshot
		if cfg.Token == "" {
			_, _ = fmt.Fprintln(stderr, "Error: GITHUB_TOKEN environment variable is required")
			return 1
		}

		ctx := context.Background()
		client := deps.GitHubClientFactory(cfg.Token)
		cutoff := deps.Now().AddDate(0, 0, -cfg.Days)
		currentSnapshot, err = fetchActivity(ctx, client, deps.Now(), cutoff, stdout, stderr, cfg.Verbose)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error fetching activity: %v\n", err)
			return 1
		}

		if cfg.Verbose {
			_, _ = fmt.Fprintf(stdout, "Fetched activity for %d users\n", len(currentSnapshot.Users))
		}

		previousSnapshot, err = loadPreviousSnapshot(store)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error loading previous snapshot: %v\n", err)
			return 1
		}

		// Save current snapshot
		if saveErr := saveSnapshot(store, currentSnapshot, deps.Now()); saveErr != nil {
			_, _ = fmt.Fprintf(stderr, "Error saving snapshot: %v\n", saveErr)
			return 1
		}

		if cfg.Verbose {
			_, _ = fmt.Fprintln(stdout, "Saved current snapshot")
		}
	}

	// Compare snapshots
	if cfg.Verbose {
		_, _ = fmt.Fprintf(stdout, "Previous snapshot has %d users, current snapshot has %d users\n",
			len(previousSnapshot.Users), len(currentSnapshot.Users))
	}

	result := diff.Compare(previousSnapshot, currentSnapshot)

	// Filter results by since date if specified
	// This is needed because snapshots contain historical data (e.g., 30 days),
	// so we need to filter out activities that occurred before the since date
	if cfg.Since != "" {
		var filterDate time.Time
		filterDate, err = parseSinceDate(cfg.Since, deps.Now())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error parsing --since date for filtering: %v\n", err)
			return 1
		}
		result = filterResultBySinceDate(result, filterDate)
		if cfg.Verbose {
			_, _ = fmt.Fprintf(stdout, "Filtered results to only show activity from %s onwards\n", filterDate.Format("2006-01-02"))
		}
	}

	if cfg.Verbose {
		_, _ = fmt.Fprintf(stdout, "Diff result: NewStars=%d, NewRepos=%d, NewEvents=%d, NewUsers=%d, GoneUsers=%d\n",
			len(result.NewStars), len(result.NewRepos), len(result.NewEvents), len(result.NewUsers), len(result.GoneUsers))
	}

	if result.IsEmpty() {
		_, _ = fmt.Fprintln(stdout, "No new activity detected.")
		return 0
	}

	// Generate report
	rpt := buildReportWithLogging(result, previousSnapshot.CapturedAt, currentSnapshot.CapturedAt, deps.Now(), stderr, cfg.Verbose)

	reportPath := cfg.ReportPath
	if reportPath == "" {
		reportPath = filepath.Join(os.TempDir(), fmt.Sprintf("gitstreams-%s.html", deps.Now().Format("2006-01-02")))
	}

	generator, err := deps.ReportGenerator()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error creating report generator: %v\n", err)
		return 1
	}

	f, err := os.Create(reportPath) // #nosec G304 -- reportPath is user-specified via flag or safe default
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error creating report file: %v\n", err)
		return 1
	}

	if err := generator.Generate(f, rpt); err != nil {
		_ = f.Close()
		_, _ = fmt.Fprintf(stderr, "Error generating report: %v\n", err)
		return 1
	}
	_ = f.Close()

	_, _ = fmt.Fprintf(stdout, "Report written to %s\n", reportPath)

	// Send notification
	if !cfg.NoNotify {
		notifier := deps.NotifierFactory()
		n := notify.Notification{
			Title:    "GitStreams",
			Message:  formatNotificationMessage(result),
			Subtitle: "Activity from people you follow",
			Sound:    "default",
			OpenURL:  "file://" + reportPath,
		}
		if err := notifier.Send(n); err != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: could not send notification: %v\n", err)
			// Don't fail on notification errors
		}
	}

	// Open report in browser
	if !cfg.NoOpen {
		if err := deps.OpenBrowser("file://" + reportPath); err != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: could not open browser: %v\n", err)
			// Don't fail on browser errors
		}
	}

	return 0
}

// errVersion is a sentinel error indicating -version was requested.
var errVersion = fmt.Errorf("version requested")

func parseFlags(args []string) (*Config, error) {
	cfg := &Config{}
	var showVersion bool

	fs := flag.NewFlagSet("gitstreams", flag.ContinueOnError)
	fs.StringVar(&cfg.DBPath, "db", "", "Path to SQLite database (default: ~/.gitstreams/gitstreams.db)")
	fs.StringVar(&cfg.Token, "token", "", "GitHub token (default: $GITHUB_TOKEN)")
	fs.BoolVar(&cfg.NoNotify, "no-notify", false, "Skip desktop notification")
	fs.BoolVar(&cfg.NoOpen, "no-open", false, "Don't open report in browser")
	fs.StringVar(&cfg.ReportPath, "report", "", "Path to write HTML report (default: temp file)")
	fs.BoolVar(&cfg.Verbose, "v", false, "Verbose output")
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.IntVar(&cfg.Days, "sync-lookback-days", 30, "How far back to fetch GitHub data (1-365 days, doesn't affect report filtering)")
	fs.StringVar(&cfg.Since, "since", "", "Generate report from historical data (e.g., '2026-01-15' or '7d' for 7 days ago)")
	fs.BoolVar(&cfg.Offline, "offline", false, "Use only cached data, skip GitHub API calls")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if showVersion {
		return nil, errVersion
	}

	// Validate days parameter
	if cfg.Days < 1 || cfg.Days > 365 {
		return nil, fmt.Errorf("days must be between 1 and 365, got %d", cfg.Days)
	}

	// Validate since and offline flags
	// Note: --since without --offline is allowed, but requires token for current data
	// Note: --offline without --since is allowed for standalone cached mode

	// Default token from environment
	if cfg.Token == "" {
		cfg.Token = os.Getenv("GITHUB_TOKEN")
	}

	// Default database path
	if cfg.DBPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		dataDir := filepath.Join(home, ".gitstreams")
		if err := os.MkdirAll(dataDir, 0750); err != nil {
			return nil, fmt.Errorf("creating data directory: %w", err)
		}
		cfg.DBPath = filepath.Join(dataDir, defaultDBName)
	}

	return cfg, nil
}

// parseSinceDate parses a date string in various formats:
// - Absolute: '2026-01-15', '2026-01-15T10:30:00Z'
// - Relative: '7d' (7 days ago), '2w' (2 weeks ago), '3m' (3 months ago)
func parseSinceDate(dateStr string, now time.Time) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("date string is empty")
	}

	// Try parsing as relative time (e.g., '7d', '2w', '3m')
	if len(dateStr) >= 2 {
		unit := dateStr[len(dateStr)-1]
		valueStr := dateStr[:len(dateStr)-1]

		// Parse the numeric value
		var value int
		if _, err := fmt.Sscanf(valueStr, "%d", &value); err == nil && value > 0 {
			switch unit {
			case 'd':
				return now.AddDate(0, 0, -value), nil
			case 'w':
				return now.AddDate(0, 0, -value*7), nil
			case 'm':
				return now.AddDate(0, -value, 0), nil
			case 'y':
				return now.AddDate(-value, 0, 0), nil
			}
		}
	}

	// Try parsing as absolute date in various formats
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006/01/02",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q (try formats like '2026-01-15' or '7d')", dateStr)
}

func fetchActivity(ctx context.Context, client GitHubClient, now, cutoff time.Time, w, progressW io.Writer, verbose bool) (*diff.Snapshot, error) {
	users, err := client.GetFollowedUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching followed users: %w", err)
	}

	snapshot := diff.NewSnapshot(now)

	// Create progress tracker for stderr output
	prog := progress.NewProgress(progressW, len(users))
	if len(users) > 0 {
		prog.Start(fmt.Sprintf("Fetching activity for %d users...", len(users)))
	}

	for i, user := range users {
		// Update progress indicator (1-indexed for human-readable output)
		prog.SetItem(i+1, user.Login)

		if verbose {
			_, _ = fmt.Fprintf(w, "Fetching activity for %s...\n", user.Login)
		}

		activity := diff.UserActivity{
			Username: user.Login,
		}

		// Fetch starred repos - filter by repo creation date
		starred, err := client.GetStarredReposByUsername(ctx, user.Login)
		if err != nil {
			if verbose {
				_, _ = fmt.Fprintf(w, "  Warning: could not fetch starred repos for %s: %v\n", user.Login, err)
			}
		} else {
			for _, repo := range starred {
				// Only include repos created after the cutoff date
				if !repo.CreatedAt.Before(cutoff) {
					activity.StarredRepos = append(activity.StarredRepos, convertRepo(repo))
				}
			}
		}

		// Fetch owned repos - filter by creation or recent push date
		owned, err := client.GetOwnedReposByUsername(ctx, user.Login)
		if err != nil {
			if verbose {
				_, _ = fmt.Fprintf(w, "  Warning: could not fetch owned repos for %s: %v\n", user.Login, err)
			}
		} else {
			for _, repo := range owned {
				// Only include repos created after the cutoff date
				if !repo.CreatedAt.Before(cutoff) {
					activity.OwnedRepos = append(activity.OwnedRepos, convertRepo(repo))
				}
			}
		}

		// Fetch events - filter by event creation date
		events, err := client.GetRecentEvents(ctx, user.Login)
		if err != nil {
			if verbose {
				_, _ = fmt.Fprintf(w, "  Warning: could not fetch events for %s: %v\n", user.Login, err)
			}
		} else {
			for _, event := range events {
				// Only include events created after the cutoff date
				if !event.CreatedAt.Before(cutoff) {
					activity.Events = append(activity.Events, convertEvent(event))
				}
			}
		}

		snapshot.Users[user.Login] = activity
	}

	// Stop progress indicator
	prog.Done()

	return snapshot, nil
}

func convertRepo(r github.Repository) diff.Repo {
	return diff.Repo{
		CreatedAt:   r.CreatedAt,
		Owner:       r.Owner.Login,
		Name:        r.Name,
		Description: r.Description,
		Language:    r.Language,
		Stars:       r.StarCount,
	}
}

func convertEvent(e github.Event) diff.Event {
	return diff.Event{
		Type:      e.Type,
		Actor:     e.Actor.Login,
		Repo:      e.Repo.Name,
		CreatedAt: e.CreatedAt,
	}
}

func loadPreviousSnapshot(store Store) (*diff.Snapshot, error) {
	snapshots, err := store.GetByUser(snapshotUserID, 1)
	if err != nil {
		return nil, fmt.Errorf("loading snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		// No previous snapshot, return empty one
		return diff.NewSnapshot(time.Time{}), nil
	}

	return storageToSnapshot(snapshots[0])
}

func saveSnapshot(store Store, snapshot *diff.Snapshot, now time.Time) error {
	ss, err := snapshotToStorage(snapshot)
	if err != nil {
		return err
	}
	ss.Timestamp = now
	return store.Save(ss)
}

func snapshotToStorage(s *diff.Snapshot) (*storage.Snapshot, error) {
	// Serialize the diff.Snapshot to JSON-compatible map
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshaling snapshot: %w", err)
	}

	var activity map[string]interface{}
	if err := json.Unmarshal(data, &activity); err != nil {
		return nil, fmt.Errorf("unmarshaling to map: %w", err)
	}

	return &storage.Snapshot{
		UserID:    snapshotUserID,
		Timestamp: s.CapturedAt,
		Activity:  map[string]interface{}{activityDataKey: activity},
	}, nil
}

func storageToSnapshot(ss *storage.Snapshot) (*diff.Snapshot, error) {
	activityData, ok := ss.Activity[activityDataKey]
	if !ok {
		return diff.NewSnapshot(ss.Timestamp), nil
	}

	data, err := json.Marshal(activityData)
	if err != nil {
		return nil, fmt.Errorf("marshaling activity data: %w", err)
	}

	var snapshot diff.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
	}

	return &snapshot, nil
}

func buildReport(result *diff.Result, periodStart, periodEnd, generatedAt time.Time) *report.Report {
	return buildReportWithLogging(result, periodStart, periodEnd, generatedAt, nil, false)
}

func buildReportWithLogging(result *diff.Result, periodStart, periodEnd, generatedAt time.Time, w io.Writer, verbose bool) *report.Report {
	rpt := &report.Report{
		GeneratedAt: generatedAt,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	if verbose && w != nil {
		_, _ = fmt.Fprintf(w, "buildReport input: NewStars=%d, NewRepos=%d, NewEvents=%d, NewUsers=%d\n",
			len(result.NewStars), len(result.NewRepos), len(result.NewEvents), len(result.NewUsers))
	}

	// Group activities by user
	userActivities := make(map[string]*report.UserActivity)

	// Add new stars
	for _, star := range result.NewStars {
		ua := getOrCreateUserActivity(userActivities, star.Username)
		ua.Activities = append(ua.Activities, report.Activity{
			Type:      report.ActivityStarred,
			User:      star.Username,
			AvatarURL: ua.AvatarURL,
			RepoName:  star.Repo.FullName(),
			RepoURL:   fmt.Sprintf("https://github.com/%s", star.Repo.FullName()),
			Timestamp: star.Repo.CreatedAt,
			Details:   star.Repo.Description,
		})
	}

	if verbose && w != nil {
		_, _ = fmt.Fprintf(w, "buildReport after stars: userActivities map has %d entries\n", len(userActivities))
	}

	// Add new repos
	for _, repo := range result.NewRepos {
		ua := getOrCreateUserActivity(userActivities, repo.Username)
		ua.Activities = append(ua.Activities, report.Activity{
			Type:      report.ActivityCreatedRepo,
			User:      repo.Username,
			AvatarURL: ua.AvatarURL,
			RepoName:  repo.Repo.FullName(),
			RepoURL:   fmt.Sprintf("https://github.com/%s", repo.Repo.FullName()),
			Timestamp: repo.Repo.CreatedAt,
			Details:   repo.Repo.Description,
		})
	}

	if verbose && w != nil {
		_, _ = fmt.Fprintf(w, "buildReport after repos: userActivities map has %d entries\n", len(userActivities))
	}

	// Add new events
	for _, event := range result.NewEvents {
		ua := getOrCreateUserActivity(userActivities, event.Username)
		activityType := eventTypeToActivityType(event.Event.Type)
		ua.Activities = append(ua.Activities, report.Activity{
			Type:      activityType,
			User:      event.Username,
			AvatarURL: ua.AvatarURL,
			RepoName:  event.Event.Repo,
			RepoURL:   fmt.Sprintf("https://github.com/%s", event.Event.Repo),
			Timestamp: event.Event.CreatedAt,
		})
	}

	if verbose && w != nil {
		_, _ = fmt.Fprintf(w, "buildReport after events: userActivities map has %d entries\n", len(userActivities))
	}

	// Convert map to slice
	for _, ua := range userActivities {
		rpt.UserActivities = append(rpt.UserActivities, *ua)
	}

	if verbose && w != nil {
		_, _ = fmt.Fprintf(w, "buildReport output: UserActivities slice has %d entries\n", len(rpt.UserActivities))
	}

	return rpt
}

func getOrCreateUserActivity(m map[string]*report.UserActivity, username string) *report.UserActivity {
	if ua, ok := m[username]; ok {
		return ua
	}
	ua := &report.UserActivity{
		User:      username,
		AvatarURL: fmt.Sprintf("https://github.com/%s.png", username),
	}
	m[username] = ua
	return ua
}

func eventTypeToActivityType(eventType string) report.ActivityType {
	switch eventType {
	case "WatchEvent":
		return report.ActivityStarred
	case "CreateEvent":
		return report.ActivityCreatedRepo
	case "ForkEvent":
		return report.ActivityForked
	case "PushEvent":
		return report.ActivityPushed
	case "PullRequestEvent":
		return report.ActivityPR
	case "IssuesEvent":
		return report.ActivityIssue
	default:
		return report.ActivityType(eventType)
	}
}

// filterResultBySinceDate filters a diff result to only include activities created on or after the given date.
// This is necessary when using --since because snapshots contain historical data (e.g., 30 days),
// and we only want to show activities that occurred after the specified since date.
func filterResultBySinceDate(result *diff.Result, sinceDate time.Time) *diff.Result {
	filtered := &diff.Result{
		OldCapturedAt: result.OldCapturedAt,
		NewCapturedAt: result.NewCapturedAt,
		NewUsers:      result.NewUsers,
		GoneUsers:     result.GoneUsers,
	}

	// Filter new stars - only include repos created on or after since date
	for _, star := range result.NewStars {
		if !star.Repo.CreatedAt.Before(sinceDate) {
			filtered.NewStars = append(filtered.NewStars, star)
		}
	}

	// Filter new repos - only include repos created on or after since date
	for _, repo := range result.NewRepos {
		if !repo.Repo.CreatedAt.Before(sinceDate) {
			filtered.NewRepos = append(filtered.NewRepos, repo)
		}
	}

	// Filter new events - only include events created on or after since date
	for _, event := range result.NewEvents {
		if !event.Event.CreatedAt.Before(sinceDate) {
			filtered.NewEvents = append(filtered.NewEvents, event)
		}
	}

	return filtered
}

func formatNotificationMessage(result *diff.Result) string {
	parts := []string{}

	if n := len(result.NewStars); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new stars", n))
	}
	if n := len(result.NewRepos); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new repos", n))
	}
	if n := len(result.NewEvents); n > 0 {
		parts = append(parts, fmt.Sprintf("%d events", n))
	}
	if n := len(result.NewUsers); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new users", n))
	}

	if len(parts) == 0 {
		return "New activity detected"
	}

	msg := ""
	for i, p := range parts {
		if i > 0 {
			if i == len(parts)-1 {
				msg += " and "
			} else {
				msg += ", "
			}
		}
		msg += p
	}

	return msg
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start() // #nosec G204 -- cmd/args are platform-specific constants, not user input
}
