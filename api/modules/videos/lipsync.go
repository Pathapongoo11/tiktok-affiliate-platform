package videos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Lip-sync microservice client.
//
// The SadTalker microservice runs on the Windows host (with GPU) and is reached
// from the Dockerised Go API via host.docker.internal. It takes a still image +
// audio and returns a talking-head MP4. Because it is GPU-bound and lives outside
// the container, all access goes through this thin HTTP client.
//
// Contract (see services/lipsync/app.py):
//
//	POST /generate            multipart {image, audio}  → 202 {job_id, status}
//	GET  /jobs/{id}                                     → {status, output_path, error}
//	GET  /jobs/{id}/download                            → MP4 bytes
//	POST /tts                 form {text, voice}        → MP3 bytes
const (
	lipsyncPollInterval = 5 * time.Second
	lipsyncMaxWait      = 15 * time.Minute

	// DefaultTTSVoice is the edge-tts voice used when none is specified.
	DefaultTTSVoice = "th-TH-PremwadeeNeural"
)

// lipsyncClient talks to the SadTalker microservice.
type lipsyncClient struct {
	baseURL    string
	httpClient *http.Client
}

func newLipsyncClient(baseURL string) *lipsyncClient {
	return &lipsyncClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 2 * time.Minute},
	}
}

// SynthesizeSpeech calls the microservice /tts endpoint to turn text into speech
// (Thai by default) and writes the resulting MP3 to destPath. voice may be empty
// to use the default Thai voice.
func SynthesizeSpeech(ctx context.Context, baseURL, text, voice, destPath string) error {
	if baseURL == "" {
		return fmt.Errorf("lipsync service url not configured")
	}
	if voice == "" {
		voice = DefaultTTSVoice
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// Send JSON (UTF-8) — multipart form fields carry no charset and mangled
	// Thai text to '?' on the Python side.
	payload, err := json.Marshal(map[string]string{"text": text, "voice": voice})
	if err != nil {
		return fmt.Errorf("marshal tts request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tts", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tts returned %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create audio file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write audio file: %w", err)
	}
	return nil
}

// StylizeImage sends a product image + prompt to the microservice /img2img
// endpoint (SDXL img2img on the GPU) and writes the stylized PNG to destPath.
// This preserves the real product's shape while applying a cartoon/3D look.
func StylizeImage(ctx context.Context, baseURL, srcImagePath, prompt, destPath string) error {
	if baseURL == "" {
		return fmt.Errorf("lipsync service url not configured")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if err := addFilePart(w, "image", srcImagePath); err != nil {
		return err
	}
	_ = w.WriteField("prompt", prompt)
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/img2img", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("img2img returned %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create stylized file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write stylized file: %w", err)
	}
	return nil
}

// RemoveBackground sends an image to /rembg and writes the transparent cut-out
// PNG to destPath (GOAL 4: prepare the product/person for compositing).
func RemoveBackground(ctx context.Context, baseURL, srcImagePath, destPath string) error {
	if baseURL == "" {
		return fmt.Errorf("lipsync service url not configured")
	}
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if err := addFilePart(w, "image", srcImagePath); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart: %w", err)
	}
	return postMultipart(ctx, baseURL+"/rembg", w.FormDataContentType(), body, destPath, 2*time.Minute)
}

// OverlayProduct composites a product PNG onto a base video as picture-in-picture
// (GOAL 4: put the product in a corner of the talking-person video).
// corner is one of bottom_right/bottom_left/top_right/top_left.
func OverlayProduct(ctx context.Context, baseURL, baseVideoPath, overlayPNG, destPath, corner string, scale float64) error {
	if baseURL == "" {
		return fmt.Errorf("lipsync service url not configured")
	}
	if corner == "" {
		corner = "bottom_right"
	}
	if scale <= 0 {
		scale = 0.3
	}
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if err := addFilePart(w, "base", baseVideoPath); err != nil {
		return err
	}
	if err := addFilePart(w, "overlay", overlayPNG); err != nil {
		return err
	}
	_ = w.WriteField("corner", corner)
	_ = w.WriteField("scale", fmt.Sprintf("%.3f", scale))
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart: %w", err)
	}
	return postMultipart(ctx, baseURL+"/overlay", w.FormDataContentType(), body, destPath, 5*time.Minute)
}

// postMultipart POSTs a prepared multipart body and streams the response to destPath.
func postMultipart(ctx context.Context, url, contentType string, body *bytes.Buffer, destPath string, timeout time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, truncate(string(b), 200))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

// GenerateTalkingVideo turns the source image into a talking-head MP4 using the
// lip-sync microservice, then writes the result to cfg.OutputPath.
//
// Requires cfg.AudioPath (the voice track) and a configured service URL.
func GenerateTalkingVideo(ctx context.Context, cfg VideoConfig, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("lipsync service url not configured")
	}
	if len(cfg.InputImages) == 0 {
		return fmt.Errorf("no input images provided")
	}
	if cfg.AudioPath == "" {
		return fmt.Errorf("ai_talking requires an audio file (audio_path)")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	client := newLipsyncClient(baseURL)

	jobID, err := client.submit(ctx, cfg.InputImages[0], cfg.AudioPath)
	if err != nil {
		return fmt.Errorf("submit lipsync job: %w", err)
	}

	if err := client.waitUntilDone(ctx, jobID); err != nil {
		return fmt.Errorf("lipsync job %s: %w", jobID, err)
	}

	if err := client.download(ctx, jobID, cfg.OutputPath); err != nil {
		return fmt.Errorf("download lipsync result: %w", err)
	}
	return nil
}

// submit uploads the image + audio and returns the created job ID.
func (c *lipsyncClient) submit(ctx context.Context, imagePath, audioPath string) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	if err := addFilePart(w, "image", imagePath); err != nil {
		return "", err
	}
	if err := addFilePart(w, "audio", audioPath); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("service returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var out struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if out.JobID == "" {
		return "", fmt.Errorf("service returned empty job_id")
	}
	return out.JobID, nil
}

// waitUntilDone polls the job until it succeeds, fails, or the deadline passes.
func (c *lipsyncClient) waitUntilDone(ctx context.Context, jobID string) error {
	deadline := time.Now().Add(lipsyncMaxWait)
	for {
		status, errMsg, err := c.poll(ctx, jobID)
		if err != nil {
			return err
		}
		switch status {
		case "done":
			return nil
		case "failed":
			return fmt.Errorf("service reported failure: %s", errMsg)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s (last status: %s)", lipsyncMaxWait, status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lipsyncPollInterval):
		}
	}
}

// poll returns the current status and error message for a job.
func (c *lipsyncClient) poll(ctx context.Context, jobID string) (status, errMsg string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("poll returned %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode poll: %w", err)
	}
	return out.Status, out.Error, nil
}

// download fetches the finished MP4 and writes it to destPath.
func (c *lipsyncClient) download(ctx context.Context, jobID, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/jobs/"+jobID+"/download", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download returned %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

// addFilePart copies a local file into a multipart form field.
func addFilePart(w *multipart.Writer, field, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	part, err := w.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return fmt.Errorf("create form field %s: %w", field, err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("copy %s: %w", field, err)
	}
	return nil
}
