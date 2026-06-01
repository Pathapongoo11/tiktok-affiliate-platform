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
	StyleAIVideo   = "ai_video"   // FLUX scene image → Ken Burns
	StyleAICartoon = "ai_cartoon" // FLUX cartoon character → Ken Burns

	// Lip-sync style (requires LIPSYNC_URL microservice + audio — falls back to
	// ai_cartoon when unavailable). FLUX character → SadTalker talking head.
	StyleAITalking = "ai_talking"

	// Review style (GOAL 4 Phase 1): a talking presenter (FLUX + SadTalker)
	// with the real product photo (background removed via rembg) composited as
	// picture-in-picture. Requires LIPSYNC_URL + audio; falls back like ai_talking.
	StyleAIReview = "ai_review"
)

// IsAIStyle reports whether the given style uses the Hugging Face FLUX pipeline.
func IsAIStyle(style string) bool {
	return style == StyleAIVideo || style == StyleAICartoon ||
		style == StyleAITalking || style == StyleAIReview
}

// CreateJobRequest is the payload for POST /api/videos/generate.
type CreateJobRequest struct {
	PostID         *uuid.UUID `json:"post_id,omitempty"`
	InputImages    []string   `json:"input_images"`
	OverlayText    string     `json:"overlay_text"`           // marketing text burned onto the video (Thai OK)
	ScenePrompt    string     `json:"scene_prompt,omitempty"` // English scene description for AI image gen
	AudioPath      string     `json:"audio_path,omitempty"`
	TTSText        string     `json:"tts_text,omitempty"`  // ai_talking: synthesize this text to speech (Thai OK)
	TTSVoice       string     `json:"tts_voice,omitempty"` // edge-tts voice; default th-TH-PremwadeeNeural
	DurationSec    int        `json:"duration_seconds"`
	AnimationStyle string     `json:"animation_style,omitempty"` // "" | "ken_burns" | "zoom_out" | "slide" | "static" | "ai_cartoon" | "ai_video" | "ai_talking" | "ai_review"
}

// VideoConfig holds FFmpeg generation parameters.
type VideoConfig struct {
	InputImages    []string // file paths to product images
	OverlayText    string   // text to overlay on the video (Thai OK — burned by FFmpeg)
	ScenePrompt    string   // English scene description for AI image generation (FLUX)
	AudioPath      string   // optional background music / voice path
	TTSText        string   // ai_talking: text to synthesize to speech when AudioPath is empty
	TTSVoice       string   // edge-tts voice (default th-TH-PremwadeeNeural)
	OutputPath     string   // destination MP4 path
	DurationSec    int      // target duration in seconds (15–60)
	FPS            int      // frames per second (default 1 for slideshow, 30 for animated)
	AnimationStyle string   // "" / "ken_burns" / "zoom_out" / "slide" / "static" / "ai_cartoon" / "ai_video" / "ai_talking"
}
