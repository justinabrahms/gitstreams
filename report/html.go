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
        .user-section {
            background: white;
            border-radius: 8px;
            border: 1px solid #d0d7de;
            margin-bottom: 15px;
            overflow: hidden;
        }
        .user-header {
            background: #f6f8fa;
            padding: 12px 15px;
            border-bottom: 1px solid #d0d7de;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .user-header img {
            width: 32px;
            height: 32px;
            border-radius: 50%;
        }
        .user-header h2 {
            margin: 0;
            font-size: 1.1em;
        }
        .activity-list {
            list-style: none;
            margin: 0;
            padding: 0;
        }
        .activity-item {
            padding: 10px 15px;
            border-bottom: 1px solid #d0d7de;
            display: flex;
            gap: 10px;
            align-items: flex-start;
        }
        .activity-item:last-child {
            border-bottom: none;
        }
        .activity-icon {
            font-size: 1.2em;
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
    </style>
</head>
<body>
    <header>
        <h1>🌊 GitStreams Activity Report</h1>
        <div class="meta">
            Generated: {{.GeneratedAt.Format "Jan 2, 2006 3:04 PM"}}<br>
            Period: {{.PeriodStart.Format "Jan 2"}} - {{.PeriodEnd.Format "Jan 2, 2006"}}
        </div>
    </header>

    <div class="summary">
        <strong>{{.TotalActivities}}</strong> interesting {{if eq .TotalActivities 1}}activity{{else}}activities{{end}} from <strong>{{len .UserActivities}}</strong> {{if eq (len .UserActivities) 1}}user{{else}}users{{end}} you follow.
    </div>

    {{if .UserActivities}}
        {{range .UserActivities}}
        <div class="user-section">
            <div class="user-header">
                {{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="{{.User}}">{{end}}
                <h2>{{.User}}</h2>
            </div>
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
        </div>
        {{end}}
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
		"icon": activityIcon,
		"verb": activityVerb,
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
