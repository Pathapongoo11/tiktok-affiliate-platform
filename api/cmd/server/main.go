package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/cache"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/config"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/db"
	apimiddleware "github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/auth"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/dashboard"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/products"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/tiktok"
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

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(apimiddleware.CORS())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"tiktok-affiliate-api"}`))
	})

	r.Route("/api", func(r chi.Router) {
		// Auth routes — register and login are public; /me requires JWT
		authSvc := auth.NewService(auth.NewRepository(pool), cfg.JWTSecret)
		authHandler := auth.NewHandler(authSvc)

		r.Group(func(r chi.Router) {
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.JWT(cfg.JWTSecret))
			r.Get("/auth/me", authHandler.Me)

			productsHandler := products.NewHandler(products.NewService(products.NewRepository(pool), ristrettoCache))
			r.Mount("/products", productsHandler.Routes())

			postsHandler := posts.NewHandler(posts.NewService(posts.NewRepository(pool)))
			r.Mount("/posts", postsHandler.Routes())

			dashboardHandler := dashboard.NewHandler(dashboard.NewService(pool, redisClient))
			r.Mount("/dashboard", dashboardHandler.Routes())

			// TikTok integration
			tiktokClient := tiktok.NewClient(cfg)
			tiktokRepo := tiktok.NewRepository(pool)
			tiktokHandler := tiktok.NewHandler(tiktokClient, tiktokRepo)
			r.Mount("/tiktok", tiktokHandler.Routes())

			// Start TikTok scheduler in background
			scheduler := tiktok.NewScheduler(pool, tiktokClient)
			go scheduler.Start(ctx)
		})
	})

	log.Printf("API server starting on :%s (env: %s)", cfg.Port, cfg.Env)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
