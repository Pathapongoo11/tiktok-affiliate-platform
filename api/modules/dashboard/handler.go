package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/stats", h.Stats)
	r.Get("/top-posts", h.TopPosts)
	r.Get("/product-performance", h.ProductPerformance)
	return r
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	stats, err := h.service.GetStats(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get stats"})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) TopPosts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	posts, err := h.service.GetTopPosts(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get top posts"})
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

func (h *Handler) ProductPerformance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	perf, err := h.service.GetProductPerformance(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get product performance"})
		return
	}
	writeJSON(w, http.StatusOK, perf)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
