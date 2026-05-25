-- +goose Up
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
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_video_jobs_user_id ON video_jobs(user_id);
CREATE INDEX idx_video_jobs_status ON video_jobs(status);

-- +goose Down
DROP TYPE IF EXISTS job_status;
DROP TABLE IF EXISTS video_jobs;
