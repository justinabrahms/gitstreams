// Package storage provides persistence for user activity snapshots.
package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested snapshot doesn't exist.
var ErrNotFound = errors.New("snapshot not found")

// Snapshot represents a point-in-time record of user activity.
type Snapshot struct {
	Activity  map[string]interface{} `json:"activity"`
	Timestamp time.Time              `json:"timestamp"`
	UserID    string                 `json:"user_id"`
	ID        int64                  `json:"id"`
}

// Store defines the interface for snapshot storage operations.
type Store interface {
	Save(snapshot *Snapshot) error
	Get(id int64) (*Snapshot, error)
	GetByUser(userID string, limit int) ([]*Snapshot, error)
	GetByTimeRange(userID string, start, end time.Time) ([]*Snapshot, error)
	Delete(id int64) error
	Close() error
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed store.
// Use ":memory:" for an in-memory database or a file path for persistence.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		activity_json TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_snapshots_user_id ON snapshots(user_id);
	CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON snapshots(timestamp);
	CREATE INDEX IF NOT EXISTS idx_snapshots_user_timestamp ON snapshots(user_id, timestamp);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Save stores a snapshot. If the snapshot has no ID, a new record is created.
// On insert, the snapshot's ID is updated with the generated value.
func (s *SQLiteStore) Save(snapshot *Snapshot) error {
	if snapshot == nil {
		return errors.New("snapshot cannot be nil")
	}

	activityJSON, err := json.Marshal(snapshot.Activity)
	if err != nil {
		return fmt.Errorf("marshaling activity: %w", err)
	}

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	if snapshot.ID == 0 {
		result, err := s.db.Exec(
			"INSERT INTO snapshots (user_id, timestamp, activity_json) VALUES (?, ?, ?)",
			snapshot.UserID, snapshot.Timestamp, string(activityJSON),
		)
		if err != nil {
			return fmt.Errorf("inserting snapshot: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("getting last insert id: %w", err)
		}
		snapshot.ID = id
	} else {
		_, err := s.db.Exec(
			"UPDATE snapshots SET user_id = ?, timestamp = ?, activity_json = ? WHERE id = ?",
			snapshot.UserID, snapshot.Timestamp, string(activityJSON), snapshot.ID,
		)
		if err != nil {
			return fmt.Errorf("updating snapshot: %w", err)
		}
	}

	return nil
}

// Get retrieves a snapshot by ID.
func (s *SQLiteStore) Get(id int64) (*Snapshot, error) {
	row := s.db.QueryRow(
		"SELECT id, user_id, timestamp, activity_json FROM snapshots WHERE id = ?",
		id,
	)

	return s.scanSnapshot(row)
}

// GetByUser retrieves the most recent snapshots for a user, up to limit.
func (s *SQLiteStore) GetByUser(userID string, limit int) (snapshots []*Snapshot, err error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		"SELECT id, user_id, timestamp, activity_json FROM snapshots WHERE user_id = ? ORDER BY timestamp DESC LIMIT ?",
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing rows: %w", cerr)
		}
	}()

	return s.scanSnapshots(rows)
}

// GetByTimeRange retrieves snapshots for a user within a time range.
func (s *SQLiteStore) GetByTimeRange(userID string, start, end time.Time) (snapshots []*Snapshot, err error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, timestamp, activity_json FROM snapshots WHERE user_id = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp DESC",
		userID, start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots by time range: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing rows: %w", cerr)
		}
	}()

	return s.scanSnapshots(rows)
}

// Delete removes a snapshot by ID.
func (s *SQLiteStore) Delete(id int64) error {
	result, err := s.db.Exec("DELETE FROM snapshots WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting snapshot: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (s *SQLiteStore) scanSnapshot(row scanner) (*Snapshot, error) {
	var snapshot Snapshot
	var activityJSON string

	err := row.Scan(&snapshot.ID, &snapshot.UserID, &snapshot.Timestamp, &activityJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning snapshot: %w", err)
	}

	if err := json.Unmarshal([]byte(activityJSON), &snapshot.Activity); err != nil {
		return nil, fmt.Errorf("unmarshaling activity: %w", err)
	}

	return &snapshot, nil
}

func (s *SQLiteStore) scanSnapshots(rows *sql.Rows) ([]*Snapshot, error) {
	var snapshots []*Snapshot
	for rows.Next() {
		var snapshot Snapshot
		var activityJSON string

		if err := rows.Scan(&snapshot.ID, &snapshot.UserID, &snapshot.Timestamp, &activityJSON); err != nil {
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}

		if err := json.Unmarshal([]byte(activityJSON), &snapshot.Activity); err != nil {
			return nil, fmt.Errorf("unmarshaling activity: %w", err)
		}

		snapshots = append(snapshots, &snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return snapshots, nil
}
