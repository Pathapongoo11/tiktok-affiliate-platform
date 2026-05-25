-- +goose Up
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
CREATE INDEX idx_analytics_post_id ON analytics(post_id);
CREATE INDEX idx_analytics_user_id ON analytics(user_id);
CREATE INDEX idx_analytics_recorded_at ON analytics(recorded_at);

-- +goose Down
DROP TABLE IF EXISTS analytics;
