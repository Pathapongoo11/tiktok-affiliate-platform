-- +goose Up
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
CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_posts_status ON posts(status);
CREATE INDEX idx_posts_scheduled_at ON posts(scheduled_at);

-- +goose Down
DROP TYPE IF EXISTS post_status;
DROP TABLE IF EXISTS posts;
