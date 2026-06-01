package videos

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// Service orchestrates video job creation and async FFmpeg / AI processing.
type Service struct {
	repo       *Repository
	uploadsDir string
	hfToken    string // Hugging Face token for AI styles; empty disables AI pipeline
	lipsyncURL string // SadTalker microservice URL; empty disables lip-sync
}

// NewService constructs a Service.
// uploadsDir is the root directory where generated MP4 files are saved (e.g. "uploads/").
// hfToken enables the FLUX AI pipeline; "" makes AI styles fall back to ken_burns.
// lipsyncURL enables the ai_talking style; "" makes it fall back to ai_cartoon.
func NewService(db *pgxpool.Pool, uploadsDir, hfToken, lipsyncURL string) *Service {
	return &Service{
		repo:       NewRepository(db),
		uploadsDir: uploadsDir,
		hfToken:    hfToken,
		lipsyncURL: lipsyncURL,
	}
}

// CreateVideoJob persists a new job record and kicks off async FFmpeg generation.
// Returns immediately with the pending job; callers poll GetJob for status.
func (s *Service) CreateVideoJob(ctx context.Context, userID uuid.UUID, req CreateJobRequest) (*models.VideoJob, error) {
	dur := req.DurationSec
	if dur == 0 {
		dur = 30
	}

	audioPath := req.AudioPath
	// ai_talking / ai_review with text but no audio → synthesize Thai speech up
	// front. edge-tts is fast (~1-2s); the MP3 is stored like any uploaded audio.
	if (req.AnimationStyle == StyleAITalking || req.AnimationStyle == StyleAIReview) &&
		audioPath == "" && req.TTSText != "" && s.lipsyncURL != "" {
		if p, err := s.synthesizeTTS(ctx, req.TTSText, req.TTSVoice); err != nil {
			log.Printf("[video] TTS synthesis failed (will fall back): %v", err)
		} else {
			audioPath = p
		}
	}

	job := &models.VideoJob{
		ID:              uuid.New(),
		UserID:          userID,
		PostID:          req.PostID,
		Status:          "pending",
		InputImages:     req.InputImages,
		OverlayText:     req.OverlayText,
		AudioPath:       audioPath,
		DurationSeconds: dur,
		AnimationStyle:  req.AnimationStyle,
		ScenePrompt:     req.ScenePrompt,
	}

	created, err := s.repo.CreateJob(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("CreateVideoJob: %w", err)
	}

	// Kick off background FFmpeg processing — does not block the HTTP response
	go s.processJob(context.Background(), created)

	return created, nil
}

// processJob runs in a goroutine. It invokes FFmpeg and updates the job record.
func (s *Service) processJob(ctx context.Context, job *models.VideoJob) {
	if err := s.repo.UpdateJobStatus(ctx, job.ID, "processing", "", ""); err != nil {
		return
	}

	filename := fmt.Sprintf("video_%s_%d.mp4", job.ID, time.Now().Unix())
	// outputFSPath is used by FFmpeg (server filesystem).
	// outputURLPath is stored in DB and returned to the browser (/uploads/... URL).
	outputFSPath := filepath.Join(s.uploadsDir, filename)
	outputURLPath := "/uploads/" + filename

	dur := job.DurationSeconds
	if dur == 0 {
		dur = 30
	}
	cfg := VideoConfig{
		InputImages:    job.InputImages,
		OverlayText:    job.OverlayText,
		AudioPath:      job.AudioPath,
		OutputPath:     outputFSPath,
		DurationSec:    dur,
		AnimationStyle: job.AnimationStyle,
		ScenePrompt:    job.ScenePrompt,
	}

	if err := s.render(ctx, cfg); err != nil {
		s.repo.UpdateJobStatus(ctx, job.ID, "failed", "", err.Error()) //nolint:errcheck
		return
	}

	// Store the public URL path so the browser can fetch it via the static file route.
	s.repo.UpdateJobStatus(ctx, job.ID, "done", outputURLPath, "") //nolint:errcheck
}

