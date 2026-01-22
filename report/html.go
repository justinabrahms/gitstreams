// Package report provides HTML report generation for gitstreams.
package report

import (
	"fmt"
	"html/template"
	"io"
	"sort"
	"time"
)

// ActivityType represents the kind of activity event.
type ActivityType string

const (
	ActivityStarred     ActivityType = "starred"
	ActivityCreatedRepo ActivityType = "created_repo"
	ActivityForked      ActivityType = "forked"
	ActivityPushed      ActivityType = "pushed"
	ActivityPR          ActivityType = "pull_request"
	ActivityIssue       ActivityType = "issue"
)

// Activity represents a single activity event from a followed user.
type Activity struct {
	Type      ActivityType
	User      string
	AvatarURL string
	RepoName  string
	RepoURL   string
	Timestamp time.Time
	Details   string
}

// AggregatedActivity represents multiple similar activities grouped together.
type AggregatedActivity struct {
	FirstTime time.Time
	LastTime  time.Time
	User      string
	AvatarURL string
	RepoName  string
	RepoURL   string
	Details   string
	Type      ActivityType
	Count     int
}

// UserActivity groups activities by user.
type UserActivity struct {
	User       string
	AvatarURL  string
	Activities []Activity
}

// Report contains all the data needed to generate an HTML report.
type Report struct {
	GeneratedAt    time.Time
	PeriodStart    time.Time
	PeriodEnd      time.Time
	UserActivities []UserActivity
}

// TotalActivities returns the total number of activities in the report.
func (r *Report) TotalActivities() int {
	total := 0
	for _, ua := range r.UserActivities {
		total += len(ua.Activities)
	}
	return total
}

// CategoryGroup represents activities grouped by category.
type CategoryGroup struct {
	Type       ActivityType
	Activities []Activity
}

// AggregatedCategoryGroup represents aggregated activities grouped by category.
type AggregatedCategoryGroup struct {
	Type       ActivityType
	Activities []AggregatedActivity
}

// ActivitiesByCategory groups all activities by their type for category-based display.
func (r *Report) ActivitiesByCategory() []CategoryGroup {
	groups := make(map[ActivityType][]Activity)

	// Collect all activities by type
	for _, ua := range r.UserActivities {
		for _, a := range ua.Activities {
			groups[a.Type] = append(groups[a.Type], a)
		}
	}

	// Order categories in a sensible way
	order := []ActivityType{
		ActivityStarred,
		ActivityCreatedRepo,
		ActivityForked,
		ActivityPushed,
		ActivityPR,
		ActivityIssue,
	}

	var result []CategoryGroup
	for _, t := range order {
		if activities, ok := groups[t]; ok && len(activities) > 0 {
			result = append(result, CategoryGroup{
				Type:       t,
				Activities: activities,
			})
		}
	}

	return result
}

// aggregateKey returns a unique key for grouping similar activities.
func aggregateKey(a Activity) string {
	return fmt.Sprintf("%s|%s|%s", a.User, a.Type, a.RepoName)
}

// aggregateActivities groups similar activities by (user, type, repo).
func aggregateActivities(activities []Activity) []AggregatedActivity {
	if len(activities) == 0 {
		return nil
	}

	// Group activities by key
	groups := make(map[string][]Activity)
	order := make([]string, 0)

	for _, a := range activities {
		key := aggregateKey(a)
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], a)
	}

	// Convert groups to aggregated activities
	result := make([]AggregatedActivity, 0, len(order))
	for _, key := range order {
		group := groups[key]
		first := group[0]

		// Find time range
		firstTime := first.Timestamp
		lastTime := first.Timestamp
		for _, a := range group {
			if a.Timestamp.Before(firstTime) {
				firstTime = a.Timestamp
			}
			if a.Timestamp.After(lastTime) {
				lastTime = a.Timestamp
			}
		}

		result = append(result, AggregatedActivity{
			Type:      first.Type,
			User:      first.User,
			AvatarURL: first.AvatarURL,
			RepoName:  first.RepoName,
			RepoURL:   first.RepoURL,
			FirstTime: firstTime,
			LastTime:  lastTime,
			Count:     len(group),
			Details:   first.Details,
		})
	}

	return result
}

