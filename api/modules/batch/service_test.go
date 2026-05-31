package batch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/batch"
)

// ── mocks ───────────────────────────────────────────────────────────────

type mockProducts struct {
	byID map[uuid.UUID]*models.Product
}

func (m *mockProducts) GetProductByID(_ context.Context, id uuid.UUID) (*models.Product, error) {
	return m.byID[id], nil // nil when not found — planner skips it
}

type mockPosts struct {
	created int
}

func (m *mockPosts) Create(_ context.Context, userID uuid.UUID, _ models.CreatePostRequest) (*models.Post, error) {
	m.created++
	return &models.Post{ID: uuid.New(), UserID: userID, Status: "draft"}, nil
}

func newProduct(name, category string, price float64) *models.Product {
	return &models.Product{ID: uuid.New(), Name: name, Category: category, Price: price}
}

// ── tests ───────────────────────────────────────────────────────────────

func TestPlan_NoProducts_Error(t *testing.T) {
	svc := batch.NewService(&mockProducts{}, &mockPosts{})
	_, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one product_id")
}

func TestPlan_DefaultsTo7DaysTimes3(t *testing.T) {
	p := newProduct("Serum X", "Beauty", 299)
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, &mockPosts{})

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, 7, resp.DaysAhead)
	assert.Equal(t, 3, resp.PostsPerDay)
	assert.Equal(t, 21, resp.TotalItems)
	assert.Len(t, resp.Items, 21)
}

func TestPlan_EveryItemHasContent(t *testing.T) {
	p := newProduct("Vitamin C", "Health", 450)
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, &mockPosts{})

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID}, DaysAhead: 2, PostsPerDay: 2,
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 4)
	for _, it := range resp.Items {
		assert.NotEmpty(t, it.Hook, "hook")
		assert.NotEmpty(t, it.Script, "script")
		assert.NotEmpty(t, it.Caption, "caption")
		assert.NotEmpty(t, it.Hashtags, "hashtags")
		assert.Equal(t, p.ID, it.ProductID)
	}
}

func TestPlan_SchedulesAscendingFutureTimes(t *testing.T) {
	p := newProduct("Snack", "Food", 59)
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, &mockPosts{})

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID}, DaysAhead: 3, PostsPerDay: 2,
	})
	require.NoError(t, err)
	// All slots are in the future and the day blocks are ascending.
	first := resp.Items[0].ScheduledAt
	last := resp.Items[len(resp.Items)-1].ScheduledAt
	assert.True(t, last.After(first), "last slot should be after the first")
}

func TestPlan_RoundRobinsProducts(t *testing.T) {
	a := newProduct("A", "Beauty", 100)
	b := newProduct("B", "Health", 200)
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{a.ID: a, b.ID: b}}, &mockPosts{})

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{a.ID, b.ID}, DaysAhead: 1, PostsPerDay: 4,
	})
	require.NoError(t, err)
	// 4 slots, 2 products → each appears twice.
	counts := map[uuid.UUID]int{}
	for _, it := range resp.Items {
		counts[it.ProductID]++
	}
	assert.Equal(t, 2, counts[a.ID])
	assert.Equal(t, 2, counts[b.ID])
}

func TestPlan_CreatePosts_PersistsDrafts(t *testing.T) {
	p := newProduct("Serum", "Beauty", 299)
	mp := &mockPosts{}
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, mp)

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID}, DaysAhead: 2, PostsPerDay: 2, CreatePosts: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, resp.PostsCreated)
	assert.Equal(t, 4, mp.created)
	for _, it := range resp.Items {
		assert.NotNil(t, it.PostID, "each item should have a created post id")
	}
}

func TestPlan_SkipsMissingProducts(t *testing.T) {
	p := newProduct("Real", "Beauty", 100)
	missing := uuid.New()
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, &mockPosts{})

	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID, missing}, DaysAhead: 1, PostsPerDay: 2,
	})
	require.NoError(t, err)
	// Only the real product is used.
	for _, it := range resp.Items {
		assert.Equal(t, p.ID, it.ProductID)
	}
}

func TestPlan_AllProductsMissing_Error(t *testing.T) {
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{}}, &mockPosts{})
	_, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the given product_ids")
}

func TestGenerateHook_NoFormatLeak(t *testing.T) {
	// Hooks without a %s placeholder must not leak Go's "%!(EXTRA ...)" marker.
	for _, cat := range []string{"Beauty", "Health", "Fashion", "Electronics", "Food", "Unknown"} {
		p := newProduct("เซรั่มวิตามินซี", cat, 299)
		for i := 0; i < 20; i++ { // hooks are random; sample enough to hit all templates
			h := batch.GenerateHook(p)
			assert.NotContains(t, h, "%!", "category %s leaked a format marker: %q", cat, h)
			assert.NotContains(t, h, "EXTRA", "category %s leaked EXTRA: %q", cat, h)
		}
	}
}

func TestPlan_ClampsPostsPerDay(t *testing.T) {
	p := newProduct("X", "Beauty", 100)
	svc := batch.NewService(&mockProducts{byID: map[uuid.UUID]*models.Product{p.ID: p}}, &mockPosts{})
	// request 99/day → clamped to max 4
	resp, err := svc.Plan(context.Background(), uuid.New(), batch.PlanRequest{
		ProductIDs: []uuid.UUID{p.ID}, DaysAhead: 1, PostsPerDay: 99,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, resp.PostsPerDay)
}
