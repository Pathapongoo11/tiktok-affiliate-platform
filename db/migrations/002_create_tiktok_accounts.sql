-- +goose Up
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
CREATE INDEX idx_tiktok_accounts_user_id ON tiktok_accounts(user_id);

-- +goose Down
DROP TABLE IF EXISTS tiktok_accounts;
