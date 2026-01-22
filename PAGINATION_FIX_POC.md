# Proof of Concept: Link Header Pagination Fix

## Problem

Current code makes an extra API call when result set has exactly 100 items, because it can't distinguish between:
- "We got 100 items and there are more"
- "We got 100 items and that's all there is"

## Solution

Use GitHub's `Link` header which explicitly tells us if there's a next page.

## Implementation

### Step 1: Update `get()` method to return Link header

```go
// get makes an HTTP GET request and returns the response headers along with the decoded result
func (c *Client) get(ctx context.Context, path string, result any) (*http.Header, error) {
    // ... existing code ...

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("executing request: %w", err)
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
                return nil, fmt.Errorf("decoding cached response: %w", unmarshalErr)
            }
        }
        return &resp.Header, nil  // Return headers even for cached responses
    }

    // ... existing code for non-cached responses ...

    if result != nil {
        if err := json.Unmarshal(body, result); err != nil {
            return nil, fmt.Errorf("decoding response: %w", err)
        }
    }

    return &resp.Header, nil  // Return headers
}
```

### Step 2: Update `getPaginated()` to check Link header

```go
// hasNextPage checks if the Link header contains rel="next"
func hasNextPage(linkHeader string) bool {
    // GitHub Link header format:
    // Link: <https://api.github.com/user/repos?page=2>; rel="next", <https://api.github.com/user/repos?page=3>; rel="last"
    return strings.Contains(linkHeader, `rel="next"`)
}

// getPaginated fetches all pages of results for a given path.
// It uses GitHub's Link header to detect when there are no more pages.
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

        headers, err := c.get(ctx, path, pageResult)
        if err != nil {
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

        // Check Link header to see if there's a next page
        // This is more reliable than checking if we got fewer than perPage items
        linkHeader := ""
        if headers != nil {
            linkHeader = headers.Get("Link")
        }

        if !hasNextPage(linkHeader) {
            // No next page indicated by GitHub, we're done
            break
        }

        page++
    }

    // Set the final result
    resultVal.Elem().Set(sliceVal)
    return nil
}
```

### Step 3: Update tests to expect 1 call for exactly 100 items

```go
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
            // No Link header with rel="next" indicates this is the last page
            // When there are more pages, GitHub includes:
            // Link: <url?page=2>; rel="next"
            if err := json.NewEncoder(w).Encode(repos); err != nil {
                t.Fatalf("encoding repos: %v", err)
            }
        } else {
            t.Errorf("should not request page %s when page 1 has no Link header with rel=\"next\"", page)
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

    // Should now make exactly 1 request when Link header indicates no next page
    if requestCount != 1 {
        t.Errorf("expected 1 request, got %d", requestCount)
    }
}
```

### Step 4: Add test for multi-page with Link headers

```go
func TestGetFollowedUsersPaginationWithLinkHeaders(t *testing.T) {
    // Test proper Link header pagination for 202 users across 3 pages
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
        query := r.URL.Query()
        page := query.Get("page")

        w.Header().Set("Content-Type", "application/json")

        switch page {
        case "1":
            // Page 1 has a next page
            w.Header().Set("Link", `<http://test/user/following?page=2&per_page=100>; rel="next"`)
            json.NewEncoder(w).Encode(page1Users)
        case "2":
            // Page 2 has a next page
            w.Header().Set("Link", `<http://test/user/following?page=3&per_page=100>; rel="next"`)
            json.NewEncoder(w).Encode(page2Users)
        case "3":
            // Page 3 is the last page (no next link)
            json.NewEncoder(w).Encode(page3Users)
        default:
            t.Errorf("unexpected page: %s", page)
        }
    }))
    defer server.Close()

    c := NewClient("test-token", WithBaseURL(server.URL))
    result, err := c.GetFollowedUsers(context.Background())
    if err != nil {
        t.Fatalf("GetFollowedUsers() error: %v", err)
    }

    if len(result) != 202 {
        t.Errorf("expected 202 users, got %d", len(result))
    }
}
```

## Benefits of This Fix

1. **Eliminates extra API call**: When there are exactly 100 items, we only make 1 request instead of 2
2. **More reliable**: Uses GitHub's explicit pagination signal instead of guessing
3. **Standard practice**: This is how GitHub recommends handling pagination
4. **Backward compatible**: Existing behavior is preserved, just more efficient

## Impact

For a user following 100 people where 30% have exactly 100 items in some category:
- **Before fix**: ~391 API calls (301 base + 90 extra)
- **After fix**: ~301 API calls (no extra calls)
- **Savings**: 23% reduction in API calls

## Files to Modify

1. `github/client.go` - Update `get()` return signature and `getPaginated()` logic
2. `github/client_test.go` - Update test expectations and add Link header tests
3. All call sites of `get()` method - Update to handle returned headers (or ignore them)

## Note on Compatibility

The `get()` method signature change will require updating all callers. Alternative approach:
- Create `getWithHeaders()` method
- Keep `get()` unchanged
- Use `getWithHeaders()` only in `getPaginated()`

This minimizes the change surface area.