// AggregatedActivitiesByCategory returns aggregated activities grouped by category.
func (r *Report) AggregatedActivitiesByCategory() []AggregatedCategoryGroup {
	groups := make(map[ActivityType][]Activity)

	// Collect all activities by type
	for _, ua := range r.UserActivities {
		for _, a := range ua.Activities {
			groups[a.Type] = append(groups[a.Type], a)
		}
	}

	// Order categories in a sensible way
	order := []ActivityType{
		ActivityStarred,
		ActivityCreatedRepo,
		ActivityForked,
		ActivityPushed,
		ActivityPR,
		ActivityIssue,
	}

	var result []AggregatedCategoryGroup
	for _, t := range order {
		if activities, ok := groups[t]; ok && len(activities) > 0 {
			result = append(result, AggregatedCategoryGroup{
				Type:       t,
				Activities: aggregateActivities(activities),
			})
		}
	}

	return result
}

// AggregatedUserActivity holds a user's activities in aggregated form.
type AggregatedUserActivity struct {
	User       string
	AvatarURL  string
	Activities []AggregatedActivity
}

// AggregatedUserActivities returns user activities with similar events aggregated.
func (r *Report) AggregatedUserActivities() []AggregatedUserActivity {
	result := make([]AggregatedUserActivity, 0, len(r.UserActivities))
	for _, ua := range r.UserActivities {
		result = append(result, AggregatedUserActivity{
			User:       ua.User,
			AvatarURL:  ua.AvatarURL,
			Activities: aggregateActivities(ua.Activities),
		})
	}
	return result
}

// categoryName returns a human-readable name for the activity type category.
func categoryName(t ActivityType) string {
	switch t {
	case ActivityStarred:
		return "New Stars"
	case ActivityCreatedRepo:
		return "Repos Created"
	case ActivityForked:
		return "Forks"
	case ActivityPushed:
		return "Recent Pushes"
	case ActivityPR:
		return "Pull Requests"
	case ActivityIssue:
		return "Issues Opened"
	default:
		return "Other Activity"
	}
}

// ActivityStats holds counts by activity type.
type ActivityStats struct {
	Stars  int
	Repos  int
	Forks  int
	Pushes int
	PRs    int
	Issues int
}

// GetStats returns activity counts by type.
func (r *Report) GetStats() ActivityStats {
	stats := ActivityStats{}
	for _, ua := range r.UserActivities {
		for _, a := range ua.Activities {
			switch a.Type {
			case ActivityStarred:
				stats.Stars++
			case ActivityCreatedRepo:
				stats.Repos++
			case ActivityForked:
				stats.Forks++
			case ActivityPushed:
				stats.Pushes++
			case ActivityPR:
				stats.PRs++
			case ActivityIssue:
				stats.Issues++
			}
		}
	}
	return stats
}

// Highlight represents the most interesting activity to feature.
type Highlight struct {
	Activity  Activity
	User      string
	AvatarURL string
	Reason    string
}

// GetHighlight returns the most interesting activity to feature.
// Priority: new repos > PRs > stars > other.
func (r *Report) GetHighlight() *Highlight {
	var best *Highlight

	for _, ua := range r.UserActivities {
		for _, a := range ua.Activities {
			candidate := &Highlight{Activity: a, User: ua.User, AvatarURL: ua.AvatarURL}

			switch a.Type {
			case ActivityCreatedRepo:
				candidate.Reason = "🚀 Fresh off the press!"
				if best == nil || best.Activity.Type != ActivityCreatedRepo {
					best = candidate
				}
			case ActivityPR:
				candidate.Reason = "💪 Making things happen!"
				if best == nil || (best.Activity.Type != ActivityCreatedRepo && best.Activity.Type != ActivityPR) {
					best = candidate
				}
			case ActivityStarred:
				candidate.Reason = "👀 Spotted something cool!"
				if best == nil || (best.Activity.Type != ActivityCreatedRepo &&
					best.Activity.Type != ActivityPR &&
					best.Activity.Type != ActivityStarred) {
					best = candidate
				}
			default:
				if best == nil {
					candidate.Reason = "✨ Check this out!"
					best = candidate
				}
			}
		}
	}

	return best
}

// MostActiveUser returns the user with the most activities.
func (r *Report) MostActiveUser() string {
	if len(r.UserActivities) == 0 {
		return ""
	}

	sorted := make([]UserActivity, len(r.UserActivities))
	copy(sorted, r.UserActivities)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Activities) > len(sorted[j].Activities)
	})

	return sorted[0].User
}

// IsHotActivity returns true if this activity type is considered "hot" (high engagement).
func IsHotActivity(t ActivityType) bool {
	return t == ActivityCreatedRepo || t == ActivityPR
}

