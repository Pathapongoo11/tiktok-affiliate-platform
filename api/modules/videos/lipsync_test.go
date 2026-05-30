package videos_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

// ---------------------------------------------------------------------------
// Guards (no network)
// ---------------------------------------------------------------------------

func TestGenerateTalkingVideo_NoURL_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages: []string{"img.png"},
		AudioPath:   "audio.wav",
		OutputPath:  filepath.Join(t.TempDir(), "out.mp4"),
	}
	err := videos.GenerateTalkingVideo(context.Background(), cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url not configured")
}

func TestGenerateTalkingVideo_NoImages_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages: nil,
		AudioPath:   "audio.wav",
		OutputPath:  filepath.Join(t.TempDir(), "out.mp4"),
	}
	err := videos.GenerateTalkingVideo(context.Background(), cfg, "http://localhost:9999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input images")
}

func TestGenerateTalkingVideo_NoAudio_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages: []string{"img.png"},
		AudioPath:   "",
		OutputPath:  filepath.Join(t.TempDir(), "out.mp4"),
	}
	err := videos.GenerateTalkingVideo(context.Background(), cfg, "http://localhost:9999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an audio file")
}

// ---------------------------------------------------------------------------
// Full happy path against a mock SadTalker service
// ---------------------------------------------------------------------------

func TestGenerateTalkingVideo_HappyPath(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "char.png")
	audioPath := filepath.Join(dir, "voice.wav")
	outPath := filepath.Join(dir, "talking.mp4")
	require.NoError(t, os.WriteFile(imgPath, []byte("fake-png"), 0o644))
	require.NoError(t, os.WriteFile(audioPath, []byte("fake-wav"), 0o644))

	var pollCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/generate":
			// Verify multipart fields arrived.
			require.NoError(t, r.ParseMultipartForm(1<<20))
			_, _, ferr := r.FormFile("image")
			assert.NoError(t, ferr)
			_, _, aerr := r.FormFile("audio")
			assert.NoError(t, aerr)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"job123","status":"pending"}`))

		case r.Method == http.MethodGet && r.URL.Path == "/jobs/job123":
			// First poll: processing. Second+: done.
			if atomic.AddInt32(&pollCount, 1) >= 2 {
				_, _ = w.Write([]byte(`{"status":"done","output_path":"/x/talking.mp4"}`))
			} else {
				_, _ = w.Write([]byte(`{"status":"processing"}`))
			}

		case r.Method == http.MethodGet && r.URL.Path == "/jobs/job123/download":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("FAKE-MP4-BYTES"))

		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := videos.VideoConfig{
		InputImages: []string{imgPath},
		AudioPath:   audioPath,
		OutputPath:  outPath,
	}

	err := videos.GenerateTalkingVideo(context.Background(), cfg, srv.URL)
	require.NoError(t, err)

	got, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "FAKE-MP4-BYTES", string(got))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&pollCount), int32(2), "should have polled until done")
}

func TestGenerateTalkingVideo_ServiceReportsFailure(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "char.png")
	audioPath := filepath.Join(dir, "voice.wav")
	require.NoError(t, os.WriteFile(imgPath, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(audioPath, []byte("y"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/generate") {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"j","status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"failed","error":"cuda out of memory"}`))
	}))
	defer srv.Close()

	cfg := videos.VideoConfig{
		InputImages: []string{imgPath},
		AudioPath:   audioPath,
		OutputPath:  filepath.Join(dir, "out.mp4"),
	}

	err := videos.GenerateTalkingVideo(context.Background(), cfg, srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cuda out of memory")
}
