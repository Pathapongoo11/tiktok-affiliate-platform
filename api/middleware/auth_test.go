package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
)

const testSecret = "test_jwt_secret"

func makeToken(userID uuid.UUID, secret string, exp time.Time) string {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     exp.Unix(),
		"iat":     time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))
	return signed
}

func nextHandler(t *testing.T, expectUserID uuid.UUID) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := middleware.GetUserID(r.Context())
		assert.True(t, ok, "expected userID in context")
		assert.Equal(t, expectUserID, uid)
		w.WriteHeader(http.StatusOK)
	})
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	userID := uuid.New()
	token := makeToken(userID, testSecret, time.Now().Add(1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(nextHandler(t, userID))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusUnauthorized, rw.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer this.is.not.a.valid.token")
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusUnauthorized, rw.Code)
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	// Token expired 1 hour ago
	token := makeToken(userID, testSecret, time.Now().Add(-1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusUnauthorized, rw.Code)
}

func TestJWTMiddleware_WrongSecret(t *testing.T) {
	userID := uuid.New()
	token := makeToken(userID, "wrong_secret", time.Now().Add(1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusUnauthorized, rw.Code)
}

func TestJWTMiddleware_MalformedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer tokenvalue")
	rw := httptest.NewRecorder()

	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusUnauthorized, rw.Code)
}

func TestGetUserID_WhenPresent(t *testing.T) {
	userID := uuid.New()
	token := makeToken(userID, testSecret, time.Now().Add(1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()

	var capturedUID uuid.UUID
	handler := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := middleware.GetUserID(r.Context())
		assert.True(t, ok)
		capturedUID = uid
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rw, req)

	assert.Equal(t, userID, capturedUID)
}