// activityIcon returns an emoji icon for the activity type.
func activityIcon(t ActivityType) string {
	switch t {
	case ActivityStarred:
		return "⭐"
	case ActivityCreatedRepo:
		return "🆕"
	case ActivityForked:
		return "🔱"
	case ActivityPushed:
		return "📤"
	case ActivityPR:
		return "🔀"
	case ActivityIssue:
		return "🐛"
	default:
		return "📋"
	}
}

// isHot is a template function to check if activity is hot.
func isHot(t ActivityType) bool {
	return IsHotActivity(t)
}

// tagline returns a fun message based on activity count.
func tagline(count int) string {
	switch {
	case count == 0:
		return "The calm before the storm..."
	case count <= 3:
		return "A quiet day in the neighborhood"
	case count <= 10:
		return "Your network has been busy!"
	case count <= 25:
		return "Lots of action today! 🎉"
	default:
		return "Your network is ON FIRE! 🔥🔥🔥"
	}
}

// activityVerb returns a human-readable verb for the activity type.
func activityVerb(t ActivityType) string {
	switch t {
	case ActivityStarred:
		return "starred"
	case ActivityCreatedRepo:
		return "created"
	case ActivityForked:
		return "forked"
	case ActivityPushed:
		return "pushed to"
	case ActivityPR:
		return "opened PR on"
	case ActivityIssue:
		return "opened issue on"
	default:
		return "acted on"
	}
}

// aggregatedVerb returns a human-readable verb for aggregated activities.
// When count > 1, includes the count (e.g., "pushed 6 times to").
func aggregatedVerb(t ActivityType, count int) string {
	if count <= 1 {
		return activityVerb(t)
	}

	switch t {
	case ActivityStarred:
		return fmt.Sprintf("starred %d repos including", count)
	case ActivityCreatedRepo:
		return fmt.Sprintf("created %d repos including", count)
	case ActivityForked:
		return fmt.Sprintf("forked %d repos including", count)
	case ActivityPushed:
		return fmt.Sprintf("pushed %d times to", count)
	case ActivityPR:
		return fmt.Sprintf("opened %d PRs on", count)
	case ActivityIssue:
		return fmt.Sprintf("opened %d issues on", count)
	default:
		return fmt.Sprintf("acted %d times on", count)
	}
}

// timeRange formats a time range as a human-readable string.
// If first and last are close together, just shows the relative time.
// If they span a significant period, shows the range.
func timeRange(first, last time.Time) string {
	if first.IsZero() && last.IsZero() {
		return "unknown time"
	}
	if first.IsZero() {
		return relativeTime(last)
	}
	if last.IsZero() || first.Equal(last) {
		return relativeTime(first)
	}

	// Calculate duration between first and last
	duration := last.Sub(first)

	// If less than 1 hour apart, just show the most recent time
	if duration < time.Hour {
		return relativeTime(last)
	}

	// Format the duration
	now := time.Now()
	endRelative := relativeTime(last)

	// Calculate how long the activity spanned
	hours := int(duration.Hours())
	if hours < 24 {
		return fmt.Sprintf("%s (over %d hours)", endRelative, hours)
	}

	days := hours / 24
	if days == 1 {
		return fmt.Sprintf("%s (over 1 day)", endRelative)
	}

	// Check if it spans from a while ago to now
	if now.Sub(last) < time.Hour {
		return fmt.Sprintf("over the last %d days", days)
	}

	return fmt.Sprintf("%s (over %d days)", endRelative, days)
}

