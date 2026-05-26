package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// DashboardService is the interface satisfied by *Service.
// It allows handlers and tests to depend on the abstraction.
type DashboardService interface {
	GetStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error)
	GetTopPosts(ctx context.Context, userID uuid.UUID) ([]*models.TopPost, error)
	GetProductPerformance(ctx context.Context, userID uuid.UUID) ([]*models.ProductPerformance, error)
}
