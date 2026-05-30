package videos

import "github.com/google/uuid"

// Animation style constants for VideoConfig.AnimationStyle and CreateJobRequest.AnimationStyle.
const (
	// FFmpeg-based styles (no external API, always available)
	StyleKenBurns = "ken_burns" // slow zoom-in on each image (default animated)
	StyleZoomOut  = "zoom_out"  // slow zoom-out on each image
	StyleSlide    = "slide"     // slow pan left→right on each image
	StyleStatic   = "static"    // no motion — original concat-demuxer slideshow

	// AI-based styles (require HUGGINGFACE_TOKEN — fall back to ken_burns if unset)
	StyleAIVideo   = "ai_video"   // Stable Video Diffusion: realistic motion from the photo
	StyleAICartoon = "ai_cartoon" // 2-step: cartoonize the image, then animate it
)

// IsAIStyle reports whether the given style requires the Hugging Face AI pipeline.
func IsAIStyle(style string) bool {
	return style == StyleAIVideo || style == StyleAICartoon
}

// CreateJobRequest is the payload for POST /api/videos/generate.
type CreateJobRequest struct {
	PostID         *uuid.UUID `json:"post_id,omitempty"`
	InputImages    []string   `json:"input_images"`
	OverlayText    string     `json:"overlay_text"`
	AudioPath      string     `json:"audio_path,omitempty"`
	DurationSec    int        `json:"duration_seconds"`
	AnimationStyle string     `json:"animation_style,omitempty"` // "" | "ken_burns" | "zoom_out" | "slide" | "static"
}

// VideoConfig holds FFmpeg generation parameters.
type VideoConfig struct {
	InputImages    []string // file paths to product images
	OverlayText    string   // text to overlay on the video
	AudioPath      string   // optional background music path
	OutputPath     string   // destination MP4 path
	DurationSec    int      // target duration in seconds (15–60)
	FPS            int      // frames per second (default 1 for slideshow, 30 for animated)
	AnimationStyle string   // "" / "ken_burns" / "zoom_out" / "slide" / "static"
}
