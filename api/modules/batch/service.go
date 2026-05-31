package batch

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
)

const (
	defaultDaysAhead   = 7
	defaultPostsPerDay = 3
	maxDaysAhead       = 30
	maxPostsPerDay     = 4 // bounded by the number of best posting slots
)

// ProductGetter looks up a product the planner will build content for.
// Satisfied by the posts service (GetProductByID).
type ProductGetter interface {
	GetProductByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
}

// PostCreator persists a generated item as a post (optional step).
// Satisfied by the posts service (Create).
type PostCreator interface {
	Create(ctx context.Context, userID uuid.UUID, req models.CreatePostRequest) (*models.Post, error)
}

// Service builds content calendars from a product list.
type Service struct {
	products ProductGetter
	posts    PostCreator
	now      func() time.Time // injectable clock for tests
}

// NewService constructs a batch Service.
func NewService(products ProductGetter, postCreator PostCreator) *Service {
	return &Service{products: products, posts: postCreator, now: time.Now}
}

// Plan builds a content calendar. For each (day, slot) it round-robins through
// the requested products and generates a hook + script + caption + hashtags.
// When req.CreatePosts is true, each item is also persisted as a scheduled draft.
func (s *Service) Plan(ctx context.Context, userID uuid.UUID, req PlanRequest) (*PlanResponse, error) {
	if len(req.ProductIDs) == 0 {
		return nil, fmt.Errorf("at least one product_id is required")
	}
	days := clamp(req.DaysAhead, defaultDaysAhead, 1, maxDaysAhead)
	perDay := clamp(req.PostsPerDay, defaultPostsPerDay, 1, maxPostsPerDay)

	// Resolve products once; skip IDs that don't exist / aren't the user's.
	products := make([]*models.Product, 0, len(req.ProductIDs))
	for _, id := range req.ProductIDs {
		p, err := s.products.GetProductByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("look up product %s: %w", id, err)
		}
		if p != nil {
			products = append(products, p)
		}
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("none of the given product_ids were found")
	}

	resp := &PlanResponse{DaysAhead: days, PostsPerDay: perDay}
	base := startOfTomorrow(s.now())
	prodIdx := 0

	for day := 0; day < days; day++ {
		for slot := 0; slot < perDay; slot++ {
			p := products[prodIdx%len(products)]
			prodIdx++

			when := slotTime(base, day, slot)
			item := PlanItem{
				ProductID:   p.ID,
				ProductName: p.Name,
				ScheduledAt: when,
				Hook:        GenerateHook(p),
				Script:      GenerateScript(p),
			}
			cap := posts.GenerateCaption(p)
			item.Caption = cap.Caption
			item.Hashtags = cap.Hashtags

			if req.CreatePosts && s.posts != nil {
				pid := p.ID
				created, err := s.posts.Create(ctx, userID, models.CreatePostRequest{
					ProductID: &pid,
					Title:     item.Hook,
					Caption:   item.Caption,
					Hashtags:  item.Hashtags,
				})
				if err == nil && created != nil {
					item.PostID = &created.ID
					resp.PostsCreated++
				}
			}

			resp.Items = append(resp.Items, item)
		}
	}
	resp.TotalItems = len(resp.Items)
	return resp, nil
}

// ── helpers ──────────────────────────────────────────────────────────────

func clamp(v, def, min, max int) int {
	if v == 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// startOfTomorrow returns 00:00 of the day after now (calendar starts tomorrow).
func startOfTomorrow(now time.Time) time.Time {
	t := now.Add(24 * time.Hour)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// slotTime maps (day, slot) to a concrete timestamp using the best posting hours.
func slotTime(base time.Time, day, slot int) time.Time {
	ht := bestPostTimes[slot%len(bestPostTimes)]
	d := base.AddDate(0, 0, day)
	return time.Date(d.Year(), d.Month(), d.Day(), ht.Hour, ht.Min, 0, 0, d.Location())
}
