package models

import "time"

// MediaType restricts media to the three kinds the brief requires.
type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVideo MediaType = "video"
	MediaBlank MediaType = "blank"
)

// MediaItem is a GLOBAL piece of media — not owned by any one window.
// This is deliberate: the sync feature requires that a single item
// (e.g. M2) be displayable in EVERY window simultaneously, including
// windows whose own playlist never normally plays it. Scoping media
// to a window would make that impossible without duplicating data.
type MediaItem struct {
	ID              string    `json:"id"`
	Label           string    `json:"label"` // e.g. "M2" — human-friendly reference
	Type            MediaType `json:"type"`
	URL             string    `json:"url,omitempty"` // empty for blank
	DurationSeconds int       `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

// Window is a display surface with its own ordered playlist.
type Window struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// PlaylistEntry ties a MediaItem into a Window's sequence at a given
// position. Position is 0-indexed and defines play order.
type PlaylistEntry struct {
	ID        string     `json:"id"`
	WindowID  string     `json:"window_id"`
	MediaItem *MediaItem `json:"media_item"`
	Position  int        `json:"position"`
}

// SyncState represents the single, currently-active sync override
// (if any). Only one sync can be active at a time — a new sync
// trigger replaces the previous one rather than queuing.
type SyncState struct {
	MediaItem       *MediaItem `json:"media_item"`
	StartedAt       time.Time  `json:"started_at"`
	DurationSeconds int        `json:"duration_seconds"`
}

// IsActive reports whether this sync override should still be
// showing, given the current time.
func (s *SyncState) IsActive(now time.Time) bool {
	if s == nil {
		return false
	}
	return now.Before(s.StartedAt.Add(time.Duration(s.DurationSeconds) * time.Second))
}