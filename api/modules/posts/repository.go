package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, p *models.Post) error {
	query := `
		INSERT INTO posts (id, user_id, tiktok_account_id, product_id, title, caption, hashtags, video_path, status, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query,
		p.ID, p.UserID, p.TikTokAccountID, p.ProductID,
		p.Title, p.Caption, p.Hashtags, p.VideoPath, p.Status, p.ScheduledAt,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.tiktok_account_id, p.product_id, p.title, p.caption, p.hashtags,
		       p.video_path, p.tiktok_video_id, p.status, p.scheduled_at, p.published_at, p.created_at, p.updated_at,
		       pr.id, pr.user_id, pr.name, pr.description, pr.price, pr.commission_rate, pr.category,
		       pr.shop_product_id, pr.image_urls, pr.score, pr.is_active, pr.created_at, pr.updated_at
		FROM posts p
		LEFT JOIN products pr ON pr.id = p.product_id
		WHERE p.id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	return r.scanPostWithProduct(row)
}

func (r *Repository) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.tiktok_account_id, p.product_id, p.title, p.caption, p.hashtags,
		       p.video_path, p.tiktok_video_id, p.status, p.scheduled_at, p.published_at, p.created_at, p.updated_at,
		       pr.id, pr.user_id, pr.name, pr.description, pr.price, pr.commission_rate, pr.category,
		       pr.shop_product_id, pr.image_urls, pr.score, pr.is_active, pr.created_at, pr.updated_at
		FROM posts p
		LEFT JOIN products pr ON pr.id = p.product_id
		WHERE p.id = $1 AND p.user_id = $2
	`
	row := r.pool.QueryRow(ctx, query, id, userID)
	return r.scanPostWithProduct(row)
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]*models.Post, int, error) {
	args := []interface{}{userID}
	where := "p.user_id = $1"
	argIdx := 2

	if status != "" {
		where += fmt.Sprintf(" AND p.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM posts p WHERE " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := `
		SELECT p.id, p.user_id, p.tiktok_account_id, p.product_id, p.title, p.caption, p.hashtags,
		       p.video_path, p.tiktok_video_id, p.status, p.scheduled_at, p.published_at, p.created_at, p.updated_at,
		       pr.id, pr.user_id, pr.name, pr.description, pr.price, pr.commission_rate, pr.category,
		       pr.shop_product_id, pr.image_urls, pr.score, pr.is_active, pr.created_at, pr.updated_at
		FROM posts p
		LEFT JOIN products pr ON pr.id = p.product_id
		WHERE ` + where + fmt.Sprintf(" ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []*models.Post
	for rows.Next() {
		p, err := r.scanPostWithProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, p)
	}
	return result, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, p *models.Post) error {
	query := `
		UPDATE posts
		SET tiktok_account_id=$1, product_id=$2, title=$3, caption=$4, hashtags=$5, video_path=$6, updated_at=NOW()
		WHERE id=$7 AND user_id=$8
	`
	_, err := r.pool.Exec(ctx, query,
		p.TikTokAccountID, p.ProductID, p.Title, p.Caption, p.Hashtags, p.VideoPath, p.ID, p.UserID,
	)
	return err
}

func (r *Repository) Schedule(ctx context.Context, id, userID uuid.UUID, req models.SchedulePostRequest) error {
	query := `
		UPDATE posts SET status='scheduled', scheduled_at=$1, updated_at=NOW()
		WHERE id=$2 AND user_id=$3
	`
	tag, err := r.pool.Exec(ctx, query, req.ScheduledAt, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM posts WHERE id=$1 AND user_id=$2", id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetProductByID fetches a product by ID for caption generation.
// Returns nil, nil when not found.
func (r *Repository) GetProductByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	query := `
		SELECT id, user_id, name, description, price, commission_rate, category,
		       shop_product_id, image_urls, score, is_active, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	p := &models.Product{}
	var imageURLsJSON []byte
	err := row.Scan(
		&p.ID, &p.UserID, &p.Name, &p.Description, &p.Price, &p.CommissionRate,
		&p.Category, &p.ShopProductID, &imageURLsJSON, &p.Score, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(imageURLsJSON) > 0 {
		_ = json.Unmarshal(imageURLsJSON, &p.ImageURLs)
	}
	return p, nil
}

type postScanner interface {
	Scan(dest ...interface{}) error
}

func (r *Repository) scanPostWithProduct(row postScanner) (*models.Post, error) {
	p := &models.Post{}
	pr := &models.Product{}
	var (
		tikTokVideoID   *string // nullable in DB — posts without a TikTok video ID are NULL
		prID            *uuid.UUID
		prUserID        *uuid.UUID
		prName          *string
		prDesc          *string
		prPrice         *float64
		prCommission    *float64
		prCategory      *string
		prShopID        *string
		prImageURLsJSON *[]byte
		prScore         *float64
		prIsActive      *bool
		prCreatedAt     interface{}
		prUpdatedAt     interface{}
		imageURLsJSON   []byte
	)

	err := row.Scan(
		&p.ID, &p.UserID, &p.TikTokAccountID, &p.ProductID,
		&p.Title, &p.Caption, &p.Hashtags, &p.VideoPath,
		&tikTokVideoID, &p.Status, &p.ScheduledAt, &p.PublishedAt,
		&p.CreatedAt, &p.UpdatedAt,
		&prID, &prUserID, &prName, &prDesc, &prPrice, &prCommission,
		&prCategory, &prShopID, &prImageURLsJSON, &prScore, &prIsActive,
		&prCreatedAt, &prUpdatedAt,
	)
	if tikTokVideoID != nil {
		p.TikTokVideoID = *tikTokVideoID
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if prID != nil {
		pr.ID = *prID
		if prUserID != nil {
			pr.UserID = *prUserID
		}
		if prName != nil {
			pr.Name = *prName
		}
		if prDesc != nil {
			pr.Description = *prDesc
		}
		if prPrice != nil {
			pr.Price = *prPrice
		}
		if prCommission != nil {
			pr.CommissionRate = *prCommission
		}
		if prCategory != nil {
			pr.Category = *prCategory
		}
		if prShopID != nil {
			pr.ShopProductID = *prShopID
		}
		if prImageURLsJSON != nil && len(*prImageURLsJSON) > 0 {
			_ = json.Unmarshal(*prImageURLsJSON, &pr.ImageURLs)
		}
		if prScore != nil {
			pr.Score = *prScore
		}
		if prIsActive != nil {
			pr.IsActive = *prIsActive
		}
		p.Product = pr
	}

	_ = imageURLsJSON
	return p, nil
}
