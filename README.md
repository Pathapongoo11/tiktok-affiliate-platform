# TikTok Affiliate Platform

TikTok Shop affiliate management platform — React + Go + PostgreSQL.

## Overview

A full-stack platform for managing TikTok Shop affiliate marketing:
- **Auth**: JWT-based authentication with TikTok OAuth integration
- **Products**: Affiliate product catalog with scoring and commission tracking
- **Posts**: Schedule and publish TikTok videos with affiliate basket pins
- **Video Jobs**: Automated video generation pipeline (images + overlay + audio)
- **Analytics**: Track views, likes, basket clicks, orders, and revenue

## Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React (Vite + TypeScript) |
| API | Go 1.23 (net/http, modular structure) |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Container | Docker + Docker Compose |
| Migrations | Goose (SQL files in `db/migrations/`) |

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.23+
- Node.js 20+

### 1. Clone and configure

```bash
git clone https://github.com/Pathapongoo11/tiktok-affiliate-platform.git
cd tiktok-affiliate-platform
cp .env.example .env
# Edit .env with your secrets
```

### 2. Start infrastructure

```bash
docker compose up postgres redis -d
```

### 3. Run API locally

```bash
cd api
go run ./cmd/server
# API available at http://localhost:8080
# Health check: GET http://localhost:8080/health
```

### 4. Run everything with Docker

```bash
docker compose up --build
```

## Project Structure

```
tiktok-affiliate-platform/
├── api/                    # Go backend
│   ├── cmd/server/         # Entry point (main.go)
│   ├── modules/            # Feature modules (auth, tiktok, products, videos, posts, dashboard)
│   ├── cache/              # Redis cache layer
│   ├── db/                 # Database helpers
│   ├── go.mod
│   └── Dockerfile
├── frontend/               # React frontend (TBD)
├── db/
│   └── migrations/         # Goose SQL migrations (001-006)
├── uploads/                # Video/image upload storage
├── docker-compose.yml
└── .env.example
```

## Database Migrations

Migrations use [Goose](https://github.com/pressly/goose) format and run automatically on `docker compose up`.

| File | Table |
|------|-------|
| 001_create_users.sql | `users` |
| 002_create_tiktok_accounts.sql | `tiktok_accounts` |
| 003_create_products.sql | `products` |
| 004_create_posts.sql | `posts` (with `post_status` enum) |
| 005_create_video_jobs.sql | `video_jobs` (with `job_status` enum) |
| 006_create_analytics.sql | `analytics` |

## Environment Variables

See `.env.example` for all required variables. Key ones:

| Variable | Description |
|----------|-------------|
| `DB_*` | PostgreSQL connection settings |
| `REDIS_URL` | Redis connection URL |
| `JWT_SECRET` | Secret for signing JWT tokens |
| `TIKTOK_CLIENT_KEY` | From TikTok Developer Portal |
| `TIKTOK_CLIENT_SECRET` | From TikTok Developer Portal |

## Branches

| Branch | Purpose |
|--------|---------|
| `main` | Production-ready releases |
| `develop` | Integration branch (default) |
| `feature/*` | Feature development |

## License

MIT
