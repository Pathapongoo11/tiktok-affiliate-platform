package tiktok

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// TikTokAccountRecord mirrors the tiktok_accounts DB table.
type TikTokAccountRecord struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	TikTokUserID   string
	DisplayName    string
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt time.Time
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AnalyticsRecord mirrors the analytics DB table.
type AnalyticsRecord struct {
	ID           uuid.UUID
	PostID       uuid.UUID
	UserID       uuid.UUID
	Views        int64
	Likes        int64
	Comments     int64
	Shares       int64
	BasketClicks int64
	Orders       int64
	Revenue      float64
	RecordedAt   time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreateTikTokAccount inserts a new TikTok account record.
func (r *Repository) CreateTikTokAccount(ctx context.Context, account *TikTokAccountRecord) error {
	query := `
		INSERT INTO tiktok_accounts (id, user_id, tiktok_user_id, display_name, access_token, refresh_token, token_expires_at, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (tiktok_user_id) DO UPDATE
		SET access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    is_active = true,
		    updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query,
		account.ID, account.UserID, account.TikTokUserID, account.DisplayName,
		account.AccessToken, account.RefreshToken, account.TokenExpiresAt, account.IsActive,
	)
	return err
}

// GetTikTokAccountByID retrieves a TikTok account by its UUID.
func (r *Repository) GetTikTokAccountByID(ctx context.Context, id uuid.UUID) (*TikTokAccountRecord, error) {
	query := `
		SELECT id, user_id, tiktok_user_id, display_name, access_token, refresh_token, token_expires_at, is_active, created_at, updated_at
		FROM tiktok_accounts
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	return scanTikTokAccount(row)
}

// ListTikTokAccountsByUserID returns all TikTok accounts for a given user.
func (r *Repository) ListTikTokAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*TikTokAccountRecord, error) {
	query := `
		SELECT id, user_id, tiktok_user_id, display_name, access_token, refresh_token, token_expires_at, is_active, created_at, updated_at
		FROM tiktok_accounts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*TikTokAccountRecord
	for rows.Next() {
		a, err := scanTikTokAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// UpdateAccountToken updates the OAuth tokens for a TikTok account.
func (r *Repository) UpdateAccountToken(ctx context.Context, id uuid.UUID, accessToken, refreshToken string, expiresAt time.Time) error {
	query := `
		UPDATE tiktok_accounts
		SET access_token = $1, refresh_token = $2, token_expires_at = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.pool.Exec(ctx, query, accessToken, refreshToken, expiresAt, id)
	return err
}

// DeleteTikTokAccount removes a TikTok account (soft-delete by setting is_active=false).
func (r *Repository) DeleteTikTokAccount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tiktok_accounts SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// GetDuePosts returns posts with status='scheduled' and scheduled_at <= NOW().
func (r *Repository) GetDuePosts(ctx context.Context) ([]*models.Post, error) {
	query := `
		SELECT id, user_id, tiktok_account_id, product_id, title, caption, hashtags,
		       video_path, tiktok_video_id, status, scheduled_at, published_at, created_at, updated_at
		FROM posts
		WHERE status = 'scheduled' AND scheduled_at <= NOW()
		ORDER BY scheduled_at ASC
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		p := &models.Post{}
		var tikTokVideoID *string // nullable — scheduled posts haven't been published yet
		err := rows.Scan(
			&p.ID, &p.UserID, &p.TikTokAccountID, &p.ProductID,
			&p.Title, &p.Caption, &p.Hashtags, &p.VideoPath,
			&tikTokVideoID, &p.Status, &p.ScheduledAt, &p.PublishedAt,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if tikTokVideoID != nil {
			p.TikTokVideoID = *tikTokVideoID
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// UpdatePostStatus updates the status and optional TikTok video ID / error message of a post.
func (r *Repository) UpdatePostStatus(ctx context.Context, postID uuid.UUID, status, tiktokVideoID, errMsg string) error {
	query := `
		UPDATE posts
		SET status = $1,
		    tiktok_video_id = CASE WHEN $2 != '' THEN $2 ELSE tiktok_video_id END,
		    updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.pool.Exec(ctx, query, status, tiktokVideoID, postID)
	return err
}

// SaveAnalytics inserts an analytics snapshot.
func (r *Repository) SaveAnalytics(ctx context.Context, a *AnalyticsRecord) error {
	query := `
		INSERT INTO analytics (id, post_id, user_id, views, likes, comments, shares, basket_clicks, orders, revenue, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.pool.Exec(ctx, query,
		a.ID, a.PostID, a.UserID, a.Views, a.Likes, a.Comments, a.Shares,
		a.BasketClicks, a.Orders, a.Revenue, a.RecordedAt,
	)
	return err
}

// GetPublishedPostsForUser returns published posts with a TikTok video ID for analytics sync.
func (r *Repository) GetPublishedPostsForUser(ctx context.Context, userID uuid.UUID) ([]*models.Post, error) {
	query := `
		SELECT id, user_id, tiktok_account_id, product_id, title, caption, hashtags,
		       video_path, tiktok_video_id, status, scheduled_at, published_at, created_at, updated_at
		FROM posts
		WHERE user_id = $1 AND status = 'published' AND tiktok_video_id != ''
		ORDER BY published_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		p := &models.Post{}
		var tikTokVideoID *string // nullable — scan via pointer to handle NULL safely
		err := rows.Scan(
			&p.ID, &p.UserID, &p.TikTokAccountID, &p.ProductID,
			&p.Title, &p.Caption, &p.Hashtags, &p.VideoPath,
			&tikTokVideoID, &p.Status, &p.ScheduledAt, &p.PublishedAt,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if tikTokVideoID != nil {
			p.TikTokVideoID = *tikTokVideoID
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

type accountScanner interface {
	Scan(dest ...any) error
}

func scanTikTokAccount(row accountScanner) (*TikTokAccountRecord, error) {
	a := &TikTokAccountRecord{}
	err := row.Scan(
		&a.ID, &a.UserID, &a.TikTokUserID, &a.DisplayName,
		&a.AccessToken, &a.RefreshToken, &a.TokenExpiresAt, &a.IsActive,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}
