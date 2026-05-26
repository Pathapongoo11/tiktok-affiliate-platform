package videos

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker periodically polls for stalled video jobs (stuck in "processing" after
// a server restart) and marks them failed so they can be retried by the operator.
type Worker struct {
	repo     *Repository
	interval time.Duration
	done     chan struct{}
}

// NewWorker constructs a Worker that polls every interval.
// A sensible default interval is 30 * time.Second.
func NewWorker(db *pgxpool.Pool, interval time.Duration) *Worker {
	return &Worker{
		repo:     NewRepository(db),
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start launches the polling loop in a goroutine.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the worker to stop after the current tick finishes.
func (w *Worker) Stop() {
	close(w.done)
}

func (w *Worker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[video-worker] started, polling every %s", w.interval)
	for {
		select {
		case <-w.done:
			log.Println("[video-worker] stopped")
			return
		case <-ctx.Done():
			log.Println("[video-worker] context cancelled, stopping")
			return
		case <-ticker.C:
			w.recoverStalledJobs(ctx)
		}
	}
}

// recoverStalledJobs marks "processing" jobs that have not been updated for
// longer than stalledThreshold as "failed".
func (w *Worker) recoverStalledJobs(ctx context.Context) {
	const stalledThreshold = 10 * time.Minute

	rows, err := w.repo.db.Query(ctx, `
		SELECT id, post_id, user_id, status, input_images, overlay_text,
		       audio_path, output_path, duration_seconds, error_message,
		       created_at, updated_at
		FROM video_jobs
		WHERE status = 'processing'
		  AND updated_at < NOW() - INTERVAL '10 minutes'`,
	)
	if err != nil {
		log.Printf("[video-worker] query stalled jobs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			log.Printf("[video-worker] scan: %v", err)
			continue
		}
		log.Printf("[video-worker] recovering stalled job %s (stuck > %s)", job.ID, stalledThreshold)
		if err := w.repo.UpdateJobStatus(ctx, job.ID, "failed", "", "job stalled: recovered by worker"); err != nil {
			log.Printf("[video-worker] update job %s: %v", job.ID, err)
		}
	}
}
