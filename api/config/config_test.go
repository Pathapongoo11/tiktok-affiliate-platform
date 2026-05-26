package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that could interfere
	keysToUnset := []string{"PORT", "ENV", "DATABASE_URL", "REDIS_URL", "JWT_SECRET",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}
	originals := map[string]string{}
	for _, k := range keysToUnset {
		originals[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	cfg := config.Load()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "development", cfg.Env)
	assert.NotEmpty(t, cfg.DatabaseURL)
	assert.Equal(t, "localhost:6379", cfg.RedisURL)
	assert.Equal(t, "dev_secret", cfg.JWTSecret)
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("ENV", "production")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("ENV")

	cfg := config.Load()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "production", cfg.Env)
}

func TestLoad_DatabaseURL_FromEnv(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://user:pass@myhost:5432/mydb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	cfg := config.Load()

	assert.Equal(t, "postgres://user:pass@myhost:5432/mydb?sslmode=disable", cfg.DatabaseURL)
}

func TestLoad_DatabaseURL_BuildFromParts(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Setenv("DB_HOST", "dbhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "myuser")
	os.Setenv("DB_PASSWORD", "mypass")
	os.Setenv("DB_NAME", "mydb")
	defer func() {
		for _, k := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
			os.Unsetenv(k)
		}
	}()

	cfg := config.Load()

	assert.Contains(t, cfg.DatabaseURL, "dbhost")
	assert.Contains(t, cfg.DatabaseURL, "5433")
	assert.Contains(t, cfg.DatabaseURL, "myuser")
	assert.Contains(t, cfg.DatabaseURL, "mydb")
}

func TestLoad_RedisURL_Override(t *testing.T) {
	os.Setenv("REDIS_URL", "myredis:6380")
	defer os.Unsetenv("REDIS_URL")

	cfg := config.Load()

	assert.Equal(t, "myredis:6380", cfg.RedisURL)
}

func TestLoad_JWTSecret_Override(t *testing.T) {
	os.Setenv("JWT_SECRET", "super_secret_key")
	defer os.Unsetenv("JWT_SECRET")

	cfg := config.Load()

	assert.Equal(t, "super_secret_key", cfg.JWTSecret)
}

func TestLoad_TikTok_Fields(t *testing.T) {
	os.Setenv("TIKTOK_CLIENT_KEY", "tk_key_123")
	os.Setenv("TIKTOK_CLIENT_SECRET", "tk_secret_456")
	os.Setenv("TIKTOK_REDIRECT_URI", "https://myapp.com/callback")
	defer func() {
		os.Unsetenv("TIKTOK_CLIENT_KEY")
		os.Unsetenv("TIKTOK_CLIENT_SECRET")
		os.Unsetenv("TIKTOK_REDIRECT_URI")
	}()

	cfg := config.Load()

	assert.Equal(t, "tk_key_123", cfg.TikTokClientKey)
	assert.Equal(t, "tk_secret_456", cfg.TikTokClientSecret)
	assert.Equal(t, "https://myapp.com/callback", cfg.TikTokRedirectURI)
}
