// Package report provides HTML report generation for gitstreams.
package report

import (
	"html/template"
	"io"
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
	RepoName  string
	RepoURL   string
	Timestamp time.Time
	Details   string
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

// activityIcon returns an emoji icon for the activity type.
func activityIcon(t ActivityType) string {
	switch t {
	case ActivityStarred:
		return "⭐"
	case ActivityCreatedRepo:
		return "📦"
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
            background: #24292f;
            color: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        header h1 {
            margin: 0 0 10px 0;
        }
        .meta {
            font-size: 0.9em;
            opacity: 0.8;
        }
        .summary {
            background: white;
            padding: 15px 20px;
            border-radius: 8px;
            border: 1px solid #d0d7de;
            margin-bottom: 20px;
        }
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
            gap: 10px;
            margin-top: 12px;
        }
        .summary-stat {
            text-align: center;
            padding: 8px;
            background: #f6f8fa;
            border-radius: 6px;
        }
        .summary-stat .count {
            font-size: 1.4em;
            font-weight: 600;
            display: block;
        }
        .summary-stat .label {
            font-size: 0.8em;
            color: #656d76;
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
        .activity-icon {
            font-size: 1.2em;
        }
        .activity-user {
            font-weight: 500;
            color: #24292f;
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
        <h1>GitStreams Activity Report</h1>
        <div class="meta">
            Generated: {{.GeneratedAt.Format "Jan 2, 2006 3:04 PM"}}<br>
            Period: {{.PeriodStart.Format "Jan 2"}} - {{.PeriodEnd.Format "Jan 2, 2006"}}
        </div>
    </header>

    <div class="summary">
        <strong>{{.TotalActivities}}</strong> interesting {{if eq .TotalActivities 1}}activity{{else}}activities{{end}} from <strong>{{len .UserActivities}}</strong> {{if eq (len .UserActivities) 1}}user{{else}}users{{end}} you follow.
        {{if .UserActivities}}
        <div class="summary-grid">
            {{range .ActivitiesByCategory}}
            <div class="summary-stat">
                <span class="count">{{icon .Type}} {{len .Activities}}</span>
                <span class="label">{{categoryName .Type}}</span>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>

    {{if .UserActivities}}
    <div class="view-toggle">
        <button class="active" onclick="toggleView('category')">By Category</button>
        <button onclick="toggleView('user')">By User</button>
    </div>

    <div class="view-category active">
        {{range .ActivitiesByCategory}}
        <div class="category-section">
            <details open>
                <summary>
                    <span class="category-icon">{{icon .Type}}</span>
                    <span class="category-title">{{categoryName .Type}}</span>
                    <span class="category-count">{{len .Activities}}</span>
                </summary>
                <ul class="activity-list">
                    {{range .Activities}}
                    <li class="activity-item">
                        <div class="activity-content">
                            <span class="activity-user">{{.User}}</span> {{verb .Type}} <a href="{{.RepoURL}}">{{.RepoName}}</a>
                            <div class="activity-time">{{.Timestamp.Format "Jan 2 at 3:04 PM"}}</div>
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
        {{range .UserActivities}}
        <div class="user-section">
            <details open>
                <summary>
                    {{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="{{.User}}">{{end}}
                    <h2>{{.User}}</h2>
                    <span class="user-count">{{len .Activities}}</span>
                </summary>
                <ul class="activity-list">
                    {{range .Activities}}
                    <li class="activity-item">
                        <span class="activity-icon">{{icon .Type}}</span>
                        <div class="activity-content">
                            <span>{{verb .Type}} <a href="{{.RepoURL}}">{{.RepoName}}</a></span>
                            <div class="activity-time">{{.Timestamp.Format "Jan 2 at 3:04 PM"}}</div>
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
            <p>No interesting activity to report.</p>
            <p>Check back later!</p>
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
		"categoryName": categoryName,
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
