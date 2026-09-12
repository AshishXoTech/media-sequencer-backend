// Command server starts the Multi-Window Media Sequencer backend.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"

	"media-sequencer-backend/internal/config"
	"media-sequencer-backend/internal/handlers"
	"media-sequencer-backend/internal/repository"
	"media-sequencer-backend/internal/router"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

		if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Free-tier Postgres providers (Neon included) cap concurrent
	// connections. Without an explicit limit, Go's default pool is
	// effectively unbounded and can exhaust that cap under load,
	// causing every subsequent request to fail with a connection
	// error instead of just queuing briefly. 10 is comfortably under
	// typical free-tier limits for a service this size.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := repository.NewStore(db)
	api := handlers.NewAPI(store)
	mux := router.New(api)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("media-sequencer-backend listening on port %s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}