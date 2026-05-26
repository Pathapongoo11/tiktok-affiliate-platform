package videos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/middleware"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// Handler exposes the video engine over HTTP using the chi router.
type Handler struct {
	service    *Service
	uploadsDir string
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, uploadsDir string) *Handler {
	return &Handler{
		service:    service,
		uploadsDir: uploadsDir,
	}
}

// Routes returns a chi.Router with all video endpoints registered.
// All routes expect the caller's router group to have already applied
// the apimiddleware.JWT middleware.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/generate", h.createJob)  // POST /api/videos/generate
	r.Post("/upload", h.uploadImage)  // POST /api/videos/upload
	r.Get("/", h.listJobs)            // GET  /api/videos
	r.Get("/{jobId}", h.getJob)       // GET  /api/videos/:jobId
	return r
}

// POST /api/videos/generate
// Body: { "input_images": [...], "overlay_text": "...", "audio_path": "...", "post_id": "..." }
func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if len(req.InputImages) == 0 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "input_images must contain at least one path"})
		return
	}

	job, err := h.service.CreateVideoJob(r.Context(), userID, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create video job"})
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

// GET /api/videos/:jobId
func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid job id"})
		return
	}

	job, err := h.service.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "job not found"})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// GET /api/videos
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	jobs, err := h.service.ListJobsByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list video jobs"})
		return
	}

	if jobs == nil {
		jobs = []*models.VideoJob{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// POST /api/videos/upload
// Accepts multipart/form-data with field "image" (image/jpeg or image/png).
// Returns { "path": "...", "filename": "..." } — use path in CreateJobRequest.InputImages.
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	// Limit to 10 MB to guard against accidental large uploads
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "request too large or not multipart"})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "field 'image' is required"})
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "only image/jpeg and image/png are accepted"})
		return
	}

	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}

	imagesDir := filepath.Join(h.uploadsDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create uploads directory"})
		return
	}

	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	destPath := filepath.Join(imagesDir, filename)

	dest, err := os.Create(destPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save file"})
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to write file"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"path":     filepath.ToSlash(destPath),
		"filename": filename,
	})
}

// writeJSON serialises v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
