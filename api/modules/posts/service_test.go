package posts_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
)

// ---------------------------------------------------------------------------
// Mock Repository
// ---------------------------------------------------------------------------

type mockPostRepo struct {
	mock.Mock
}

func (m *mockPostRepo) Create(ctx context.Context, p *models.Post) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockPostRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Post), args.Error(1)
}

func (m *mockPostRepo) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Post, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Post), args.Error(1)
}

func (m *mockPostRepo) List(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]*models.Post, int, error) {
	args := m.Called(ctx, userID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Post), args.Int(1), args.Error(2)
}

func (m *mockPostRepo) Update(ctx context.Context, p *models.Post) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockPostRepo) Schedule(ctx context.Context, id, userID uuid.UUID, req models.SchedulePostRequest) error {
	args := m.Called(ctx, id, userID, req)
	return args.Error(0)
}

func (m *mockPostRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreatePost_DefaultsDraft(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	userID := uuid.New()
	req := models.CreatePostRequest{
		Title:   "My first post",
		Caption: "Check this out!",
	}

	repo.On("Create", mock.Anything, mock.MatchedBy(func(p *models.Post) bool {
		return p.Status == "draft" && p.UserID == userID
	})).Return(nil)

	post, err := svc.Create(context.Background(), userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, post)
	assert.Equal(t, "draft", post.Status)
	assert.Equal(t, "My first post", post.Title)
	assert.Equal(t, userID, post.UserID)
	repo.AssertExpectations(t)
}

func TestCreatePost_GeneratesNewID(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	userID := uuid.New()
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.Post")).Return(nil)

	post1, _ := svc.Create(context.Background(), userID, models.CreatePostRequest{Title: "A"})
	post2, _ := svc.Create(context.Background(), userID, models.CreatePostRequest{Title: "B"})

	assert.NotEqual(t, post1.ID, post2.ID, "each post should get a unique ID")
}

func TestSchedulePost_SetsScheduledAt(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	postID := uuid.New()
	userID := uuid.New()
	scheduledAt := time.Now().Add(2 * time.Hour)

	req := models.SchedulePostRequest{ScheduledAt: scheduledAt}
	scheduledPost := &models.Post{
		ID:          postID,
		UserID:      userID,
		Status:      "scheduled",
		ScheduledAt: &scheduledAt,
	}

	repo.On("Schedule", mock.Anything, postID, userID, req).Return(nil)
	repo.On("GetByID", mock.Anything, postID).Return(scheduledPost, nil)

	post, err := svc.Schedule(context.Background(), postID, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, post)
	assert.Equal(t, "scheduled", post.Status)
	assert.NotNil(t, post.ScheduledAt)
	repo.AssertExpectations(t)
}

func TestSchedulePost_PostNotFound(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	postID := uuid.New()
	userID := uuid.New()
	req := models.SchedulePostRequest{ScheduledAt: time.Now().Add(1 * time.Hour)}

	repo.On("Schedule", mock.Anything, postID, userID, req).Return(pgx.ErrNoRows)

	post, err := svc.Schedule(context.Background(), postID, userID, req)

	assert.Nil(t, post)
	assert.NoError(t, err, "pgx.ErrNoRows should be converted to nil post, nil error")
	repo.AssertExpectations(t)
}

func TestGetPost_IncludesProduct(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	postID := uuid.New()
	prodID := uuid.New()
	product := &models.Product{ID: prodID, Name: "Cool Gadget", Price: 299}
	post := &models.Post{
		ID:        postID,
		Title:     "Post with product",
		ProductID: &prodID,
		Product:   product,
	}
	repo.On("GetByID", mock.Anything, postID).Return(post, nil)

	result, err := svc.GetByID(context.Background(), postID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Product)
	assert.Equal(t, "Cool Gadget", result.Product.Name)
	repo.AssertExpectations(t)
}

func TestListPosts_DefaultPagination(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	userID := uuid.New()
	list := []*models.Post{
		{ID: uuid.New(), Title: "Post 1"},
		{ID: uuid.New(), Title: "Post 2"},
	}
	// page=1 -> offset=0, limit=20
	repo.On("List", mock.Anything, userID, "", 20, 0).Return(list, 2, nil)

	result, total, err := svc.List(context.Background(), userID, "", 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestListPosts_InvalidPageDefaults(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	userID := uuid.New()
	repo.On("List", mock.Anything, userID, "", 20, 0).Return([]*models.Post{}, 0, nil)

	// page=0 should be corrected to 1, limit=0 corrected to 20
	_, _, err := svc.List(context.Background(), userID, "", 0, 0)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUpdatePost_Success(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	postID := uuid.New()
	userID := uuid.New()
	existing := &models.Post{
		ID:     postID,
		UserID: userID,
		Title:  "Old Title",
	}
	repo.On("GetByIDForUser", mock.Anything, postID, userID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Post")).Return(nil)

	req := models.UpdatePostRequest{
		Title:   "New Title",
		Caption: "Updated caption",
	}

	result, err := svc.Update(context.Background(), postID, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Title", result.Title)
	repo.AssertExpectations(t)
}

func TestDeletePost_Success(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)

	postID := uuid.New()
	userID := uuid.New()
	repo.On("Delete", mock.Anything, postID, userID).Return(nil)

	err := svc.Delete(context.Background(), postID, userID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
