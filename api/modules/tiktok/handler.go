package tiktok

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
)

type Handler struct {
	client *Client
	repo   *Repository
}

func NewHandler(client *Client, repo *Repository) *Handler {
	return &Handler{client: client, repo: repo}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/status", h.Status)
	r.Get("/auth-url", h.AuthURL)
	r.Get("/callback", h.OAuthCallback)
	r.Get("/mock-oauth-callback", h.MockOAuthCallback)
	r.Get("/accounts", h.ListAccounts)
	r.Delete("/accounts/{id}", h.DeleteAccount)
	r.Post("/analytics/sync", h.SyncAnalytics)
	return r
}

// GET /api/tiktok/status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mock_mode":       h.client.IsMockMode(),
		"credentials_set": !h.client.IsMockMode(),
	})
}

// GET /api/tiktok/auth-url
func (h *Handler) AuthURL(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	state := fmt.Sprintf("%s_%d", userID, time.Now().Unix())
	authURL := h.client.GetAuthURL(state)
	writeJSON(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

// GET /api/tiktok/callback?code=...&state=...
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}
	h.handleOAuthExchange(w, r, code, state)
}

// GET /api/tiktok/mock-oauth-callback?code=...&state=...
func (h *Handler) MockOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		code = "mock_code_12345"
	}
	if state == "" {
		state = "mock_state"
	}
	h.handleOAuthExchange(w, r, code, state)
}

func (h *Handler) handleOAuthExchange(w http.ResponseWriter, r *http.Request, code, state string) {
	ctx := r.Context()

	// Extract userID from state (format: "<uuid>_<timestamp>")
	// In mock mode we may not have a real JWT; fall back to context.
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		// Try to parse from state
		var uid uuid.UUID
		if len(state) >= 36 {
			if parsed, err := uuid.Parse(state[:36]); err == nil {
				uid = parsed
				ok = true
			}
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "could not determine user id")
			return
		}
		userID = uid
	}

	token, err := h.client.ExchangeCode(ctx, code)
	if err != nil {
		log.Printf("[TikTok] ExchangeCode error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to exchange code")
		return
	}

	account := &TikTokAccountRecord{
		ID:             uuid.New(),
		UserID:         userID,
		TikTokUserID:   token.OpenID,
		DisplayName:    token.OpenID, // Will be updated when user info is fetched later
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		TokenExpiresAt: time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
		IsActive:       true,
	}

	if err := h.repo.CreateTikTokAccount(ctx, account); err != nil {
		log.Printf("[TikTok] CreateTikTokAccount error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":        "TikTok account connected successfully",
		"tiktok_user_id": token.OpenID,
		"account_id":     account.ID,
		"mock_mode":      h.client.IsMockMode(),
	})
}

// GET /api/tiktok/accounts
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accounts, err := h.repo.ListTikTokAccountsByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[TikTok] ListAccounts error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list accounts")
		return
	}

	// Return safe (no tokens) representation
	type safeAccount struct {
		ID             uuid.UUID `json:"id"`
		TikTokUserID   string    `json:"tiktok_user_id"`
		DisplayName    string    `json:"display_name"`
		IsActive       bool      `json:"is_active"`
		TokenExpiresAt time.Time `json:"token_expires_at"`
		CreatedAt      time.Time `json:"created_at"`
	}

	safe := make([]safeAccount, 0, len(accounts))
	for _, a := range accounts {
		safe = append(safe, safeAccount{
			ID:             a.ID,
			TikTokUserID:   a.TikTokUserID,
			DisplayName:    a.DisplayName,
			IsActive:       a.IsActive,
			TokenExpiresAt: a.TokenExpiresAt,
			CreatedAt:      a.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts": safe,
		"total":    len(safe),
	})
}

// DELETE /api/tiktok/accounts/{id}
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	// Verify ownership
	account, err := h.repo.GetTikTokAccountByID(r.Context(), accountID)
	if err != nil || account == nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if account.UserID != userID {
		writeError(w, http.StatusForbidden, "not your account")
		return
	}

	if err := h.repo.DeleteTikTokAccount(r.Context(), accountID); err != nil {
		log.Printf("[TikTok] DeleteAccount error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "account disconnected"})
}

// POST /api/tiktok/analytics/sync
func (h *Handler) SyncAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()

	// Get all published posts for this user
	posts, err := h.repo.GetPublishedPostsForUser(ctx, userID)
	if err != nil {
		log.Printf("[TikTok] SyncAnalytics fetch posts error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch posts")
		return
	}

	synced := 0
	errors := 0
	for _, post := range posts {
		if post.TikTokAccountID == nil {
			continue
		}
		account, err := h.repo.GetTikTokAccountByID(ctx, *post.TikTokAccountID)
		if err != nil || account == nil {
			errors++
			continue
		}

		metrics, err := h.client.GetVideoMetrics(ctx, account.AccessToken, post.TikTokVideoID)
		if err != nil {
			log.Printf("[TikTok] GetVideoMetrics error for post %s: %v", post.ID, err)
			errors++
			continue
		}

		record := &AnalyticsRecord{
			ID:         uuid.New(),
			PostID:     post.ID,
			UserID:     userID,
			Views:      metrics.Views,
			Likes:      metrics.Likes,
			Comments:   metrics.Comments,
			Shares:     metrics.Shares,
			RecordedAt: metrics.RecordedAt,
		}
		if err := h.repo.SaveAnalytics(ctx, record); err != nil {
			log.Printf("[TikTok] SaveAnalytics error for post %s: %v", post.ID, err)
			errors++
			continue
		}
		synced++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"synced": synced,
		"errors": errors,
		"total":  len(posts),
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
