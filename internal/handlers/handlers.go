package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"media-sequencer-backend/internal/models"
	"media-sequencer-backend/internal/repository"
	"media-sequencer-backend/internal/service"
)

type API struct {
	store *repository.Store
}

func NewAPI(store *repository.Store) *API {
	return &API{store: store}
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ListWindows(w http.ResponseWriter, r *http.Request) {
	windows, err := a.store.ListWindows()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusOK, windows)
}

// NowPlaying handles GET /windows/{id}/now-playing — the single most
// important endpoint in the system. Every window in the frontend
// polls this every couple of seconds. It applies the sync override
// (if active) on top of the window's own deterministic cycle
// position; see service.ResolveNowPlaying for how those two combine.
func (a *API) NowPlaying(w http.ResponseWriter, r *http.Request) {
	windowID := r.PathValue("id")

	if _, err := a.store.GetWindow(windowID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "window not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	entries, err := a.store.PlaylistForWindow(windowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	sync, err := a.store.GetSyncState()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	result := service.ResolveNowPlaying(entries, sync, time.Now())
	if result == nil {
		respondError(w, http.StatusNotFound, "window has no playlist configured")
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (a *API) ListMedia(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListMediaItems()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusOK, items)
}

type createMediaRequest struct {
	Label           string `json:"label"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	DurationSeconds int    `json:"duration_seconds"`
}

// CreateMedia handles POST /media — registers a brand new global
// media item that can then be added to any window's playlist, or
// used as a sync target.
func (a *API) CreateMedia(w http.ResponseWriter, r *http.Request) {
	var req createMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mediaType := models.MediaType(req.Type)
	if mediaType != models.MediaImage && mediaType != models.MediaVideo && mediaType != models.MediaBlank {
		respondError(w, http.StatusBadRequest, "type must be one of: image, video, blank")
		return
	}
	if req.DurationSeconds <= 0 {
		respondError(w, http.StatusBadRequest, "duration_seconds must be positive")
		return
	}
	if mediaType != models.MediaBlank && req.URL == "" {
		respondError(w, http.StatusBadRequest, "url is required for image/video media")
		return
	}

	item := &models.MediaItem{
		ID:              models.NewID(),
		Label:           req.Label,
		Type:            mediaType,
		URL:             req.URL,
		DurationSeconds: req.DurationSeconds,
		CreatedAt:       time.Now().UTC(),
	}
	if err := a.store.CreateMediaItem(item); err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

type addToPlaylistRequest struct {
	MediaItemID string `json:"media_item_id"`
}

// AddMediaToWindow handles POST /windows/{id}/media — the "dynamic
// playlist update" requirement. Appends an EXISTING media item to
// the end of a window's playlist. (Create the media item first via
// POST /media if it doesn't exist yet.)
func (a *API) AddMediaToWindow(w http.ResponseWriter, r *http.Request) {
	windowID := r.PathValue("id")

	if _, err := a.store.GetWindow(windowID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "window not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var req addToPlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := a.store.GetMediaItem(req.MediaItemID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusBadRequest, "media_item_id does not exist")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := a.store.AppendToPlaylist(models.NewID(), windowID, req.MediaItemID); err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	entries, err := a.store.PlaylistForWindow(windowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusCreated, entries)
}

type triggerSyncRequest struct {
	MediaItemID     string `json:"media_item_id"`
	DurationSeconds int    `json:"duration_seconds"`
}

// TriggerSync handles POST /sync. This is the entire sync feature on
// the write side: it just upserts one row recording which item, and
// for how long. All the actual "make every window show this" behavior
// happens for free the next time each window polls its own
// now-playing endpoint and ResolveNowPlaying sees an active override.
func (a *API) TriggerSync(w http.ResponseWriter, r *http.Request) {
	var req triggerSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DurationSeconds <= 0 {
		respondError(w, http.StatusBadRequest, "duration_seconds must be positive")
		return
	}

	item, err := a.store.GetMediaItem(req.MediaItemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusBadRequest, "media_item_id does not exist")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now().UTC()
	if err := a.store.SetSyncState(item.ID, now, req.DurationSeconds); err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"media_item":       item,
		"started_at":       now,
		"duration_seconds": req.DurationSeconds,
	})
}

// SyncStatus handles GET /sync — lets the frontend show a "syncing
// to M2, 3s remaining" indicator if desired.
func (a *API) SyncStatus(w http.ResponseWriter, r *http.Request) {
	sync, err := a.store.GetSyncState()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if sync == nil || !sync.IsActive(time.Now()) {
		respondJSON(w, http.StatusOK, map[string]interface{}{"active": false})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"active":           true,
		"media_item":       sync.MediaItem,
		"started_at":       sync.StartedAt,
		"duration_seconds": sync.DurationSeconds,
	})
}