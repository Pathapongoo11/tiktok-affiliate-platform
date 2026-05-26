package posts

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// PostRepository defines the persistence interface used by Service.
type PostRepository interface {
	Create(ctx context.Context, p *models.Post) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Post, error)
	GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Post, error)
	List(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]*models.Post, int, error)
	Update(ctx context.Context, p *models.Post) error
	Schedule(ctx context.Context, id, userID uuid.UUID, req models.SchedulePostRequest) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	GetProductByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
}

type Service struct {
	repo PostRepository
}

func NewService(repo PostRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, req models.CreatePostRequest) (*models.Post, error) {
	p := &models.Post{
		ID:              uuid.New(),
		UserID:          userID,
		TikTokAccountID: req.TikTokAccountID,
		ProductID:       req.ProductID,
		Title:           req.Title,
		Caption:         req.Caption,
		Hashtags:        req.Hashtags,
		VideoPath:       req.VideoPath,
		Status:          "draft",
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, status string, page, limit int) ([]*models.Post, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	return s.repo.List(ctx, userID, status, limit, offset)
}

func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, req models.UpdatePostRequest) (*models.Post, error) {
	existing, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	existing.TikTokAccountID = req.TikTokAccountID
	existing.ProductID = req.ProductID
	existing.Title = req.Title
	existing.Caption = req.Caption
	existing.Hashtags = req.Hashtags
	existing.VideoPath = req.VideoPath

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *Service) Schedule(ctx context.Context, id, userID uuid.UUID, req models.SchedulePostRequest) (*models.Post, error) {
	if err := s.repo.Schedule(ctx, id, userID, req); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}

// GetProductByID returns a product by ID for use in caption generation.
// Returns nil, nil if not found.
func (s *Service) GetProductByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	return s.repo.GetProductByID(ctx, id)
}
