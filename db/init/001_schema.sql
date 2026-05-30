-- ============================================================
-- TikTok Affiliate Platform — initial schema
-- This file is used by Docker postgres initdb only.
-- Source of truth for schema changes: db/migrations/*.sql
-- ============================================================

-- 001 users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    display_name VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 002 tiktok_accounts
CREATE TABLE IF NOT EXISTS tiktok_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tiktok_user_id VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tiktok_accounts_user_id ON tiktok_accounts(user_id);

-- 003 products
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    price NUMERIC(10,2),
    commission_rate NUMERIC(5,2),
    category VARCHAR(255),
    shop_product_id VARCHAR(255),
    image_urls JSONB DEFAULT '[]',
    score NUMERIC(4,2) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_products_user_id ON products(user_id);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);

-- 004 posts
CREATE TYPE post_status AS ENUM ('draft', 'scheduled', 'posting', 'published', 'failed');

CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tiktok_account_id UUID REFERENCES tiktok_accounts(id),
    product_id UUID REFERENCES products(id),
    title VARCHAR(500),
    caption TEXT,
    hashtags TEXT[],
    video_path VARCHAR(500),
    tiktok_video_id VARCHAR(255),
    status post_status DEFAULT 'draft',
    scheduled_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
CREATE INDEX IF NOT EXISTS idx_posts_scheduled_at ON posts(scheduled_at);

-- 005 video_jobs
CREATE TYPE job_status AS ENUM ('pending', 'processing', 'done', 'failed');

CREATE TABLE IF NOT EXISTS video_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status job_status DEFAULT 'pending',
    input_images JSONB DEFAULT '[]',
    overlay_text TEXT,
    audio_path VARCHAR(500),
    output_path VARCHAR(500),
    duration_seconds INT,
    animation_style VARCHAR(20) NOT NULL DEFAULT '',
    scene_prompt TEXT NOT NULL DEFAULT '',
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_video_jobs_user_id ON video_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_video_jobs_status ON video_jobs(status);

-- 006 analytics
CREATE TABLE IF NOT EXISTS analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    views BIGINT DEFAULT 0,
    likes BIGINT DEFAULT 0,
    comments BIGINT DEFAULT 0,
    shares BIGINT DEFAULT 0,
    basket_clicks BIGINT DEFAULT 0,
    orders BIGINT DEFAULT 0,
    revenue NUMERIC(12,2) DEFAULT 0,
    recorded_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_analytics_post_id ON analytics(post_id);
CREATE INDEX IF NOT EXISTS idx_analytics_user_id ON analytics(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_recorded_at ON analytics(recorded_at);
