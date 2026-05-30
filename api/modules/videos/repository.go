package videos

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// Repository handles all DB operations for the video_jobs table.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateJob inserts a new video_job record and returns it with DB-generated fields.
func (r *Repository) CreateJob(ctx context.Context, job *models.VideoJob) (*models.VideoJob, error) {
	imagesJSON, err := json.Marshal(job.InputImages)
	if err != nil {
		return nil, fmt.Errorf("marshal input_images: %w", err)
	}

	query := `
		INSERT INTO video_jobs
			(id, post_id, user_id, status, input_images, overlay_text,
			 audio_path, duration_seconds, animation_style, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, post_id, user_id, status, input_images, overlay_text,
		          audio_path, output_path, duration_seconds, animation_style,
		          error_message, created_at, updated_at`

	row := r.db.QueryRow(ctx, query,
		job.ID,
		job.PostID,
		job.UserID,
		job.Status,
		imagesJSON,
		job.OverlayText,
		job.AudioPath,
		job.DurationSeconds,
		job.AnimationStyle,
	)

	return scanJob(row)
}

// GetJob retrieves a single video_job by its UUID.
func (r *Repository) GetJob(ctx context.Context, id uuid.UUID) (*models.VideoJob, error) {
	query := `
		SELECT id, post_id, user_id, status, input_images, overlay_text,
		       audio_path, output_path, duration_seconds, animation_style,
		       error_message, created_at, updated_at
		FROM video_jobs
		WHERE id = $1`

	row := r.db.QueryRow(ctx, query, id)
	return scanJob(row)
}

// UpdateJobStatus sets status, output_path and error_message for a job.
// Empty strings for outputPath / errMsg leave existing DB values unchanged.
func (r *Repository) UpdateJobStatus(ctx context.Context, id uuid.UUID, status, outputPath, errMsg string) error {
	query := `
		UPDATE video_jobs
		SET status        = $2,
		    output_path   = CASE WHEN $3 != '' THEN $3 ELSE output_path   END,
		    error_message = CASE WHEN $4 != '' THEN $4 ELSE error_message END,
		    updated_at    = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id, status, outputPath, errMsg)
	if err != nil {
		return fmt.Errorf("UpdateJobStatus: %w", err)
	}
	return nil
}

// ListJobsByUser returns all video jobs for userID ordered newest first.
func (r *Repository) ListJobsByUser(ctx context.Context, userID uuid.UUID) ([]*models.VideoJob, error) {
	query := `
		SELECT id, post_id, user_id, status, input_images, overlay_text,
		       audio_path, output_path, duration_seconds, animation_style,
		       error_message, created_at, updated_at
		FROM video_jobs
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ListJobsByUser: %w", err)
	}
	defer rows.Close()

	var jobs []*models.VideoJob
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// rowScanner is satisfied by both *pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanJob maps a single DB row into a models.VideoJob.
func scanJob(s rowScanner) (*models.VideoJob, error) {
	var job models.VideoJob
	var imagesJSON []byte
	var postID *uuid.UUID
	var audioPath, outputPath, errMsg *string
	var createdAt, updatedAt time.Time

	err := s.Scan(
		&job.ID,
		&postID,
		&job.UserID,
		&job.Status,
		&imagesJSON,
		&job.OverlayText,
		&audioPath,
		&outputPath,
		&job.DurationSeconds,
		&job.AnimationStyle,
		&errMsg,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanJob: %w", err)
	}

	job.PostID = postID
	job.CreatedAt = createdAt
	job.UpdatedAt = updatedAt

	if audioPath != nil {
		job.AudioPath = *audioPath
	}
	if outputPath != nil {
		job.OutputPath = *outputPath
	}
	if errMsg != nil {
		job.ErrorMessage = *errMsg
	}

	if err := json.Unmarshal(imagesJSON, &job.InputImages); err != nil {
		return nil, fmt.Errorf("unmarshal input_images: %w", err)
	}

	return &job, nil
}
