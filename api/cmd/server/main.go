package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/cache"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/config"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/db"
	apimiddleware "github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/auth"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/batch"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/dashboard"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/products"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/tiktok"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	redisClient := cache.NewRedis(cfg.RedisURL)
	ristrettoCache, err := cache.NewRistretto()
	if err != nil {
		log.Fatalf("Cache init failed: %v", err)
	}

	// Uploads directory for generated videos and images
	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		log.Fatalf("create uploads dir: %v", err)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(apimiddleware.CORS())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"tiktok-affiliate-api"}`))
	})

	// Serve generated videos, uploaded images, and audio files.
	// /uploads/* → <uploadsDir>/ on the server filesystem.
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))

	r.Route("/api", func(r chi.Router) {
		// ── Auth (public) ──────────────────────────────────────────────
		authSvc := auth.NewService(auth.NewRepository(pool), cfg.JWTSecret)
		authHandler := auth.NewHandler(authSvc)

		r.Group(func(r chi.Router) {
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
		})

		// ── Protected routes (JWT required) ───────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.JWT(cfg.JWTSecret))

			r.Get("/auth/me", authHandler.Me)

			// Products
			productsHandler := products.NewHandler(
				products.NewService(products.NewRepository(pool), ristrettoCache),
			)
			r.Mount("/products", productsHandler.Routes())

			// Posts
			postsSvc := posts.NewService(posts.NewRepository(pool))
			postsHandler := posts.NewHandler(postsSvc)
			r.Mount("/posts", postsHandler.Routes())

			// Content factory — batch content calendar from a product list
			batchHandler := batch.NewHandler(batch.NewService(postsSvc, postsSvc))
			r.Mount("/batch", batchHandler.Routes())

			// Dashboard
			dashboardHandler := dashboard.NewHandler(dashboard.NewService(pool, redisClient))
			r.Mount("/dashboard", dashboardHandler.Routes())

			// TikTok integration
			tiktokClient := tiktok.NewClient(cfg)
			tiktokRepo := tiktok.NewRepository(pool)
			tiktokHandler := tiktok.NewHandler(tiktokClient, tiktokRepo)
			r.Mount("/tiktok", tiktokHandler.Routes())

			// Video engine
			videoSvc := videos.NewService(pool, uploadsDir, cfg.HuggingFaceToken, cfg.LipsyncURL)
			videoHandler := videos.NewHandler(videoSvc, uploadsDir)
			r.Mount("/videos", videoHandler.Routes())
		})
	})

	// ── Background workers ──────────────────────────────────────────────
	// TikTok post scheduler: picks up scheduled posts and publishes them
	tiktokClient := tiktok.NewClient(cfg)
	scheduler := tiktok.NewScheduler(pool, tiktokClient)
	go scheduler.Start(ctx)

	// Video stall-recovery worker: marks stuck "processing" jobs as failed
	videoWorker := videos.NewWorker(pool, 30*time.Second)
	videoWorker.Start(ctx)
	defer videoWorker.Stop()

	log.Printf("API server starting on :%s (env: %s)", cfg.Port, cfg.Env)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