// relativeTime formats a timestamp as a human-readable relative time string.
// Examples: "just now", "2 hours ago", "yesterday", "3 days ago"
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown time"
	}

	now := time.Now()
	diff := now.Sub(t)

	// Handle future times (shouldn't happen but be safe)
	if diff < 0 {
		return t.Format("Jan 2 at 3:04 PM")
	}

	seconds := int64(diff.Seconds())
	minutes := int64(diff.Minutes())
	hours := int64(diff.Hours())
	days := hours / 24

	switch {
	case seconds < 60:
		return "just now"
	case minutes == 1:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours == 1:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case days < 14:
		return "last week"
	case days < 30:
		weeks := days / 7
		return fmt.Sprintf("%d weeks ago", weeks)
	case days < 60:
		return "last month"
	case days < 365:
		months := days / 30
		return fmt.Sprintf("%d months ago", months)
	default:
		return t.Format("Jan 2, 2006")
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitStreams Activity Report</title>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background: #f6f8fa;
            color: #24292f;
        }
        header {
            background: linear-gradient(135deg, #24292f 0%, #1a1f24 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        header h1 {
            margin: 0 0 10px 0;
        }
        .tagline {
            font-size: 1.1em;
            margin-bottom: 10px;
            opacity: 0.9;
        }
        .meta {
            font-size: 0.9em;
            opacity: 0.7;
        }
        .summary {
            background: white;
            padding: 15px 20px;
            border-radius: 8px;
            border: 1px solid #d0d7de;
            margin-bottom: 20px;
        }
        .summary-main {
            font-size: 1.1em;
            margin-bottom: 10px;
        }
        .stats-grid {
            display: flex;
            flex-wrap: wrap;
            gap: 15px;
            margin-top: 12px;
            padding-top: 12px;
            border-top: 1px solid #eee;
        }
        .stat-item {
            display: flex;
            align-items: center;
            gap: 6px;
            font-size: 0.9em;
            color: #656d76;
        }
        .stat-item .stat-icon {
            font-size: 1.1em;
        }
        .stat-item .stat-count {
            font-weight: 600;
            color: #24292f;
        }
        .highlight {
            background: linear-gradient(135deg, #fff8e1 0%, #fff3c4 100%);
            border: 1px solid #f0c36d;
            border-radius: 8px;
            padding: 15px 20px;
            margin-bottom: 20px;
        }
        .highlight-header {
            font-weight: 600;
            color: #b08800;
            margin-bottom: 8px;
            font-size: 0.85em;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .highlight-content {
            display: flex;
            align-items: flex-start;
            gap: 12px;
        }
        .highlight-icon {
            font-size: 1.5em;
        }
        .highlight-text {
            flex: 1;
        }
        .highlight-text a {
            color: #0969da;
            text-decoration: none;
            font-weight: 500;
        }
        .highlight-text a:hover {
            text-decoration: underline;
        }
        .highlight-reason {
            font-size: 0.9em;
            color: #656d76;
            margin-top: 4px;
        }
        .category-section {
            background: white;
            border-radius: 8px;
            border: 1px solid #d0d7de;
            margin-bottom: 15px;
            overflow: hidden;
        }
        .category-section details {
            margin: 0;
        }
        .category-section summary {
            background: #f6f8fa;
            padding: 12px 15px;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 10px;
            list-style: none;
            user-select: none;
        }
        .category-section summary::-webkit-details-marker {
            display: none;
        }
        .category-section summary::before {
            content: "▶";
            font-size: 0.7em;
            transition: transform 0.2s;
        }
        .category-section details[open] summary::before {
            transform: rotate(90deg);
        }
        .category-section summary:hover {
            background: #eaeef2;
        }
        .category-icon {
            font-size: 1.2em;
        }
        .category-title {
            flex: 1;
            font-weight: 600;
            font-size: 1em;
        }
        .category-count {
            background: #ddf4ff;
            color: #0969da;
            padding: 2px 8px;
            border-radius: 10px;
            font-size: 0.85em;
            font-weight: 500;
        }
        .user-section {
            background: white;
            border-radius: 8px;
            border: 1px solid #d0d7de;
            margin-bottom: 15px;
            overflow: hidden;
        }
        .user-section details {
            margin: 0;
        }
        .user-section summary {
            background: #f6f8fa;
            padding: 12px 15px;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 10px;
            list-style: none;
            user-select: none;
        }
        .user-section summary::-webkit-details-marker {
            display: none;
        }
        .user-section summary::before {
            content: "▶";
            font-size: 0.7em;
            transition: transform 0.2s;
        }
        .user-section details[open] summary::before {
            transform: rotate(90deg);
        }
        .user-section summary:hover {
            background: #eaeef2;
        }
        .user-section summary img {
            width: 24px;
            height: 24px;
            border-radius: 50%;
        }
        .user-section summary h2 {
            margin: 0;
            font-size: 1em;
            flex: 1;
        }
        .user-count {
            background: #ddf4ff;
            color: #0969da;
            padding: 2px 8px;
            border-radius: 10px;
            font-size: 0.85em;
            font-weight: 500;
        }
        .mvp-badge {
            background: #ffc107;
            color: #000;
            font-size: 0.7em;
            padding: 2px 8px;
            border-radius: 12px;
            font-weight: 600;
            margin-left: auto;
        }
        .activity-list {
            list-style: none;
            margin: 0;
            padding: 0;
        }
        .activity-item {
            padding: 10px 15px;
            border-top: 1px solid #d0d7de;
            display: flex;
            gap: 10px;
            align-items: flex-start;
        }
        .activity-item:last-child {
            border-bottom: none;
        }
        .activity-item.hot {
            background: linear-gradient(90deg, #fff5f5 0%, white 100%);
        }
        .activity-icon {
            font-size: 1.2em;
        }
        .activity-avatar {
            width: 32px;
            height: 32px;
            border-radius: 50%;
            margin-right: 8px;
        }
        .activity-user {
            font-weight: 500;
            color: #24292f;
            display: flex;
            align-items: center;
        }
        .highlight-avatar {
            width: 40px;
            height: 40px;
            border-radius: 50%;
            margin-right: 12px;
        }
        .hot-badge {
            font-size: 0.8em;
            margin-left: 4px;
        }
        .activity-content {
            flex: 1;
        }
        .activity-content a {
            color: #0969da;
            text-decoration: none;
        }
        .activity-content a:hover {
            text-decoration: underline;
        }
        .activity-time {
            font-size: 0.85em;
            color: #656d76;
        }
        .activity-details {
            font-size: 0.9em;
            color: #656d76;
            margin-top: 4px;
        }
        .empty-state {
            text-align: center;
            padding: 40px;
            color: #656d76;
        }
        .empty-state .empty-icon {
            font-size: 3em;
            margin-bottom: 10px;
        }
        .view-toggle {
            display: flex;
            gap: 8px;
            margin-bottom: 15px;
        }
        .view-toggle button {
            padding: 6px 12px;
            border: 1px solid #d0d7de;
            background: white;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9em;
        }
        .view-toggle button.active {
            background: #0969da;
            color: white;
            border-color: #0969da;
        }
        .view-toggle button:hover:not(.active) {
            background: #f6f8fa;
        }
        .view-category, .view-user {
            display: none;
        }
        .view-category.active, .view-user.active {
            display: block;
        }
    </style>
</head>
<body>
    <header>
        <h1>🌊 GitStreams</h1>
        <div class="tagline">{{tagline .TotalActivities}}</div>
        <div class="meta">
            {{.PeriodStart.Format "Jan 2"}} → {{.PeriodEnd.Format "Jan 2, 2006"}}
        </div>
    </header>

    {{$stats := .GetStats}}
    <div class="summary">
        <div class="summary-main">
            <strong>{{.TotalActivities}}</strong> {{if eq .TotalActivities 1}}thing happened{{else}}things happened{{end}} across <strong>{{len .UserActivities}}</strong> {{if eq (len .UserActivities) 1}}developer{{else}}developers{{end}} you follow.
        </div>
        {{if gt .TotalActivities 0}}
        <div class="stats-grid">
            {{if gt $stats.Stars 0}}<div class="stat-item"><span class="stat-icon">⭐</span><span class="stat-count">{{$stats.Stars}}</span> star{{if ne $stats.Stars 1}}s{{end}}</div>{{end}}
            {{if gt $stats.Repos 0}}<div class="stat-item"><span class="stat-icon">🆕</span><span class="stat-count">{{$stats.Repos}}</span> new repo{{if ne $stats.Repos 1}}s{{end}}</div>{{end}}
            {{if gt $stats.PRs 0}}<div class="stat-item"><span class="stat-icon">🔀</span><span class="stat-count">{{$stats.PRs}}</span> PR{{if ne $stats.PRs 1}}s{{end}}</div>{{end}}
            {{if gt $stats.Forks 0}}<div class="stat-item"><span class="stat-icon">🔱</span><span class="stat-count">{{$stats.Forks}}</span> fork{{if ne $stats.Forks 1}}s{{end}}</div>{{end}}
            {{if gt $stats.Pushes 0}}<div class="stat-item"><span class="stat-icon">📤</span><span class="stat-count">{{$stats.Pushes}}</span> push{{if ne $stats.Pushes 1}}es{{end}}</div>{{end}}
            {{if gt $stats.Issues 0}}<div class="stat-item"><span class="stat-icon">🐛</span><span class="stat-count">{{$stats.Issues}}</span> issue{{if ne $stats.Issues 1}}s{{end}}</div>{{end}}
        </div>
        {{end}}
    </div>

    {{$highlight := .GetHighlight}}
    {{if $highlight}}
    <div class="highlight">
        <div class="highlight-header">✨ Highlight of the Day</div>
        <div class="highlight-content">
            {{if $highlight.AvatarURL}}<img src="{{$highlight.AvatarURL}}" alt="{{$highlight.User}}" class="highlight-avatar">{{end}}
            <span class="highlight-icon">{{icon $highlight.Activity.Type}}</span>
            <div class="highlight-text">
                <strong>{{$highlight.User}}</strong> {{verb $highlight.Activity.Type}} <a href="{{$highlight.Activity.RepoURL}}">{{$highlight.Activity.RepoName}}</a>
                <div class="highlight-reason">{{$highlight.Reason}}</div>
            </div>
        </div>
    </div>
    {{end}}

    {{$mostActive := .MostActiveUser}}
    {{if .UserActivities}}
    <div class="view-toggle">
        <button class="active" onclick="toggleView('category')">By Category</button>
        <button onclick="toggleView('user')">By User</button>
    </div>

    <div class="view-category active">
        {{range .AggregatedActivitiesByCategory}}
        <div class="category-section">
            <details open>
                <summary>
                    <span class="category-icon">{{icon .Type}}</span>
                    <span class="category-title">{{categoryName .Type}}</span>
                    <span class="category-count">{{len .Activities}}</span>
                </summary>
                <ul class="activity-list">
                    {{range .Activities}}
                    <li class="activity-item{{if isHot .Type}} hot{{end}}">
                        <span class="activity-icon">{{icon .Type}}{{if isHot .Type}}<span class="hot-badge">🔥</span>{{end}}</span>
                        <div class="activity-content">
                            <span class="activity-user">{{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="{{.User}}" class="activity-avatar">{{end}}{{.User}}</span> {{aggVerb .Type .Count}} <a href="{{.RepoURL}}">{{.RepoName}}</a>
                            <div class="activity-time">{{timeRange .FirstTime .LastTime}}</div>
                            {{if .Details}}<div class="activity-details">{{.Details}}</div>{{end}}
                        </div>
                    </li>
                    {{end}}
                </ul>
            </details>
        </div>
        {{end}}
    </div>

    <div class="view-user">
        {{range .AggregatedUserActivities}}
        <div class="user-section">
            <details open>
                <summary>
                    {{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="{{.User}}">{{end}}
                    <h2>{{.User}}</h2>
                    {{if eq .User $mostActive}}<span class="mvp-badge">🏆 MVP</span>{{end}}
                    <span class="user-count">{{len .Activities}}</span>
                </summary>
                <ul class="activity-list">
                    {{range .Activities}}
                    <li class="activity-item{{if isHot .Type}} hot{{end}}">
                        <span class="activity-icon">{{icon .Type}}{{if isHot .Type}}<span class="hot-badge">🔥</span>{{end}}</span>
                        <div class="activity-content">
                            <span>{{aggVerb .Type .Count}} <a href="{{.RepoURL}}">{{.RepoName}}</a></span>
                            <div class="activity-time">{{timeRange .FirstTime .LastTime}}</div>
                            {{if .Details}}<div class="activity-details">{{.Details}}</div>{{end}}
                        </div>
                    </li>
                    {{end}}
                </ul>
            </details>
        </div>
        {{end}}
    </div>

    <script>
        function toggleView(view) {
            document.querySelectorAll('.view-toggle button').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.view-category, .view-user').forEach(v => v.classList.remove('active'));
            event.target.classList.add('active');
            document.querySelector('.view-' + view).classList.add('active');
        }
    </script>
    {{else}}
        <div class="empty-state">
            <div class="empty-icon">😴</div>
            <p>Nothing to see here... yet!</p>
            <p>Your network is taking a break. Check back later!</p>
        </div>
    {{end}}
</body>
</html>
`

// HTMLGenerator generates HTML reports.
type HTMLGenerator struct {
	tmpl *template.Template
}

// NewHTMLGenerator creates a new HTMLGenerator with the default template.
func NewHTMLGenerator() (*HTMLGenerator, error) {
	funcMap := template.FuncMap{
		"icon":         activityIcon,
		"verb":         activityVerb,
		"aggVerb":      aggregatedVerb,
		"isHot":        isHot,
		"tagline":      tagline,
		"categoryName": categoryName,
		"relTime":      relativeTime,
		"timeRange":    timeRange,
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return nil, err
	}

	return &HTMLGenerator{tmpl: tmpl}, nil
}

// Generate writes an HTML report to the provided writer.
func (g *HTMLGenerator) Generate(w io.Writer, report *Report) error {
	return g.tmpl.Execute(w, report)
}
