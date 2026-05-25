package products

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

func (r *Repository) Create(ctx context.Context, p *models.Product) error {
	imageURLsJSON, _ := json.Marshal(p.ImageURLs)
	query := `
		INSERT INTO products (id, user_id, name, description, price, commission_rate, category, shop_product_id, image_urls, score, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query,
		p.ID, p.UserID, p.Name, p.Description, p.Price, p.CommissionRate,
		p.Category, p.ShopProductID, imageURLsJSON, p.Score, p.IsActive,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	query := `
		SELECT id, user_id, name, description, price, commission_rate, category, shop_product_id, image_urls, score, is_active, created_at, updated_at
		FROM products WHERE id = $1 AND is_active = true
	`
	return r.scanProduct(r.pool.QueryRow(ctx, query, id))
}

func (r *Repository) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Product, error) {
	query := `
		SELECT id, user_id, name, description, price, commission_rate, category, shop_product_id, image_urls, score, is_active, created_at, updated_at
		FROM products WHERE id = $1 AND user_id = $2
	`
	return r.scanProduct(r.pool.QueryRow(ctx, query, id, userID))
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, category string, limit, offset int) ([]*models.Product, int, error) {
	args := []interface{}{userID}
	where := "user_id = $1 AND is_active = true"
	argIdx := 2

	if category != "" {
		where += " AND category = $" + fmt.Sprintf("%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM products WHERE " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := `
		SELECT id, user_id, name, description, price, commission_rate, category, shop_product_id, image_urls, score, is_active, created_at, updated_at
		FROM products WHERE ` + where + ` ORDER BY score DESC, created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p, err := r.scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, p *models.Product) error {
	imageURLsJSON, _ := json.Marshal(p.ImageURLs)
	query := `
		UPDATE products
		SET name=$1, description=$2, price=$3, commission_rate=$4, category=$5, shop_product_id=$6, image_urls=$7, score=$8, updated_at=NOW()
		WHERE id=$9 AND user_id=$10
	`
	_, err := r.pool.Exec(ctx, query,
		p.Name, p.Description, p.Price, p.CommissionRate,
		p.Category, p.ShopProductID, imageURLsJSON, p.Score, p.ID, p.UserID,
	)
	return err
}

func (r *Repository) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	query := `UPDATE products SET is_active=false, updated_at=NOW() WHERE id=$1 AND user_id=$2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func (r *Repository) scanProduct(row scannable) (*models.Product, error) {
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
