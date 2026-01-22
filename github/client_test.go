package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-token")
	if c.token != "test-token" {
		t.Errorf("expected token 'test-token', got %q", c.token)
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultBaseURL, c.baseURL)
	}
}

func TestNewClientWithOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}
	customURL := "https://custom.github.example.com"

	c := NewClient("token", WithHTTPClient(customClient), WithBaseURL(customURL))

	if c.httpClient != customClient {
		t.Error("custom HTTP client not set")
	}
	if c.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, c.baseURL)
	}
}

func TestGetFollowedUsers(t *testing.T) {
	users := []User{
		{Login: "user1", ID: 1, Name: "User One"},
		{Login: "user2", ID: 2, Name: "User Two"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/following" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("GetFollowedUsers() error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 users, got %d", len(result))
	}
	if result[0].Login != "user1" {
		t.Errorf("expected login 'user1', got %q", result[0].Login)
	}
}

func TestGetFollowedUsersByUsername(t *testing.T) {
	users := []User{{Login: "followed1", ID: 10}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/following" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetFollowedUsersByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetFollowedUsersByUsername() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 user, got %d", len(result))
	}
}

func TestGetStarredRepos(t *testing.T) {
	repos := []Repository{
		{ID: 1, Name: "repo1", FullName: "owner/repo1", StarCount: 100},
		{ID: 2, Name: "repo2", FullName: "owner/repo2", StarCount: 200},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/starred" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(repos); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetStarredRepos(context.Background())
	if err != nil {
		t.Fatalf("GetStarredRepos() error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 repos, got %d", len(result))
	}
	if result[0].StarCount != 100 {
		t.Errorf("expected star count 100, got %d", result[0].StarCount)
	}
}

func TestGetStarredReposByUsername(t *testing.T) {
	repos := []Repository{{ID: 1, Name: "starred-repo"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/starred" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(repos); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetStarredReposByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetStarredReposByUsername() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
}

func TestGetOwnedRepos(t *testing.T) {
	repos := []Repository{
		{ID: 1, Name: "my-repo", FullName: "me/my-repo", Private: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "owner" {
			t.Errorf("expected type=owner query param, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(repos); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetOwnedRepos(context.Background())
	if err != nil {
		t.Fatalf("GetOwnedRepos() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
}

func TestGetOwnedReposByUsername(t *testing.T) {
	repos := []Repository{{ID: 1, Name: "user-repo"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/repos" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(repos); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetOwnedReposByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetOwnedReposByUsername() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
}

func TestGetRecentEvents(t *testing.T) {
	events := []Event{
		{ID: "123", Type: "PushEvent", Actor: User{Login: "testuser"}},
		{ID: "124", Type: "CreateEvent", Actor: User{Login: "testuser"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetRecentEvents(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetRecentEvents() error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 events, got %d", len(result))
	}
	if result[0].Type != "PushEvent" {
		t.Errorf("expected type 'PushEvent', got %q", result[0].Type)
	}
}

func TestGetReceivedEvents(t *testing.T) {
	events := []Event{
		{ID: "200", Type: "WatchEvent"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser/received_events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetReceivedEvents(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetReceivedEvents() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer server.Close()

	c := NewClient("bad-token", WithBaseURL(server.URL))
	_, err := c.GetFollowedUsers(context.Background())
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.GetFollowedUsers(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRequestHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("unexpected API version header: %s", r.Header.Get("X-GitHub-Api-Version"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL))
	_, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no auth header for empty token, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	c := NewClient("", WithBaseURL(server.URL))
	_, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestETagCaching(t *testing.T) {
	users := []User{{Login: "user1", ID: 1}}
	requestCount := 0
	etag := `"abc123"`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Check if client sent If-None-Match header
		if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch == etag {
			// Return 304 Not Modified
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// First request - return data with ETag
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", etag)
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))

	// First request - should get data and cache it
	result1, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("first request error: %v", err)
	}
	if len(result1) != 1 {
		t.Errorf("expected 1 user, got %d", len(result1))
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	// Second request - should send If-None-Match and get 304
	result2, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("second request error: %v", err)
	}
	if len(result2) != 1 {
		t.Errorf("expected 1 user from cache, got %d", len(result2))
	}
	if result2[0].Login != "user1" {
		t.Errorf("expected login 'user1' from cache, got %q", result2[0].Login)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}

func TestIfNoneMatchHeader(t *testing.T) {
	users := []User{{Login: "user1", ID: 1}}
	etag := `"test-etag"`
	receivedIfNoneMatch := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfNoneMatch = r.Header.Get("If-None-Match")

		if receivedIfNoneMatch == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", etag)
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL))

	// First request - no If-None-Match
	_, _ = c.GetFollowedUsers(context.Background())
	if receivedIfNoneMatch != "" {
		t.Errorf("first request should not have If-None-Match, got %q", receivedIfNoneMatch)
	}

	// Second request - should have If-None-Match
	_, _ = c.GetFollowedUsers(context.Background())
	if receivedIfNoneMatch != etag {
		t.Errorf("second request should have If-None-Match %q, got %q", etag, receivedIfNoneMatch)
	}
}

func TestRateLimitHeaderParsing(t *testing.T) {
	resetTime := time.Now().Add(time.Hour).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
		w.Header().Set("X-RateLimit-Used", "1")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL))

	// Before any request, rate limit should be nil
	if rl := c.GetRateLimit(); rl != nil {
		t.Error("expected nil rate limit before any request")
	}

	_, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	rl := c.GetRateLimit()
	if rl == nil {
		t.Fatal("expected rate limit after request")
		return
	}

	if rl.Limit != 5000 {
		t.Errorf("expected limit 5000, got %d", rl.Limit)
	}
	if rl.Remaining != 4999 {
		t.Errorf("expected remaining 4999, got %d", rl.Remaining)
	}
	if rl.Used != 1 {
		t.Errorf("expected used 1, got %d", rl.Used)
	}
	if rl.Reset.Unix() != resetTime {
		t.Errorf("expected reset time %d, got %d", resetTime, rl.Reset.Unix())
	}
}

func TestRateLimitLowWarning(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "50") // Below threshold of 100
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL), WithLogger(logger))

	_, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "rate limit is low") {
		t.Errorf("expected rate limit warning in logs, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "remaining=50") {
		t.Errorf("expected remaining=50 in warning, got: %s", logOutput)
	}
}

func TestClearCache(t *testing.T) {
	users := []User{{Login: "user1", ID: 1}}
	requestCount := 0
	etag := `"cache-test"`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", etag)
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}))
	defer server.Close()

	c := NewClient("token", WithBaseURL(server.URL))

	// First request
	_, _ = c.GetFollowedUsers(context.Background())
	// Second request (should use cache)
	_, _ = c.GetFollowedUsers(context.Background())

	if requestCount != 2 {
		t.Errorf("expected 2 requests before clear, got %d", requestCount)
	}

	// Clear cache
	c.ClearCache()

	// Third request (should not send If-None-Match)
	_, _ = c.GetFollowedUsers(context.Background())

	if requestCount != 3 {
		t.Errorf("expected 3 requests after clear, got %d", requestCount)
	}
}

func TestWithLogger(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := NewClient("token", WithLogger(logger))
	if c.logger != logger {
		t.Error("expected custom logger to be set")
	}
}

func TestGetFollowedUsersPagination(t *testing.T) {
	// Create test users across multiple pages
	// Page 1: 100 users, Page 2: 100 users, Page 3: 2 users (total: 202)
	page1Users := make([]User, 100)
	for i := 0; i < 100; i++ {
		page1Users[i] = User{Login: fmt.Sprintf("user%d", i), ID: int64(i)}
	}

	page2Users := make([]User, 100)
	for i := 0; i < 100; i++ {
		page2Users[i] = User{Login: fmt.Sprintf("user%d", i+100), ID: int64(i + 100)}
	}

	page3Users := []User{
		{Login: "user200", ID: 200},
		{Login: "user201", ID: 201},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check path
		if r.URL.Path != "/user/following" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Parse query params
		query := r.URL.Query()
		page := query.Get("page")
		perPage := query.Get("per_page")

		// Verify per_page is set to 100
		if perPage != "100" {
			t.Errorf("expected per_page=100, got %s", perPage)
		}

		w.Header().Set("Content-Type", "application/json")

		// Return appropriate page
		switch page {
		case "1":
			if err := json.NewEncoder(w).Encode(page1Users); err != nil {
				t.Fatalf("encoding page 1: %v", err)
			}
		case "2":
			if err := json.NewEncoder(w).Encode(page2Users); err != nil {
				t.Fatalf("encoding page 2: %v", err)
			}
		case "3":
			if err := json.NewEncoder(w).Encode(page3Users); err != nil {
				t.Fatalf("encoding page 3: %v", err)
			}
		default:
			t.Errorf("unexpected page number: %s", page)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetFollowedUsers(context.Background())
	if err != nil {
		t.Fatalf("GetFollowedUsers() error: %v", err)
	}

	// Should have fetched all 202 users
	if len(result) != 202 {
		t.Errorf("expected 202 users, got %d", len(result))
	}

	// Verify first user from page 1
	if result[0].Login != "user0" {
		t.Errorf("expected first user 'user0', got %q", result[0].Login)
	}

	// Verify last user from page 3
	if result[201].Login != "user201" {
		t.Errorf("expected last user 'user201', got %q", result[201].Login)
	}

	// Verify a user from page 2
	if result[150].Login != "user150" {
		t.Errorf("expected middle user 'user150', got %q", result[150].Login)
	}
}

func TestGetStarredReposPagination(t *testing.T) {
	// Test with exactly 100 repos (single page, should not request page 2)
	repos := make([]Repository, 100)
	for i := 0; i < 100; i++ {
		repos[i] = Repository{
			ID:       int64(i),
			Name:     fmt.Sprintf("repo%d", i),
			FullName: fmt.Sprintf("owner/repo%d", i),
		}
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		query := r.URL.Query()
		page := query.Get("page")

		if page == "1" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(repos); err != nil {
				t.Fatalf("encoding repos: %v", err)
			}
		} else if page == "2" {
			// Should not request page 2 if page 1 had exactly 100 items
			// Return empty to stop pagination
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode([]Repository{}); err != nil {
				t.Fatalf("encoding empty repos: %v", err)
			}
		} else {
			t.Errorf("unexpected page: %s", page)
		}
	}))
	defer server.Close()

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.GetStarredRepos(context.Background())
	if err != nil {
		t.Fatalf("GetStarredRepos() error: %v", err)
	}

	if len(result) != 100 {
		t.Errorf("expected 100 repos, got %d", len(result))
	}

	// Should have made exactly 2 requests (page 1 and page 2 to check if more data exists)
	// Actually, with the < perPage check, it should only make 1 request
	// Let me fix the logic - if we get exactly perPage items, we need to check the next page
	// So it should make 2 requests
	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}
