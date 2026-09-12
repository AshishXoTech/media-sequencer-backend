package service

import (
	"time"

	"media-sequencer-backend/internal/models"
)

// CycleDuration is the fixed 5-hour cycle length required by the
// brief. Using a package-level constant (not a config value) because
// it's a hard requirement of the spec, not something an operator
// should be able to tune.
const CycleDuration = 5 * time.Hour

// epoch is a fixed reference point in time that every window's cycle
// is measured from. Using a single shared epoch (rather than each
// window's own creation time) means all windows' 5-hour boundaries
// land at the same wall-clock moments — useful for reasoning about
// the system, and avoids one window's cycle silently drifting from
// another's if it were re-created later.
var epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// NowPlayingResult is what the API returns for "what should this
// window be showing right now."
type NowPlayingResult struct {
	MediaItem         *models.MediaItem `json:"media_item"`
	PositionInItemSec int               `json:"position_in_item_seconds"`
	Synced            bool              `json:"synced"`
}

// ComputeNowPlaying is the entire playback engine in one pure
// function: given a playlist and the current time, it deterministically
// derives exactly what should be on screen. There is no persisted
// "current index" or "elapsed timer" anywhere in the system — nothing
// to lose on a restart, nothing to desync between two browser tabs
// polling the same window. Two callers asking at the same instant
// always get the same answer, by construction.
func ComputeNowPlaying(entries []*models.PlaylistEntry, now time.Time) *NowPlayingResult {
	if len(entries) == 0 {
		return nil
	}

	playlistDuration := 0
	for _, e := range entries {
		playlistDuration += e.MediaItem.DurationSeconds
	}
	if playlistDuration == 0 {
		return nil // guard against a playlist of zero-duration items
	}

	elapsedSinceEpoch := now.Sub(epoch)

	// Step 1: where are we inside the current 5-hour cycle?
	// This is what makes the list snap back to item 0 exactly every
	// 5 hours, rather than drifting — a plain "loop forever" modulo
	// by playlist duration alone would never realign to a 5-hour
	// boundary unless the playlist duration happened to divide it evenly.
	cyclePos := int64(elapsedSinceEpoch/time.Second) % int64(CycleDuration/time.Second)

	// Step 2: within that cycle position, where are we inside one
	// loop of the playlist? (the list repeats many times per 5h cycle)
	posInPlaylist := int(cyclePos % int64(playlistDuration))

	// Step 3: walk the playlist to find which item that position
	// falls inside, and how far into that item we are.
	acc := 0
	for _, e := range entries {
		next := acc + e.MediaItem.DurationSeconds
		if posInPlaylist < next {
			return &NowPlayingResult{
				MediaItem:         e.MediaItem,
				PositionInItemSec: posInPlaylist - acc,
			}
		}
		acc = next
	}

	last := entries[len(entries)-1]
	return &NowPlayingResult{MediaItem: last.MediaItem, PositionInItemSec: 0}
}

// ResolveNowPlaying applies the sync override (if active) on top of
// ComputeNowPlaying's normal deterministic result. This is the ENTIRE
// sync feature: a time-bounded override checked first. When it
// expires, this function stops returning it — the normal cycle
// resumes automatically on the very next poll, with no separate
// "resume" code path and no risk of a window forgetting its own
// configuration, because that configuration was never overwritten,
// only shadowed.
func ResolveNowPlaying(entries []*models.PlaylistEntry, sync *models.SyncState, now time.Time) *NowPlayingResult {
	if sync != nil && sync.IsActive(now) {
		return &NowPlayingResult{
			MediaItem:         sync.MediaItem,
			PositionInItemSec: int(now.Sub(sync.StartedAt) / time.Second),
			Synced:            true,
		}
	}
	return ComputeNowPlaying(entries, now)
}