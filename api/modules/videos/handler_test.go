package videos_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// buildMultipartRequest creates a multipart/form-data request with a single
// file field carrying the given field name, content type, and bytes.
func buildMultipartRequest(t *testing.T, fieldName, contentType string, data []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="%s"; filename="test"`, fieldName)},
		"Content-Type":        {contentType},
	})
	require.NoError(t, err)

	_, err = io.Copy(part, bytes.NewReader(data))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// newTestHandler constructs a Handler with nil service (safe for upload tests
// that do not call service methods) and a temp uploads directory.
func newTestHandler(t *testing.T) *videos.Handler {
	t.Helper()
	uploadsDir := t.TempDir()
	// Service is nil — upload handler does not call any service methods.
	return videos.NewHandler(nil, uploadsDir)
}

// newTestHandlerWithDir same as newTestHandler but also returns the uploads dir.
func newTestHandlerWithDir(t *testing.T) (*videos.Handler, string) {
	t.Helper()
	uploadsDir := t.TempDir()
	return videos.NewHandler(nil, uploadsDir), uploadsDir
}

// ---------------------------------------------------------------------------
// Upload handler — image field (field name: "image")
// ---------------------------------------------------------------------------

func TestUpload_ImageJPEG_Success(t *testing.T) {
	h := newTestHandler(t)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	req := buildMultipartRequest(t, "image", "image/jpeg", jpegData)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["path"])
	assert.True(t, strings.HasSuffix(resp["filename"], ".jpg"),
		"JPEG upload should produce .jpg, got %s", resp["filename"])
}

func TestUpload_ImagePNG_Success(t *testing.T) {
	h := newTestHandler(t)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}

	req := buildMultipartRequest(t, "image", "image/png", pngData)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, strings.HasSuffix(resp["filename"], ".png"))
}

func TestUpload_ImageWebP_Success(t *testing.T) {
	h := newTestHandler(t)
	webpData := []byte("RIFF\x00\x00\x00\x00WEBP")

	req := buildMultipartRequest(t, "image", "image/webp", webpData)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, strings.HasSuffix(resp["filename"], ".webp"))
}

func TestUpload_ImageWrongContentType_Rejected(t *testing.T) {
	h := newTestHandler(t)
	req := buildMultipartRequest(t, "image", "application/octet-stream", []byte("garbage"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// Upload handler — audio field (new: field name "audio")
// ---------------------------------------------------------------------------

func TestUpload_AudioMPEG_Success(t *testing.T) {
	h := newTestHandler(t)
	mp3Data := []byte("ID3\x03\x00\x00\x00\x00\x00\x00")

	req := buildMultipartRequest(t, "audio", "audio/mpeg", mp3Data)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["path"])
	assert.True(t, strings.HasSuffix(resp["filename"], ".mp3"),
		"audio/mpeg upload should produce .mp3, got %s", resp["filename"])
}

func TestUpload_AudioWAV_Success(t *testing.T) {
	h := newTestHandler(t)
	wavData := []byte("RIFF\x00\x00\x00\x00WAVEfmt ")

	req := buildMultipartRequest(t, "audio", "audio/wav", wavData)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, strings.HasSuffix(resp["filename"], ".wav"))
}

func TestUpload_AudioUnknownContentType_FallsBackToMP3(t *testing.T) {
	h := newTestHandler(t)
	req := buildMultipartRequest(t, "audio", "audio/unknown-codec", []byte("audio data"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, strings.HasSuffix(resp["filename"], ".mp3"),
		"unknown audio content-type should fall back to .mp3, got %s", resp["filename"])
}

// ---------------------------------------------------------------------------
// Upload handler — wrong field name ("file" is the old buggy name)
// ---------------------------------------------------------------------------

func TestUpload_OldFieldName_File_ReturnsBadRequest(t *testing.T) {
	h := newTestHandler(t)
	// Regression: the old frontend sent field "file" — this must now be rejected.
	req := buildMultipartRequest(t, "file", "image/jpeg", []byte{0xFF, 0xD8})
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"old 'file' field name should be rejected; frontend must use 'image' or 'audio'")
	body := rr.Body.String()
	assert.Contains(t, body, "image", "error should mention expected field names")
}

// ---------------------------------------------------------------------------
// Upload handler — files are persisted to correct subdirectory
// ---------------------------------------------------------------------------

func TestUpload_ImageFile_SavedInImagesSubdir(t *testing.T) {
	h, uploadsDir := newTestHandlerWithDir(t)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47}

	req := buildMultipartRequest(t, "image", "image/png", pngData)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	// File must exist on disk
	_, err := os.Stat(resp["path"])
	assert.NoError(t, err, "image file should exist at %s", resp["path"])

	// Must be under the "images" subdirectory
	rel, err := filepath.Rel(uploadsDir, resp["path"])
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(filepath.ToSlash(rel), "images/"),
		"image should be in images/ subdir, got: %s", rel)
}

func TestUpload_AudioFile_SavedInAudioSubdir(t *testing.T) {
	h, uploadsDir := newTestHandlerWithDir(t)
	mp3Data := []byte("ID3\x03\x00")

	req := buildMultipartRequest(t, "audio", "audio/mpeg", mp3Data)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	// File must exist on disk
	_, err := os.Stat(resp["path"])
	assert.NoError(t, err, "audio file should exist at %s", resp["path"])

	// Must be under the "audio" subdirectory
	rel, err := filepath.Rel(uploadsDir, resp["path"])
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(filepath.ToSlash(rel), "audio/"),
		"audio should be in audio/ subdir, got: %s", rel)
}

// ---------------------------------------------------------------------------
// Unique UUID filenames
// ---------------------------------------------------------------------------

func TestUpload_MultipleImages_ProduceUniqueFilenames(t *testing.T) {
	h := newTestHandler(t)
	seen := make(map[string]bool)

	for i := 0; i < 5; i++ {
		data := []byte{0xFF, 0xD8, 0xFF, byte(i)}
		req := buildMultipartRequest(t, "image", "image/jpeg", data)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
		var resp map[string]string
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

		assert.False(t, seen[resp["filename"]], "duplicate filename: %s", resp["filename"])
		seen[resp["filename"]] = true
	}
}

func TestUpload_Filename_IsUUID(t *testing.T) {
	h := newTestHandler(t)
	req := buildMultipartRequest(t, "image", "image/jpeg", []byte{0xFF, 0xD8})
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	base := filepath.Base(resp["path"])
	nameOnly := strings.TrimSuffix(base, filepath.Ext(base))
	_, err := uuid.Parse(nameOnly)
	assert.NoError(t, err, "filename base should be a valid UUID, got: %s", nameOnly)
}

// ---------------------------------------------------------------------------
// CreateJobRequest — JSON tags must be snake_case (frontend sends snake_case)
// ---------------------------------------------------------------------------

func TestCreateJobRequest_SnakeCaseKeys_Deserialise(t *testing.T) {
	// Verifies the fix: frontend now sends snake_case keys for generate payload.
	payload := `{
		"input_images":    ["/uploads/images/a.jpg", "/uploads/images/b.jpg"],
		"overlay_text":    "Test Product ฿299",
		"duration_seconds": 30,
		"audio_path":      "/uploads/audio/bg.mp3"
	}`

	var req videos.CreateJobRequest
	require.NoError(t, json.Unmarshal([]byte(payload), &req))
	assert.Len(t, req.InputImages, 2, "input_images should deserialise")
	assert.Equal(t, "Test Product ฿299", req.OverlayText, "overlay_text should deserialise")
	assert.Equal(t, 30, req.DurationSec, "duration_seconds should deserialise")
	assert.Equal(t, "/uploads/audio/bg.mp3", req.AudioPath, "audio_path should deserialise")
}

func TestCreateJobRequest_CamelCaseKeys_AreIgnored(t *testing.T) {
	// Regression guard: old camelCase payload must NOT be silently accepted.
	payload := `{
		"imagePaths":      ["/uploads/images/a.jpg"],
		"overlayText":     "should be ignored",
		"durationSeconds": 15,
		"audioPath":       "/uploads/audio/bg.mp3"
	}`

	var req videos.CreateJobRequest
	require.NoError(t, json.Unmarshal([]byte(payload), &req))
	assert.Empty(t, req.InputImages, "camelCase imagePaths must not be parsed (use input_images)")
	assert.Empty(t, req.OverlayText, "camelCase overlayText must not be parsed (use overlay_text)")
	assert.Zero(t, req.DurationSec, "camelCase durationSeconds must not be parsed (use duration_seconds)")
	assert.Empty(t, req.AudioPath, "camelCase audioPath must not be parsed (use audio_path)")
}
