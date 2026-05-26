package posts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
)

// scheduleRequest builds a JSON body with a valid RFC 3339 scheduled_at.
func scheduleRequest(t *testing.T, postID, userID uuid.UUID, at time.Time) *http.Request {
	t.Helper()
	body := `{"scheduled_at":"` + at.UTC().Format(time.RFC3339) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/posts/"+postID.String()+"/schedule", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Inject authenticated user and chi route param.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", postID.String())
	ctx := middleware.InjectUserID(req.Context(), userID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestScheduleHandler_Returns200_OnSuccess(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)
	handler := posts.NewHandler(svc)

	postID := uuid.New()
	userID := uuid.New()
	at := time.Now().Add(2 * time.Hour)

	repo.On("Schedule", mock.Anything, postID, userID, mock.AnythingOfType("models.SchedulePostRequest")).Return(nil)

	rr := httptest.NewRecorder()
	handler.Schedule(rr, scheduleRequest(t, postID, userID, at))

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "scheduled", resp["status"])
	repo.AssertExpectations(t)
}

func TestScheduleHandler_Returns404_WhenPostNotFound(t *testing.T) {
	repo := new(mockPostRepo)
	svc := posts.NewService(repo)
	handler := posts.NewHandler(svc)

	postID := uuid.New()
	userID := uuid.New()
	at := time.Now().Add(1 * time.Hour)

	repo.On("Schedule", mock.Anything, postID, userID, mock.AnythingOfType("models.SchedulePostRequest")).Return(pgx.ErrNoRows)

	rr := httptest.NewRecorder()
	handler.Schedule(rr, scheduleRequest(t, postID, userID, at))

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp models.ErrorResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "post not found", resp.Error)
	repo.AssertExpectations(t)
}