// render dispatches to the right pipeline based on the requested style, with
// graceful fallbacks so a missing dependency never hard-fails the job:
//
//	ai_talking → FLUX character + SadTalker lip-sync   (needs hfToken + lipsyncURL + audio)
//	ai_*       → FLUX image + Ken Burns                (needs hfToken)
//	others     → pure FFmpeg                           (always available)
func (s *Service) render(ctx context.Context, cfg VideoConfig) error {
	if !IsAIStyle(cfg.AnimationStyle) {
		return GenerateVideo(cfg)
	}

	// All AI styles need the FLUX token; without it fall back to FFmpeg ken_burns.
	if s.hfToken == "" {
		log.Printf("[video] AI style %q requested but HUGGINGFACE_TOKEN is unset — falling back to ken_burns", cfg.AnimationStyle)
		cfg.AnimationStyle = StyleKenBurns
		return GenerateVideo(cfg)
	}

	if cfg.AnimationStyle == StyleAITalking {
		return s.renderTalking(ctx, cfg)
	}
	if cfg.AnimationStyle == StyleAIReview {
		return s.renderReview(ctx, cfg)
	}
	return s.renderAICartoon(ctx, cfg)
}

// renderReview builds a realistic "person reviews the product" video (GOAL 4
// Phase 1): a FLUX-generated presenter lip-synced to the voice track, with the
// real product photo (background removed via rembg) composited picture-in-picture.
//
// Pipeline: FLUX character → SadTalker talking head → rembg product cut-out →
// FFmpeg overlay (bottom-right PiP). Degrades gracefully:
//   - no service / no audio → ai_cartoon (FLUX + Ken Burns)
//   - no real product photo → presenter video alone (just the talking head)
//   - rembg/overlay failure  → presenter video alone
func (s *Service) renderReview(ctx context.Context, cfg VideoConfig) error {
	if s.lipsyncURL == "" || cfg.AudioPath == "" {
		log.Printf("[video] ai_review unavailable (lipsyncURL set=%t, audio set=%t) — falling back to ai_cartoon",
			s.lipsyncURL != "", cfg.AudioPath != "")
		cfg.AnimationStyle = StyleAICartoon
		return GenerateAIVideo(ctx, cfg, s.hfToken)
	}

	// Without a real product photo there's nothing to composite — the talking
	// presenter alone is the review video.
	if !hasRealImage(cfg.InputImages) {
		log.Printf("[video] ai_review: no real product image — rendering presenter only")
		return s.renderTalking(ctx, cfg)
	}

	// Stage 1: FLUX presenter + SadTalker → base talking-head MP4 (temp file).
	presenterPath := cfg.OutputPath + ".presenter.mp4"
	talkCfg := cfg
	talkCfg.OutputPath = presenterPath
	if err := s.renderTalking(ctx, talkCfg); err != nil {
		return fmt.Errorf("review presenter: %w", err)
	}
	defer os.Remove(presenterPath)

	// Stage 2: cut out the product background (rembg). If it fails, ship the
	// presenter alone rather than hard-failing the whole job.
	cutoutPath := cfg.OutputPath + ".product.png"
	if err := RemoveBackground(ctx, s.lipsyncURL, cfg.InputImages[0], cutoutPath); err != nil {
		log.Printf("[video] ai_review: rembg failed (%v) — shipping presenter alone", err)
		return os.Rename(presenterPath, cfg.OutputPath)
	}
	defer os.Remove(cutoutPath)

	// Stage 3: overlay the product picture-in-picture (bottom-right, 30% width).
	if err := OverlayProduct(ctx, s.lipsyncURL, presenterPath, cutoutPath, cfg.OutputPath, "bottom_right", 0.3); err != nil {
		log.Printf("[video] ai_review: overlay failed (%v) — shipping presenter alone", err)
		return os.Rename(presenterPath, cfg.OutputPath)
	}
	return nil
}

