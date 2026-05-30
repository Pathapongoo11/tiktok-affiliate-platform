package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	JWTSecret          string
	TikTokClientKey    string
	TikTokClientSecret string
	TikTokRedirectURI  string
	HuggingFaceToken   string
	Env                string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        buildDatabaseURL(),
		RedisURL:           getEnv("REDIS_URL", "localhost:6379"),
		JWTSecret:          getEnv("JWT_SECRET", "dev_secret"),
		TikTokClientKey:    getEnv("TIKTOK_CLIENT_KEY", ""),
		TikTokClientSecret: getEnv("TIKTOK_CLIENT_SECRET", ""),
		TikTokRedirectURI:  getEnv("TIKTOK_REDIRECT_URI", "http://localhost:3000/auth/tiktok/callback"),
		HuggingFaceToken:   getEnv("HUGGINGFACE_TOKEN", ""),
		Env:                getEnv("ENV", "development"),
	}
}

func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "tiktok")
	pass := getEnv("DB_PASSWORD", "tiktok_secret")
	name := getEnv("DB_NAME", "tiktok_affiliate")
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=disable"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
