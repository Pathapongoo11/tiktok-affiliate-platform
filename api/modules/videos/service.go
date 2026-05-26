package videos

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// Service orchestrates video job creation and async FFmpeg processing.
type Service struct {
	repo       *Repository
	uploadsDir string
}

// NewService constructs a Service.
// uploadsDir is the root directory where generated MP4 files are saved (e.g. "uploads/").
func NewService(db *pgxpool.Pool, uploadsDir string) *Service {
	return &Service{
		repo:       NewRepository(db),
		uploadsDir: uploadsDir,
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

	outputPath := filepath.Join(
		s.uploadsDir,
		fmt.Sprintf("video_%s_%d.mp4", job.ID, time.Now().Unix()),
	)

	dur := job.DurationSeconds
	if dur == 0 {
		dur = 30
	}
	cfg := VideoConfig{
		InputImages: job.InputImages,
		OverlayText: job.OverlayText,
		AudioPath:   job.AudioPath,
		OutputPath:  outputPath,
		DurationSec: dur,
		FPS:         1,
	}

	if err := GenerateVideo(cfg); err != nil {
		s.repo.UpdateJobStatus(ctx, job.ID, "failed", "", err.Error()) //nolint:errcheck
		return
	}

	s.repo.UpdateJobStatus(ctx, job.ID, "done", outputPath, "") //nolint:errcheck
}

// GetJob returns a single job by ID.
func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*models.VideoJob, error) {
	return s.repo.GetJob(ctx, id)
}

// ListJobsByUser returns all jobs for a user, newest first.
func (s *Service) ListJobsByUser(ctx context.Context, userID uuid.UUID) ([]*models.VideoJob, error) {
	return s.repo.ListJobsByUser(ctx, userID)
}
