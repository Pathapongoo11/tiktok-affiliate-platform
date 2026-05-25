package products

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

type Service struct {
	repo  *Repository
	cache *ristretto.Cache[string, any]
}

func NewService(repo *Repository, cache *ristretto.Cache[string, any]) *Service {
	return &Service{repo: repo, cache: cache}
}

// calculateScore computes the product score:
// Score = (commission_rate * 0.3) + (price_tier_score * 0.25) + (manual_score * 0.45)
// For new products, manual_score is the raw score passed in (default 5.0 out of 10).
func calculateScore(price, commissionRate, manualScore float64) float64 {
	priceTierScore := 5.0
	if price >= 200 && price <= 3000 {
		priceTierScore = 10.0
	}
	return (commissionRate * 0.3) + (priceTierScore * 0.25) + (manualScore * 0.45)
}

func buildFlags(p *models.Product) models.ProductFlags {
	var warnings []string
	if p.CommissionRate > 40 {
		warnings = append(warnings, "High commission may indicate low conversion")
	}
	if p.Price > 3000 {
		warnings = append(warnings, "Above impulse-buy range for Thai market")
	}
	return models.ProductFlags{Warnings: warnings}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, req models.CreateProductRequest) (*models.ProductWithFlags, error) {
	manualScore := 5.0
	p := &models.Product{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		Price:          req.Price,
		CommissionRate: req.CommissionRate,
		Category:       req.Category,
		ShopProductID:  req.ShopProductID,
		ImageURLs:      req.ImageURLs,
		IsActive:       true,
	}
	p.Score = calculateScore(p.Price, p.CommissionRate, manualScore)

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	s.invalidateCache(userID)
	return &models.ProductWithFlags{Product: p, Flags: buildFlags(p)}, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.ProductWithFlags, error) {
	cacheKey := "product:" + id.String()
	if v, ok := s.cache.Get(cacheKey); ok {
		if p, ok := v.(*models.Product); ok {
			return &models.ProductWithFlags{Product: p, Flags: buildFlags(p)}, nil
		}
	}

	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	s.cache.SetWithTTL(cacheKey, p, 1, 5*time.Minute)
	return &models.ProductWithFlags{Product: p, Flags: buildFlags(p)}, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, category string, page, limit int) ([]*models.ProductWithFlags, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	prods, total, err := s.repo.List(ctx, userID, category, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*models.ProductWithFlags, len(prods))
	for i, p := range prods {
		result[i] = &models.ProductWithFlags{Product: p, Flags: buildFlags(p)}
	}
	return result, total, nil
}

func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, req models.UpdateProductRequest) (*models.ProductWithFlags, error) {
	existing, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.Price = req.Price
	existing.CommissionRate = req.CommissionRate
	existing.Category = req.Category
	existing.ShopProductID = req.ShopProductID
	existing.ImageURLs = req.ImageURLs
	existing.Score = calculateScore(existing.Price, existing.CommissionRate, 5.0)

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	s.cache.Del("product:" + id.String())
	s.invalidateCache(userID)
	return &models.ProductWithFlags{Product: existing, Flags: buildFlags(existing)}, nil
}

func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	err := s.repo.SoftDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	s.cache.Del("product:" + id.String())
	s.invalidateCache(userID)
	return nil
}

func (s *Service) invalidateCache(userID uuid.UUID) {
	s.cache.Del("products:user:" + userID.String())
}
