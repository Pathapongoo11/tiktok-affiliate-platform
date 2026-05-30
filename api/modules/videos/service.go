package videos

import (
	"context"
	"fmt"
	"log"
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
}

// NewService constructs a Service.
// uploadsDir is the root directory where generated MP4 files are saved (e.g. "uploads/").
// hfToken enables the AI video pipeline; pass "" to disable it (AI styles then
// fall back to the ken_burns FFmpeg animation).
func NewService(db *pgxpool.Pool, uploadsDir, hfToken string) *Service {
	return &Service{
		repo:       NewRepository(db),
		uploadsDir: uploadsDir,
		hfToken:    hfToken,
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
	}

	if err := s.render(ctx, cfg); err != nil {
		s.repo.UpdateJobStatus(ctx, job.ID, "failed", "", err.Error()) //nolint:errcheck
		return
	}

	// Store the public URL path so the browser can fetch it via the static file route.
	s.repo.UpdateJobStatus(ctx, job.ID, "done", outputURLPath, "") //nolint:errcheck
}

// render dispatches to the AI pipeline or the FFmpeg pipeline based on the
// requested style. AI styles fall back to ken_burns when no HF token is set.
func (s *Service) render(ctx context.Context, cfg VideoConfig) error {
	if IsAIStyle(cfg.AnimationStyle) {
		if s.hfToken == "" {
			log.Printf("[video] AI style %q requested but HUGGINGFACE_TOKEN is unset — falling back to ken_burns", cfg.AnimationStyle)
			cfg.AnimationStyle = StyleKenBurns
			return GenerateVideo(cfg)
		}
		return GenerateAIVideo(ctx, cfg, s.hfToken)
	}
	return GenerateVideo(cfg)
}

// GetJob returns a single job by ID.
func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*models.VideoJob, error) {
	return s.repo.GetJob(ctx, id)
}

// ListJobsByUser returns all jobs for a user, newest first.
func (s *Service) ListJobsByUser(ctx context.Context, userID uuid.UUID) ([]*models.VideoJob, error) {
	return s.repo.ListJobsByUser(ctx, userID)
}
