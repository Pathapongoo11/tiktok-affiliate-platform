package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

const cacheTTL = 5 * time.Minute

type Service struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

func NewService(pool *pgxpool.Pool, redisClient *redis.Client) *Service {
	return &Service{pool: pool, redis: redisClient}
}

func (s *Service) GetStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error) {
	cacheKey := fmt.Sprintf("dashboard:stats:%s", userID)

	if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
		var stats models.DashboardStats
		if json.Unmarshal(cached, &stats) == nil {
			return &stats, nil
		}
	}

	query := `
		SELECT
			COUNT(DISTINCT p.id) AS total_posts,
			COALESCE(SUM(a.views), 0) AS total_views,
			COALESCE(SUM(a.likes), 0) AS total_likes,
			COALESCE(SUM(a.revenue), 0) AS total_revenue,
			CASE
				WHEN COALESCE(SUM(a.views), 0) > 0
				THEN ROUND(CAST((COALESCE(SUM(a.likes), 0) + COALESCE(SUM(a.comments), 0) + COALESCE(SUM(a.shares), 0)) AS NUMERIC) /
				     NULLIF(COALESCE(SUM(a.views), 0), 0) * 100, 2)
				ELSE 0
			END AS avg_engagement
		FROM posts p
		LEFT JOIN analytics a ON a.post_id = p.id
		WHERE p.user_id = $1
	`

	stats := &models.DashboardStats{}
	err := s.pool.QueryRow(ctx, query, userID).Scan(
		&stats.TotalPosts,
		&stats.TotalViews,
		&stats.TotalLikes,
		&stats.TotalRevenue,
		&stats.AvgEngagement,
	)
	if err != nil {
		return nil, err
	}

	if b, err := json.Marshal(stats); err == nil {
		s.redis.Set(ctx, cacheKey, b, cacheTTL)
	}
	return stats, nil
}

func (s *Service) GetTopPosts(ctx context.Context, userID uuid.UUID) ([]*models.TopPost, error) {
	cacheKey := fmt.Sprintf("dashboard:top-posts:%s", userID)

	if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
		var result []*models.TopPost
		if json.Unmarshal(cached, &result) == nil {
			return result, nil
		}
	}

	query := `
		SELECT p.id, p.user_id, p.tiktok_account_id, p.product_id, p.title, p.caption, p.hashtags,
		       p.video_path, p.tiktok_video_id, p.status, p.scheduled_at, p.published_at, p.created_at, p.updated_at,
		       COALESCE(SUM(a.views), 0) AS total_views
		FROM posts p
		LEFT JOIN analytics a ON a.post_id = p.id
		WHERE p.user_id = $1
		GROUP BY p.id
		ORDER BY total_views DESC
		LIMIT 10
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.TopPost
	for rows.Next() {
		post := &models.Post{}
		var (
			views         int64
			tikTokVideoID *string // nullable — posts not yet published to TikTok have NULL
		)
		err := rows.Scan(
			&post.ID, &post.UserID, &post.TikTokAccountID, &post.ProductID,
			&post.Title, &post.Caption, &post.Hashtags, &post.VideoPath,
			&tikTokVideoID, &post.Status, &post.ScheduledAt, &post.PublishedAt,
			&post.CreatedAt, &post.UpdatedAt, &views,
		)
		if err != nil {
			return nil, err
		}
		if tikTokVideoID != nil {
			post.TikTokVideoID = *tikTokVideoID
		}
		result = append(result, &models.TopPost{Post: post, Views: views})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if b, err := json.Marshal(result); err == nil {
		s.redis.Set(ctx, cacheKey, b, cacheTTL)
	}
	return result, nil
}

func (s *Service) GetProductPerformance(ctx context.Context, userID uuid.UUID) ([]*models.ProductPerformance, error) {
	cacheKey := fmt.Sprintf("dashboard:product-performance:%s", userID)

	if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
		var result []*models.ProductPerformance
		if json.Unmarshal(cached, &result) == nil {
			return result, nil
		}
	}

	query := `
		SELECT pr.id, pr.user_id, pr.name, pr.description, pr.price, pr.commission_rate,
		       pr.category, pr.shop_product_id, pr.image_urls, pr.score, pr.is_active, pr.created_at, pr.updated_at,
		       COALESCE(SUM(a.basket_clicks), 0) AS total_basket_clicks,
		       COALESCE(SUM(a.orders), 0) AS total_orders,
		       COALESCE(SUM(a.revenue), 0) AS total_revenue
		FROM products pr
		JOIN posts p ON p.product_id = pr.id
		LEFT JOIN analytics a ON a.post_id = p.id
		WHERE pr.user_id = $1 AND pr.is_active = true
		GROUP BY pr.id
		ORDER BY total_basket_clicks DESC
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.ProductPerformance
	for rows.Next() {
		prod := &models.Product{}
		var perf models.ProductPerformance
		var imageURLsJSON []byte

		err := rows.Scan(
			&prod.ID, &prod.UserID, &prod.Name, &prod.Description, &prod.Price, &prod.CommissionRate,
			&prod.Category, &prod.ShopProductID, &imageURLsJSON, &prod.Score, &prod.IsActive,
			&prod.CreatedAt, &prod.UpdatedAt,
			&perf.BasketClicks, &perf.Orders, &perf.Revenue,
		)
		if err != nil {
			return nil, err
		}
		if len(imageURLsJSON) > 0 {
			_ = json.Unmarshal(imageURLsJSON, &prod.ImageURLs)
		}
		perf.Product = prod
		result = append(result, &perf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if b, err := json.Marshal(result); err == nil {
		s.redis.Set(ctx, cacheKey, b, cacheTTL)
	}
	return result, nil
}
