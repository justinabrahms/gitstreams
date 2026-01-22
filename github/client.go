// Package github provides a client for interacting with the GitHub API.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// rateLimitWarningThreshold is the number of remaining requests below which
// a warning will be logged.
const rateLimitWarningThreshold = 100

// cacheEntry stores a cached API response with its ETag.
type cacheEntry struct {
	timestamp time.Time
	etag      string
	data      []byte
}

// RateLimit contains GitHub API rate limit information.
type RateLimit struct {
	Reset     time.Time
	Limit     int
	Remaining int
	Used      int
}

// Client is a GitHub API client.
type Client struct {
	httpClient  *http.Client
	logger      *slog.Logger
	cache       map[string]*cacheEntry
	rateLimit   *RateLimit
	baseURL     string
	token       string
	cacheMu     sync.RWMutex
	rateLimitMu sync.RWMutex
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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	HTMLURL     string    `json:"html_url"`
	Language    string    `json:"language"`
	Owner       User      `json:"owner"`
	ID          int64     `json:"id"`
	StarCount   int       `json:"stargazers_count"`
	ForkCount   int       `json:"forks_count"`
	Private     bool      `json:"private"`
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

// WithLogger sets a custom logger for the client.
func WithLogger(l *slog.Logger) Option {
	return func(client *Client) {
		client.logger = l
	}
}

// NewClient creates a new GitHub API client.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		token:      token,
		logger:     slog.Default(),
		cache:      make(map[string]*cacheEntry),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetRateLimit returns the current rate limit information.
// Returns nil if no rate limit information has been received yet.
func (c *Client) GetRateLimit() *RateLimit {
	c.rateLimitMu.RLock()
	defer c.rateLimitMu.RUnlock()
	if c.rateLimit == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	rl := *c.rateLimit
	return &rl
}

// ClearCache clears the ETag cache.
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache = make(map[string]*cacheEntry)
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

	// Check cache for ETag and add If-None-Match header
	c.cacheMu.RLock()
	cached := c.cache[path]
	c.cacheMu.RUnlock()
	if cached != nil && cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse and store rate limit headers
	c.parseRateLimitHeaders(resp)

	// Handle 304 Not Modified - return cached data
	if resp.StatusCode == http.StatusNotModified && cached != nil {
		c.logger.Debug("using cached response",
			"path", path,
			"etag", cached.etag,
		)
		if result != nil {
			if unmarshalErr := json.Unmarshal(cached.data, result); unmarshalErr != nil {
				return fmt.Errorf("decoding cached response: %w", unmarshalErr)
			}
		}
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Store ETag and response in cache if we got an ETag
	if etag := resp.Header.Get("ETag"); etag != "" {
		c.cacheMu.Lock()
		c.cache[path] = &cacheEntry{
			etag:      etag,
			data:      body,
			timestamp: time.Now(),
		}
		c.cacheMu.Unlock()
		c.logger.Debug("cached response",
			"path", path,
			"etag", etag,
		)
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// parseRateLimitHeaders extracts rate limit information from response headers.
func (c *Client) parseRateLimitHeaders(resp *http.Response) {
	rl := &RateLimit{}

	if v := resp.Header.Get("X-RateLimit-Limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Limit = n
		}
	}

	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Remaining = n
		}
	}

	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			rl.Reset = time.Unix(n, 0)
		}
	}

	if v := resp.Header.Get("X-RateLimit-Used"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Used = n
		}
	}

	// Only update if we got valid rate limit data
	if rl.Limit > 0 {
		c.rateLimitMu.Lock()
		c.rateLimit = rl
		c.rateLimitMu.Unlock()

		// Log rate limit info
		c.logger.Debug("rate limit status",
			"remaining", rl.Remaining,
			"limit", rl.Limit,
			"reset", rl.Reset,
		)

		// Warn if rate limit is low
		if rl.Remaining < rateLimitWarningThreshold {
			c.logger.Warn("GitHub API rate limit is low",
				"remaining", rl.Remaining,
				"limit", rl.Limit,
				"reset", rl.Reset,
				"reset_in", time.Until(rl.Reset).Round(time.Second),
			)
		}
	}
}

