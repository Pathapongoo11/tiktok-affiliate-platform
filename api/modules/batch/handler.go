package batch

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// Handler exposes the content-factory batch planner over HTTP.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/plan", h.Plan) // POST /api/batch/plan
	return r
}

// POST /api/batch/plan
// Body: PlanRequest. Returns a generated content calendar (and, if create_posts
// is true, persists each item as a scheduled draft post).
func (h *Handler) Plan(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	plan, err := h.service.Plan(r.Context(), userID, req)
	if err != nil {
		log.Printf("[batch] plan error user=%s: %v", userID, err)
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
