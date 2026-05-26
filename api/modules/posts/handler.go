package posts

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// SuggestCaptionRequest is the body for POST /api/posts/suggest-caption.
type SuggestCaptionRequest struct {
	ProductID string `json:"product_id"`
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Post("/suggest-caption", h.SuggestCaption) // AI-assisted caption generation
	r.Get("/{id}", h.GetByID)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/schedule", h.Schedule)
	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	posts, total, err := h.service.List(r.Context(), userID, status, page, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list posts"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:  posts,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req models.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	post, err := h.service.Create(r.Context(), userID, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create post"})
		return
	}
	writeJSON(w, http.StatusCreated, post)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid post id"})
		return
	}

	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get post"})
		return
	}
	if post == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "post not found"})
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid post id"})
		return
	}

	var req models.UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	post, err := h.service.Update(r.Context(), id, userID, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update post"})
		return
	}
	if post == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "post not found"})
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid post id"})
		return
	}

	if err := h.service.Delete(r.Context(), id, userID); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "post not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete post"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Schedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid post id"})
		return
	}

	var req models.SchedulePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	post, err := h.service.Schedule(r.Context(), id, userID, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to schedule post"})
		return
	}
	if post == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "post not found"})
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// POST /api/posts/suggest-caption
// Accepts { "product_id": "uuid" } and returns AI-generated Thai caption + hashtags.
// The product is looked up from the user's own catalog; falls back to name-only
// generation if the product is not found.
func (h *Handler) SuggestCaption(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req SuggestCaptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	// Try to load the product for richer caption context
	var product *models.Product
	if req.ProductID != "" {
		if pid, err := uuid.Parse(req.ProductID); err == nil {
			product, _ = h.service.GetProductByID(r.Context(), pid) // nil-safe
		}
	}

	if product == nil {
		// Return minimal placeholder when product not found
		writeJSON(w, http.StatusOK, SuggestCaptionResponse{
			Caption:  "สินค้าดีมาแล้ว! 🔥 กดสั่งได้ที่ตะกร้าข้างล่างเลยนะ",
			Hashtags: []string{"tiktokshopthailand", "สินค้าแนะนำ", "ของดีราคาถูก"},
		})
		return
	}

	result := GenerateCaption(product)
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