// getPaginated fetches all pages of results for a given path.
// It handles GitHub's pagination by requesting 100 items per page until
// no more results are returned.
func (c *Client) getPaginated(ctx context.Context, basePath string, result any) error {
	// Use reflection to work with any slice type
	resultVal := reflect.ValueOf(result)
	if resultVal.Kind() != reflect.Ptr || resultVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("result must be a pointer to a slice")
	}

	sliceVal := resultVal.Elem()

	page := 1
	perPage := 100 // GitHub's maximum per_page value

	for {
		// Build path with pagination parameters
		separator := "?"
		if strings.Contains(basePath, "?") {
			separator = "&"
		}
		path := fmt.Sprintf("%s%spage=%d&per_page=%d", basePath, separator, page, perPage)

		// Create a new slice to hold this page's results
		pageResult := reflect.New(sliceVal.Type()).Interface()

		if err := c.get(ctx, path, pageResult); err != nil {
			return err
		}

		// Get the slice value from the pointer
		pageSlice := reflect.ValueOf(pageResult).Elem()

		// If we got no results, we're done
		if pageSlice.Len() == 0 {
			break
		}

		// Append this page's results to the total
		sliceVal = reflect.AppendSlice(sliceVal, pageSlice)

		// If we got fewer results than per_page, this is the last page
		if pageSlice.Len() < perPage {
			break
		}

		page++
	}

	// Set the final result
	resultVal.Elem().Set(sliceVal)
	return nil
}

// GetFollowedUsers returns the users that the authenticated user follows.
// This method automatically handles pagination to fetch all followed users.
func (c *Client) GetFollowedUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := c.getPaginated(ctx, "/user/following", &users); err != nil {
		return nil, fmt.Errorf("fetching followed users: %w", err)
	}
	return users, nil
}

// GetFollowedUsersByUsername returns the users that a specific user follows.
// This method automatically handles pagination to fetch all followed users.
func (c *Client) GetFollowedUsersByUsername(ctx context.Context, username string) ([]User, error) {
	var users []User
	path := fmt.Sprintf("/users/%s/following", username)
	if err := c.getPaginated(ctx, path, &users); err != nil {
		return nil, fmt.Errorf("fetching users followed by %s: %w", username, err)
	}
	return users, nil
}

// GetStarredRepos returns repositories starred by the authenticated user.
// This method automatically handles pagination to fetch all starred repos.
func (c *Client) GetStarredRepos(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	if err := c.getPaginated(ctx, "/user/starred", &repos); err != nil {
		return nil, fmt.Errorf("fetching starred repos: %w", err)
	}
	return repos, nil
}

// GetStarredReposByUsername returns repositories starred by a specific user.
// This method automatically handles pagination to fetch all starred repos.
func (c *Client) GetStarredReposByUsername(ctx context.Context, username string) ([]Repository, error) {
	var repos []Repository
	path := fmt.Sprintf("/users/%s/starred", username)
	if err := c.getPaginated(ctx, path, &repos); err != nil {
		return nil, fmt.Errorf("fetching repos starred by %s: %w", username, err)
	}
	return repos, nil
}

// GetOwnedRepos returns repositories owned by the authenticated user.
// This method automatically handles pagination to fetch all owned repos.
func (c *Client) GetOwnedRepos(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	if err := c.getPaginated(ctx, "/user/repos?type=owner", &repos); err != nil {
		return nil, fmt.Errorf("fetching owned repos: %w", err)
	}
	return repos, nil
}

// GetOwnedReposByUsername returns repositories owned by a specific user.
// This method automatically handles pagination to fetch all owned repos.
func (c *Client) GetOwnedReposByUsername(ctx context.Context, username string) ([]Repository, error) {
	var repos []Repository
	path := fmt.Sprintf("/users/%s/repos?type=owner", username)
	if err := c.getPaginated(ctx, path, &repos); err != nil {
		return nil, fmt.Errorf("fetching repos owned by %s: %w", username, err)
	}
	return repos, nil
}

// GetRecentEvents returns recent events for the authenticated user.
// This method automatically handles pagination to fetch all recent events.
func (c *Client) GetRecentEvents(ctx context.Context, username string) ([]Event, error) {
	var events []Event
	path := fmt.Sprintf("/users/%s/events", username)
	if err := c.getPaginated(ctx, path, &events); err != nil {
		return nil, fmt.Errorf("fetching events for %s: %w", username, err)
	}
	return events, nil
}

// GetReceivedEvents returns events received by a user (their feed).
// This method automatically handles pagination to fetch all received events.
func (c *Client) GetReceivedEvents(ctx context.Context, username string) ([]Event, error) {
	var events []Event
	path := fmt.Sprintf("/users/%s/received_events", username)
	if err := c.getPaginated(ctx, path, &events); err != nil {
		return nil, fmt.Errorf("fetching received events for %s: %w", username, err)
	}
	return events, nil
}
