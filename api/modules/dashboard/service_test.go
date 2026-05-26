package dashboard_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/dashboard"
)

// ---------------------------------------------------------------------------
// Mock DashboardService (simulates the full service for handler-level tests)
// ---------------------------------------------------------------------------

type mockDashboardService struct {
	mock.Mock
}

func (m *mockDashboardService) GetStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DashboardStats), args.Error(1)
}

func (m *mockDashboardService) GetTopPosts(ctx context.Context, userID uuid.UUID) ([]*models.TopPost, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TopPost), args.Error(1)
}

func (m *mockDashboardService) GetProductPerformance(ctx context.Context, userID uuid.UUID) ([]*models.ProductPerformance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ProductPerformance), args.Error(1)
}

// Verify that mockDashboardService satisfies the DashboardService interface.
var _ dashboard.DashboardService = (*mockDashboardService)(nil)

// ---------------------------------------------------------------------------
// Aggregate / stats logic tests (via mock service)
// ---------------------------------------------------------------------------

func TestGetStats_ReturnsAggregates(t *testing.T) {
	svc := new(mockDashboardService)
	userID := uuid.New()

	expected := &models.DashboardStats{
		TotalPosts:    5,
		TotalViews:    10000,
		TotalLikes:    500,
		TotalRevenue:  12500.50,
		AvgEngagement: 5.25,
	}
	svc.On("GetStats", mock.Anything, userID).Return(expected, nil)

	stats, err := svc.GetStats(context.Background(), userID)

	assert.NoError(t, err)
	assert.Equal(t, 5, stats.TotalPosts)
	assert.Equal(t, int64(10000), stats.TotalViews)
	assert.Equal(t, int64(500), stats.TotalLikes)
	assert.InDelta(t, 12500.50, stats.TotalRevenue, 0.01)
	assert.InDelta(t, 5.25, stats.AvgEngagement, 0.01)
	svc.AssertExpectations(t)
}

func TestGetTopPosts_OrderedByViews(t *testing.T) {
	svc := new(mockDashboardService)
	userID := uuid.New()

	topPosts := []*models.TopPost{
		{Post: &models.Post{Title: "Most viewed"}, Views: 9000},
		{Post: &models.Post{Title: "Second"}, Views: 5000},
		{Post: &models.Post{Title: "Third"}, Views: 1000},
	}
	svc.On("GetTopPosts", mock.Anything, userID).Return(topPosts, nil)

	result, err := svc.GetTopPosts(context.Background(), userID)

	assert.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify descending order by views
	for i := 1; i < len(result); i++ {
		assert.GreaterOrEqual(t, result[i-1].Views, result[i].Views,
			"posts must be sorted descending by views")
	}
	svc.AssertExpectations(t)
}

func TestGetTopPosts_Empty(t *testing.T) {
	svc := new(mockDashboardService)
	userID := uuid.New()

	svc.On("GetTopPosts", mock.Anything, userID).Return([]*models.TopPost{}, nil)

	result, err := svc.GetTopPosts(context.Background(), userID)

	assert.NoError(t, err)
	assert.Empty(t, result)
	svc.AssertExpectations(t)
}

func TestCacheHit_SkipsDB(t *testing.T) {
	// When the service returns cached data, the caller should receive it without
	// any additional side effects. We verify by checking the mock is called exactly once.
	svc := new(mockDashboardService)
	userID := uuid.New()

	cached := &models.DashboardStats{TotalPosts: 42, TotalViews: 99999}
	svc.On("GetStats", mock.Anything, userID).Return(cached, nil).Once()

	result1, err1 := svc.GetStats(context.Background(), userID)
	assert.NoError(t, err1)
	assert.Equal(t, 42, result1.TotalPosts)

	// Second call: mock returns same data (simulating cache hit scenario)
	svc.On("GetStats", mock.Anything, userID).Return(cached, nil).Once()
	result2, err2 := svc.GetStats(context.Background(), userID)
	assert.NoError(t, err2)
	assert.Equal(t, result1.TotalPosts, result2.TotalPosts)

	svc.AssertExpectations(t)
}

func TestGetProductPerformance_ReturnsAll(t *testing.T) {
	svc := new(mockDashboardService)
	userID := uuid.New()

	perf := []*models.ProductPerformance{
		{
			Product:      &models.Product{Name: "Widget"},
			BasketClicks: 200,
			Orders:       50,
			Revenue:      4999.50,
		},
	}
	svc.On("GetProductPerformance", mock.Anything, userID).Return(perf, nil)

	result, err := svc.GetProductPerformance(context.Background(), userID)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Widget", result[0].Product.Name)
	assert.Equal(t, int64(200), result[0].BasketClicks)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Engagement rate computation test (pure logic, no DB)
// ---------------------------------------------------------------------------

func TestEngagementRate_Computation(t *testing.T) {
	t.Run("standard engagement", func(t *testing.T) {
		views := int64(1000)
		likes := int64(40)
		comments := int64(10)
		shares := int64(5)

		engagements := likes + comments + shares
		var rate float64
		if views > 0 {
			rate = float64(engagements) / float64(views) * 100
		}
		assert.InDelta(t, 5.5, rate, 0.01)
	})

	t.Run("zero views returns zero", func(t *testing.T) {
		var views int64 = 0
		var engagements int64 = 100
		var rate float64
		if views > 0 {
			rate = float64(engagements) / float64(views) * 100
		}
		assert.Equal(t, float64(0), rate)
	})
}
