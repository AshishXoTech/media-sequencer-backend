package router

import (
	"net/http"

	"media-sequencer-backend/internal/handlers"
	"media-sequencer-backend/internal/middleware"
)

func New(api *handlers.API) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", api.Health)
	mux.HandleFunc("GET /windows", api.ListWindows)
	mux.HandleFunc("GET /windows/{id}/now-playing", api.NowPlaying)
	mux.HandleFunc("POST /windows/{id}/media", api.AddMediaToWindow)
	mux.HandleFunc("GET /media", api.ListMedia)
	mux.HandleFunc("POST /media", api.CreateMedia)
	mux.HandleFunc("POST /sync", api.TriggerSync)
	mux.HandleFunc("GET /sync", api.SyncStatus)

	return middleware.CORS(mux)
}