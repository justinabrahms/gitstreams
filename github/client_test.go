package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