// renderAICartoon produces the AI base image then animates it with Ken Burns.
//
// GOAL 1: when the user uploaded a real product photo (not the "ai-generated"
// placeholder) and the img2img service is available, stylize THAT photo so the
// real product is preserved. Otherwise fall back to FLUX text→image.
func (s *Service) renderAICartoon(ctx context.Context, cfg VideoConfig) error {
	basePath := cfg.OutputPath + ".base.png"

	if s.lipsyncURL != "" && hasRealImage(cfg.InputImages) {
		prompt := buildCartoonPrompt(firstNonEmpty(cfg.ScenePrompt, cfg.OverlayText), cfg.AnimationStyle)
		if err := StylizeImage(ctx, s.lipsyncURL, cfg.InputImages[0], prompt, basePath); err != nil {
			log.Printf("[video] img2img failed (%v) — falling back to FLUX text→image", err)
			if err := GenerateAICharacterImage(ctx, cfg, s.hfToken, basePath); err != nil {
				return fmt.Errorf("generate base image: %w", err)
			}
		}
	} else {
		if err := GenerateAICharacterImage(ctx, cfg, s.hfToken, basePath); err != nil {
			return fmt.Errorf("generate base image: %w", err)
		}
	}
	defer os.Remove(basePath)

	animCfg := cfg
	animCfg.InputImages = []string{basePath}
	animCfg.AnimationStyle = StyleKenBurns
	return GenerateVideo(animCfg)
}

// hasRealImage reports whether the slice contains a genuine uploaded image path
// (not empty and not the "ai-generated" placeholder the frontend sends for AI styles).
func hasRealImage(paths []string) bool {
	for _, p := range paths {
		if p != "" && p != "ai-generated" {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// renderTalking builds a talking-character video: FLUX generates the character
// image, then the SadTalker microservice lip-syncs it to the audio track.
// Falls back to ai_cartoon (Ken Burns) when the service or audio is unavailable.
func (s *Service) renderTalking(ctx context.Context, cfg VideoConfig) error {
	if s.lipsyncURL == "" || cfg.AudioPath == "" {
		log.Printf("[video] ai_talking unavailable (lipsyncURL set=%t, audio set=%t) — falling back to ai_cartoon",
			s.lipsyncURL != "", cfg.AudioPath != "")
		cfg.AnimationStyle = StyleAICartoon
		return GenerateAIVideo(ctx, cfg, s.hfToken)
	}

	// Stage 1: FLUX generates the character image.
	charPath := cfg.OutputPath + ".character.png"
	if err := GenerateAICharacterImage(ctx, cfg, s.hfToken, charPath); err != nil {
		return fmt.Errorf("generate character image: %w", err)
	}
	defer os.Remove(charPath)

	// Stage 2: SadTalker lip-syncs the character to the audio.
	talkCfg := cfg
	talkCfg.InputImages = []string{charPath}
	if err := GenerateTalkingVideo(ctx, talkCfg, s.lipsyncURL); err != nil {
		return fmt.Errorf("lipsync: %w", err)
	}
	return nil
}

// synthesizeTTS renders text to a Thai-voice MP3 via the lip-sync microservice
// and returns the saved file path (under uploadsDir/audio).
func (s *Service) synthesizeTTS(ctx context.Context, text, voice string) (string, error) {
	filename := fmt.Sprintf("tts_%s.mp3", uuid.New())
	fsPath := filepath.Join(s.uploadsDir, "audio", filename)
	if err := SynthesizeSpeech(ctx, s.lipsyncURL, text, voice, fsPath); err != nil {
		return "", err
	}
	return filepath.ToSlash(fsPath), nil
}

// GetJob returns a single job by ID.
func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*models.VideoJob, error) {
	return s.repo.GetJob(ctx, id)
}

// ListJobsByUser returns all jobs for a user, newest first.
func (s *Service) ListJobsByUser(ctx context.Context, userID uuid.UUID) ([]*models.VideoJob, error) {
	return s.repo.ListJobsByUser(ctx, userID)
}
