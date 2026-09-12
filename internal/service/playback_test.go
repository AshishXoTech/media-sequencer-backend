package service

import (
	"testing"
	"time"

	"media-sequencer-backend/internal/models"
)

func item(label string, seconds int) *models.MediaItem {
	return &models.MediaItem{ID: label, Label: label, Type: models.MediaImage, DurationSeconds: seconds}
}

func entries(items ...*models.MediaItem) []*models.PlaylistEntry {
	var out []*models.PlaylistEntry
	for i, it := range items {
		out = append(out, &models.PlaylistEntry{MediaItem: it, Position: i})
	}
	return out
}

func TestComputeNowPlaying_WalksPlaylistInOrder(t *testing.T) {
	list := entries(item("M1", 10), item("M2", 15), item("M3", 10))

	cases := []struct {
		secondsAfterEpoch int64
		wantLabel         string
		wantPosInItem     int
	}{
		{0, "M1", 0},
		{5, "M1", 5},
		{10, "M2", 0},
		{24, "M2", 14},
		{25, "M3", 0},
		{34, "M3", 9},
		{35, "M1", 0}, // playlist total is 35s, should have looped back to M1
	}

	for _, c := range cases {
		now := epoch.Add(time.Duration(c.secondsAfterEpoch) * time.Second)
		got := ComputeNowPlaying(list, now)
		if got.MediaItem.Label != c.wantLabel || got.PositionInItemSec != c.wantPosInItem {
			t.Errorf("at t=%ds: got (%s, %d), want (%s, %d)",
				c.secondsAfterEpoch, got.MediaItem.Label, got.PositionInItemSec, c.wantLabel, c.wantPosInItem)
		}
	}
}

func TestComputeNowPlaying_RestartsExactlyAtFiveHourBoundary(t *testing.T) {
	// Playlist total is 35s, which does NOT evenly divide 5 hours.
	// A naive "loop forever" modulo by playlist duration alone would
	// still land mid-list at the 5h mark. The two-level modulo must
	// force position back to exactly 0 at the cycle boundary.
	list := entries(item("M1", 10), item("M2", 15), item("M3", 10))

	atBoundary := epoch.Add(CycleDuration)
	at := ComputeNowPlaying(list, atBoundary)

	if at.MediaItem.Label != "M1" || at.PositionInItemSec != 0 {
		t.Errorf("at 5h boundary: got (%s, %d), want (M1, 0)", at.MediaItem.Label, at.PositionInItemSec)
	}
}

func TestResolveNowPlaying_SyncOverridesNormalSequence(t *testing.T) {
	list := entries(item("M1", 10), item("M2", 15), item("M3", 10))
	syncTarget := item("M9", 999) // an item NOT in this window's own list at all

	now := epoch.Add(3 * time.Second)
	sync := &models.SyncState{MediaItem: syncTarget, StartedAt: now, DurationSeconds: 10}

	got := ResolveNowPlaying(list, sync, now)
	if got.MediaItem.Label != "M9" || !got.Synced {
		t.Errorf("during active sync: got (%s, synced=%v), want (M9, synced=true)", got.MediaItem.Label, got.Synced)
	}
}

func TestResolveNowPlaying_ResumesNormalSequenceAfterSyncExpires(t *testing.T) {
	list := entries(item("M1", 10), item("M2", 15), item("M3", 10))
	syncTarget := item("M9", 999)

	syncStart := epoch.Add(3 * time.Second)
	sync := &models.SyncState{MediaItem: syncTarget, StartedAt: syncStart, DurationSeconds: 5}

	after := epoch.Add(9 * time.Second) // 6s after sync started, but duration was only 5s
	got := ResolveNowPlaying(list, sync, after)

	if got.MediaItem.Label != "M1" || got.Synced || got.PositionInItemSec != 9 {
		t.Errorf("after sync expiry: got (%s, synced=%v, pos=%d), want (M1, false, 9)",
			got.MediaItem.Label, got.Synced, got.PositionInItemSec)
	}
}