package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
	}
	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}

	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("create uploads dir: %v", err)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "tiktok-affiliate-api",
		})
	})

	// Protected routes require a valid JWT
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWT(jwtSecret))

		if dbURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			pool, err := pgxpool.New(ctx, dbURL)
			if err != nil {
				log.Fatalf("connect to database: %v", err)
			}
			defer pool.Close()

			if err := pool.Ping(ctx); err != nil {
				log.Fatalf("ping database: %v", err)
			}

			// --- Video Engine ---
			videoService := videos.NewService(pool, uploadsDir)
			videoHandler := videos.NewHandler(videoService, uploadsDir)
			r.Mount("/api/videos", videoHandler.Routes())

			// Start stall-recovery worker
			videoWorker := videos.NewWorker(pool, 30*time.Second)
			videoWorker.Start(context.Background())
			defer videoWorker.Stop()
		} else {
			log.Println("WARNING: DATABASE_URL not set — /api/videos routes are disabled")
		}
	})

	log.Printf("API server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
