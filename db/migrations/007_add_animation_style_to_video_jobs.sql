-- +goose Up
ALTER TABLE video_jobs
    ADD COLUMN IF NOT EXISTS animation_style VARCHAR(20) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE video_jobs DROP COLUMN IF EXISTS animation_style;
