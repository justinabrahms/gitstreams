// Package github provides a client for interacting with the GitHub API.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// Client is a GitHub API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// User represents a GitHub user.
type User struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	ID        int64  `json:"id"`
}

// Repository represents a GitHub repository.
type Repository struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Language    string `json:"language"`
	Owner       User   `json:"owner"`
	ID          int64  `json:"id"`
	StarCount   int    `json:"stargazers_count"`
	ForkCount   int    `json:"forks_count"`
	Private     bool   `json:"private"`
}

// Event represents a GitHub event.
type Event struct {
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     User            `json:"actor"`
	Repo      EventRepo       `json:"repo"`
}

// EventRepo is a minimal repo representation in events.
type EventRepo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	ID   int64  `json:"id"`
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		client.httpClient = c
	}
}

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(client *Client) {
		client.baseURL = url
	}
}

// NewClient creates a new GitHub API client.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		token:      token,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// GetFollowedUsers returns the users that the authenticated user follows.
func (c *Client) GetFollowedUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := c.get(ctx, "/user/following", &users); err != nil {
		return nil, fmt.Errorf("fetching followed users: %w", err)
	}
	return users, nil
}

// GetFollowedUsersByUsername returns the users that a specific user follows.
func (c *Client) GetFollowedUsersByUsername(ctx context.Context, username string) ([]User, error) {
	var users []User
	path := fmt.Sprintf("/users/%s/following", username)
	if err := c.get(ctx, path, &users); err != nil {
		return nil, fmt.Errorf("fetching users followed by %s: %w", username, err)
	}
	return users, nil
}

// GetStarredRepos returns repositories starred by the authenticated user.
func (c *Client) GetStarredRepos(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	if err := c.get(ctx, "/user/starred", &repos); err != nil {
		return nil, fmt.Errorf("fetching starred repos: %w", err)
	}
	return repos, nil
}

// GetStarredReposByUsername returns repositories starred by a specific user.
func (c *Client) GetStarredReposByUsername(ctx context.Context, username string) ([]Repository, error) {
	var repos []Repository
	path := fmt.Sprintf("/users/%s/starred", username)
	if err := c.get(ctx, path, &repos); err != nil {
		return nil, fmt.Errorf("fetching repos starred by %s: %w", username, err)
	}
	return repos, nil
}

// GetOwnedRepos returns repositories owned by the authenticated user.
func (c *Client) GetOwnedRepos(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	if err := c.get(ctx, "/user/repos?type=owner", &repos); err != nil {
		return nil, fmt.Errorf("fetching owned repos: %w", err)
	}
	return repos, nil
}

// GetOwnedReposByUsername returns repositories owned by a specific user.
func (c *Client) GetOwnedReposByUsername(ctx context.Context, username string) ([]Repository, error) {
	var repos []Repository
	path := fmt.Sprintf("/users/%s/repos?type=owner", username)
	if err := c.get(ctx, path, &repos); err != nil {
		return nil, fmt.Errorf("fetching repos owned by %s: %w", username, err)
	}
	return repos, nil
}

// GetRecentEvents returns recent events for the authenticated user.
func (c *Client) GetRecentEvents(ctx context.Context, username string) ([]Event, error) {
	var events []Event
	path := fmt.Sprintf("/users/%s/events", username)
	if err := c.get(ctx, path, &events); err != nil {
		return nil, fmt.Errorf("fetching events for %s: %w", username, err)
	}
	return events, nil
}

// GetReceivedEvents returns events received by a user (their feed).
func (c *Client) GetReceivedEvents(ctx context.Context, username string) ([]Event, error) {
	var events []Event
	path := fmt.Sprintf("/users/%s/received_events", username)
	if err := c.get(ctx, path, &events); err != nil {
		return nil, fmt.Errorf("fetching received events for %s: %w", username, err)
	}
	return events, nil
}
