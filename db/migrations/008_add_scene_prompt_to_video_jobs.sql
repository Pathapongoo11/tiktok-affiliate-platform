-- +goose Up
ALTER TABLE video_jobs
    ADD COLUMN IF NOT EXISTS scene_prompt TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE video_jobs DROP COLUMN IF EXISTS scene_prompt;
