package storage

import (
	"errors"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestNewSQLiteStore(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSaveAndGet(t *testing.T) {
	store := newTestStore(t)

	snapshot := &Snapshot{
		UserID:    "user123",
		Timestamp: time.Now().Truncate(time.Second),
		Activity: map[string]interface{}{
			"commits":       float64(5),
			"pull_requests": float64(2),
			"reviews":       float64(3),
		},
	}

	err := store.Save(snapshot)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if snapshot.ID == 0 {
		t.Error("expected snapshot ID to be set after save")
	}

	retrieved, err := store.Get(snapshot.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.UserID != snapshot.UserID {
		t.Errorf("UserID mismatch: got %q, want %q", retrieved.UserID, snapshot.UserID)
	}

	if !retrieved.Timestamp.Equal(snapshot.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", retrieved.Timestamp, snapshot.Timestamp)
	}

	if retrieved.Activity["commits"] != snapshot.Activity["commits"] {
		t.Errorf("Activity commits mismatch: got %v, want %v",
			retrieved.Activity["commits"], snapshot.Activity["commits"])
	}
}

func TestSaveNil(t *testing.T) {
	store := newTestStore(t)

	err := store.Save(nil)
	if err == nil {
		t.Error("expected error when saving nil snapshot")
	}
}

func TestSaveUpdate(t *testing.T) {
	store := newTestStore(t)

	snapshot := &Snapshot{
		UserID:    "user123",
		Timestamp: time.Now().Truncate(time.Second),
		Activity:  map[string]interface{}{"commits": float64(5)},
	}

	err := store.Save(snapshot)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	snapshot.Activity["commits"] = float64(10)
	err = store.Save(snapshot)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := store.Get(snapshot.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Activity["commits"] != float64(10) {
		t.Errorf("Expected updated commits to be 10, got %v", retrieved.Activity["commits"])
	}
}

func TestGetNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByUser(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		snapshot := &Snapshot{
			UserID:    "user123",
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Activity:  map[string]interface{}{"index": float64(i)},
		}
		err := store.Save(snapshot)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Add one for a different user
	err := store.Save(&Snapshot{
		UserID:    "otheruser",
		Timestamp: now,
		Activity:  map[string]interface{}{"index": float64(99)},
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	snapshots, err := store.GetByUser("user123", 10)
	if err != nil {
		t.Fatalf("GetByUser failed: %v", err)
	}

	if len(snapshots) != 5 {
		t.Errorf("expected 5 snapshots, got %d", len(snapshots))
	}

	// Should be in descending order by timestamp
	if snapshots[0].Activity["index"] != float64(4) {
		t.Errorf("expected first snapshot to have index 4, got %v", snapshots[0].Activity["index"])
	}
}

func TestGetByUserLimit(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	for i := 0; i < 10; i++ {
		err := store.Save(&Snapshot{
			UserID:    "user123",
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Activity:  map[string]interface{}{"index": float64(i)},
		})
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	snapshots, err := store.GetByUser("user123", 3)
	if err != nil {
		t.Fatalf("GetByUser failed: %v", err)
	}

	if len(snapshots) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(snapshots))
	}
}

func TestGetByUserDefaultLimit(t *testing.T) {
	store := newTestStore(t)

	err := store.Save(&Snapshot{
		UserID:   "user123",
		Activity: map[string]interface{}{"test": true},
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Passing 0 should use default limit
	snapshots, err := store.GetByUser("user123", 0)
	if err != nil {
		t.Fatalf("GetByUser failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestGetByTimeRange(t *testing.T) {
	store := newTestStore(t)

	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Create snapshots at different times
	times := []time.Time{
		baseTime.Add(-48 * time.Hour), // 2 days before
		baseTime.Add(-24 * time.Hour), // 1 day before
		baseTime,                      // base time
		baseTime.Add(24 * time.Hour),  // 1 day after
		baseTime.Add(48 * time.Hour),  // 2 days after
	}

	for i, ts := range times {
		err := store.Save(&Snapshot{
			UserID:    "user123",
			Timestamp: ts,
			Activity:  map[string]interface{}{"day": float64(i)},
		})
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Query for middle 3 days
	start := baseTime.Add(-24 * time.Hour)
	end := baseTime.Add(24 * time.Hour)

	snapshots, err := store.GetByTimeRange("user123", start, end)
	if err != nil {
		t.Fatalf("GetByTimeRange failed: %v", err)
	}

	if len(snapshots) != 3 {
		t.Errorf("expected 3 snapshots in range, got %d", len(snapshots))
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)

	snapshot := &Snapshot{
		UserID:   "user123",
		Activity: map[string]interface{}{"test": true},
	}

	err := store.Save(snapshot)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = store.Delete(snapshot.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(snapshot.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Delete(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSaveDefaultTimestamp(t *testing.T) {
	store := newTestStore(t)

	before := time.Now()
	snapshot := &Snapshot{
		UserID:   "user123",
		Activity: map[string]interface{}{"test": true},
	}

	err := store.Save(snapshot)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	after := time.Now()

	retrieved, err := store.Get(snapshot.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Timestamp.Before(before) || retrieved.Timestamp.After(after) {
		t.Errorf("timestamp %v not in expected range [%v, %v]",
			retrieved.Timestamp, before, after)
	}
}

func TestStoreInterface(t *testing.T) {
	// Verify SQLiteStore implements Store interface
	var _ Store = (*SQLiteStore)(nil)
}
