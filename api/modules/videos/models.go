package videos

import "github.com/google/uuid"

// CreateJobRequest is the payload for POST /api/videos/generate.
type CreateJobRequest struct {
	PostID      *uuid.UUID `json:"post_id,omitempty"`
	InputImages []string   `json:"input_images"`
	OverlayText string     `json:"overlay_text"`
	AudioPath   string     `json:"audio_path,omitempty"`
	DurationSec int        `json:"duration_seconds"`
}

// VideoConfig holds FFmpeg generation parameters.
type VideoConfig struct {
	InputImages []string // file paths to product images
	OverlayText string   // text to overlay on the video
	AudioPath   string   // optional background music path
	OutputPath  string   // destination MP4 path
	DurationSec int      // target duration in seconds (15–60)
	FPS         int      // frames per second (default 1 for slideshow)
}
