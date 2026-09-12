package repository

import (
	"database/sql"
	"errors"
	"time"

	"media-sequencer-backend/internal/models"
)

var ErrNotFound = errors.New("not found")

// Store wraps a *sql.DB. All queries are plain SQL via the standard
// library's database/sql — no ORM. For a schema this small (4
// tables, no complex joins beyond one 3-way join), an ORM would add
// a dependency and an abstraction layer without buying back much;
// hand-written SQL stays easier to reason about and to show in a
// discussion round.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListWindows() ([]*models.Window, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM windows ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Window
	for rows.Next() {
		w := &models.Window{}
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) GetWindow(id string) (*models.Window, error) {
	w := &models.Window{}
	err := s.db.QueryRow(`SELECT id, name, created_at FROM windows WHERE id = $1`, id).
		Scan(&w.ID, &w.Name, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

// PlaylistForWindow returns a window's playlist entries, each with
// its full MediaItem populated, in position order — exactly what
// service.ComputeNowPlaying needs.
func (s *Store) PlaylistForWindow(windowID string) ([]*models.PlaylistEntry, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.window_id, p.position,
		       m.id, m.label, m.type, m.url, m.duration_seconds, m.created_at
		FROM playlist_entries p
		JOIN media_items m ON m.id = p.media_item_id
		WHERE p.window_id = $1
		ORDER BY p.position ASC`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.PlaylistEntry
	for rows.Next() {
		e := &models.PlaylistEntry{MediaItem: &models.MediaItem{}}
		if err := rows.Scan(
			&e.ID, &e.WindowID, &e.Position,
			&e.MediaItem.ID, &e.MediaItem.Label, &e.MediaItem.Type,
			&e.MediaItem.URL, &e.MediaItem.DurationSeconds, &e.MediaItem.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateMediaItem inserts a brand new global media item.
func (s *Store) CreateMediaItem(m *models.MediaItem) error {
	_, err := s.db.Exec(`
		INSERT INTO media_items (id, label, type, url, duration_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		m.ID, m.Label, m.Type, m.URL, m.DurationSeconds, m.CreatedAt)
	return err
}

func (s *Store) GetMediaItem(id string) (*models.MediaItem, error) {
	m := &models.MediaItem{}
	err := s.db.QueryRow(`
		SELECT id, label, type, url, duration_seconds, created_at
		FROM media_items WHERE id = $1`, id,
	).Scan(&m.ID, &m.Label, &m.Type, &m.URL, &m.DurationSeconds, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *Store) ListMediaItems() ([]*models.MediaItem, error) {
	rows, err := s.db.Query(`SELECT id, label, type, url, duration_seconds, created_at FROM media_items ORDER BY label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.MediaItem
	for rows.Next() {
		m := &models.MediaItem{}
		if err := rows.Scan(&m.ID, &m.Label, &m.Type, &m.URL, &m.DurationSeconds, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AppendToPlaylist adds mediaItemID to the END of windowID's
// playlist (dynamic playlist update, as required by the brief).
//
// This runs inside a transaction holding a Postgres advisory lock
// scoped to windowID (via pg_advisory_xact_lock, keyed by a hash of
// the window ID). That serializes concurrent "add media" requests
// to the SAME window — two requests racing to add to Window 1 will
// queue rather than both computing the same next position and one
// failing on the UNIQUE(window_id, position) constraint. Requests
// to DIFFERENT windows are unaffected and still run fully in
// parallel, since each window has its own lock key.
func (s *Store) AppendToPlaylist(entryID, windowID, mediaItemID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op if Commit succeeds

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, windowID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO playlist_entries (id, window_id, media_item_id, position)
		SELECT $1, $2, $3, COALESCE(MAX(position) + 1, 0)
		FROM playlist_entries WHERE window_id = $2`,
		entryID, windowID, mediaItemID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetSyncState returns the current sync override row, or nil if
// none has ever been set.
func (s *Store) GetSyncState() (*models.SyncState, error) {
	var mediaID sql.NullString
	var startedAt sql.NullTime
	var durationSec sql.NullInt64

	err := s.db.QueryRow(`SELECT media_item_id, started_at, duration_seconds FROM sync_state WHERE id = 1`).
		Scan(&mediaID, &startedAt, &durationSec)
	if errors.Is(err, sql.ErrNoRows) || !mediaID.Valid {
		return nil, nil // no sync ever triggered — not an error
	}
	if err != nil {
		return nil, err
	}

	item, err := s.GetMediaItem(mediaID.String)
	if err != nil {
		return nil, err
	}
	return &models.SyncState{
		MediaItem:       item,
		StartedAt:       startedAt.Time,
		DurationSeconds: int(durationSec.Int64),
	}, nil
}

// SetSyncState upserts the single sync_state row. Using a fixed
// id=1 row (rather than INSERT-ing a new row per sync) means
// "trigger sync" is always a single, atomic UPSERT — there is never
// more than one active sync to reconcile.
func (s *Store) SetSyncState(mediaItemID string, startedAt time.Time, durationSeconds int) error {
	_, err := s.db.Exec(`
		INSERT INTO sync_state (id, media_item_id, started_at, duration_seconds)
		VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			media_item_id = $1, started_at = $2, duration_seconds = $3`,
		mediaItemID, startedAt, durationSeconds)
	return err
}