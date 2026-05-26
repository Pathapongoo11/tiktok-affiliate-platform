package products

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// ServiceInterface is implemented by *Service and can be mocked in tests.
type ServiceInterface interface {
	Create(ctx context.Context, userID uuid.UUID, req models.CreateProductRequest) (*models.ProductWithFlags, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ProductWithFlags, error)
	List(ctx context.Context, userID uuid.UUID, category string, page, limit int) ([]*models.ProductWithFlags, int, error)
	Update(ctx context.Context, id, userID uuid.UUID, req models.UpdateProductRequest) (*models.ProductWithFlags, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// NewHandlerFromInterface creates a Handler that accepts any ServiceInterface,
// enabling handler tests to inject a mock.
func NewHandlerFromInterface(svc ServiceInterface) *Handler {
	return &Handler{service: svc}
}

// BuildFlags exposes buildFlags for tests.
func BuildFlags(p *models.Product) models.ProductFlags {
	return buildFlags(p)
}

// CalculateScore exposes calculateScore for tests.
func CalculateScore(price, commissionRate, manualScore float64) float64 {
	return calculateScore(price, commissionRate, manualScore)
}
