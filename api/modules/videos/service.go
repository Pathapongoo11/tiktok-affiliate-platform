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
	job := &models.VideoJob{
		ID:              uuid.New(),
		UserID:          userID,
		PostID:          req.PostID,
		Status:          "pending",
		InputImages:     req.InputImages,
		OverlayText:     req.OverlayText,
		AudioPath:       req.AudioPath,
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
	return GenerateAIVideo(ctx, cfg, s.hfToken)
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

// GetJob returns a single job by ID.
func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*models.VideoJob, error) {
	return s.repo.GetJob(ctx, id)
}

// ListJobsByUser returns all jobs for a user, newest first.
func (s *Service) ListJobsByUser(ctx context.Context, userID uuid.UUID) ([]*models.VideoJob, error) {
	return s.repo.ListJobsByUser(ctx, userID)
}
