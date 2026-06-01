package videos_test

import (
	"context"
	"encoding/json"
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
// SynthesizeSpeech
// ---------------------------------------------------------------------------

func TestSynthesizeSpeech_NoURL_ReturnsError(t *testing.T) {
	err := videos.SynthesizeSpeech(context.Background(), "", "hi", "", filepath.Join(t.TempDir(), "a.mp3"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url not configured")
}

func TestSynthesizeSpeech_HappyPath_WritesMP3(t *testing.T) {
	out := filepath.Join(t.TempDir(), "speech.mp3")

	var gotText, gotVoice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tts", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body struct {
			Text  string `json:"text"`
			Voice string `json:"voice"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotText = body.Text
		gotVoice = body.Voice
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("FAKE-MP3"))
	}))
	defer srv.Close()

	err := videos.SynthesizeSpeech(context.Background(), srv.URL, "สวัสดี", "", out)
	require.NoError(t, err)

	assert.Equal(t, "สวัสดี", gotText)
	assert.Equal(t, videos.DefaultTTSVoice, gotVoice, "empty voice should default to Thai voice")

	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "FAKE-MP3", string(got))
}

func TestSynthesizeSpeech_ServiceError_Propagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := videos.SynthesizeSpeech(context.Background(), srv.URL, "hi", "th-TH-NiwatNeural", filepath.Join(t.TempDir(), "a.mp3"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ---------------------------------------------------------------------------
// StylizeImage (img2img)
// ---------------------------------------------------------------------------

func TestStylizeImage_NoURL_ReturnsError(t *testing.T) {
	err := videos.StylizeImage(context.Background(), "", "img.png", "make it cartoon", filepath.Join(t.TempDir(), "out.png"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url not configured")
}

func TestStylizeImage_HappyPath_WritesPNG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	out := filepath.Join(dir, "out.png")
	require.NoError(t, os.WriteFile(src, []byte("fake-png"), 0o644))

	var gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/img2img", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotPrompt = r.FormValue("prompt")
		_, _, ferr := r.FormFile("image")
		assert.NoError(t, ferr)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("STYLIZED-PNG"))
	}))
	defer srv.Close()

	err := videos.StylizeImage(context.Background(), srv.URL, src, "cartoon product", out)
	require.NoError(t, err)
	assert.Equal(t, "cartoon product", gotPrompt)

	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "STYLIZED-PNG", string(got))
}

func TestStylizeImage_ServiceError_Propagates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o644))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := videos.StylizeImage(context.Background(), srv.URL, src, "p", filepath.Join(dir, "out.png"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// ---------------------------------------------------------------------------
// RemoveBackground / OverlayProduct (compositing)
// ---------------------------------------------------------------------------

func TestRemoveBackground_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "p.jpg")
	out := filepath.Join(dir, "cut.png")
	require.NoError(t, os.WriteFile(src, []byte("fake"), 0o644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rembg", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		_, _, ferr := r.FormFile("image")
		assert.NoError(t, ferr)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("CUTOUT-PNG"))
	}))
	defer srv.Close()

	require.NoError(t, videos.RemoveBackground(context.Background(), srv.URL, src, out))
	got, _ := os.ReadFile(out)
	assert.Equal(t, "CUTOUT-PNG", string(got))
}

func TestRemoveBackground_NoURL(t *testing.T) {
	err := videos.RemoveBackground(context.Background(), "", "p.png", filepath.Join(t.TempDir(), "o.png"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url not configured")
}

func TestOverlayProduct_HappyPath(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.mp4")
	ov := filepath.Join(dir, "ov.png")
	out := filepath.Join(dir, "out.mp4")
	require.NoError(t, os.WriteFile(base, []byte("vid"), 0o644))
	require.NoError(t, os.WriteFile(ov, []byte("png"), 0o644))

	var gotCorner, gotScale string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/overlay", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotCorner = r.FormValue("corner")
		gotScale = r.FormValue("scale")
		_, _, e1 := r.FormFile("base")
		_, _, e2 := r.FormFile("overlay")
		assert.NoError(t, e1)
		assert.NoError(t, e2)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("COMPOSED-MP4"))
	}))
	defer srv.Close()

	require.NoError(t, videos.OverlayProduct(context.Background(), srv.URL, base, ov, out, "top_left", 0.25))
	assert.Equal(t, "top_left", gotCorner)
	assert.Equal(t, "0.250", gotScale)
	got, _ := os.ReadFile(out)
	assert.Equal(t, "COMPOSED-MP4", string(got))
}

func TestOverlayProduct_DefaultsCornerAndScale(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.mp4")
	ov := filepath.Join(dir, "ov.png")
	require.NoError(t, os.WriteFile(base, []byte("v"), 0o644))
	require.NoError(t, os.WriteFile(ov, []byte("p"), 0o644))

	var gotCorner, gotScale string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		gotCorner = r.FormValue("corner")
		gotScale = r.FormValue("scale")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	require.NoError(t, videos.OverlayProduct(context.Background(), srv.URL, base, ov, filepath.Join(dir, "o.mp4"), "", 0))
	assert.Equal(t, "bottom_right", gotCorner)
	assert.Equal(t, "0.300", gotScale)
}

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
