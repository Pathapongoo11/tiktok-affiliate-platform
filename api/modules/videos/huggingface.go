package videos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Hugging Face Inference API endpoints and models.
//
// Runs on the free Hugging Face Inference API via the hf-inference provider.
// The legacy api-inference.huggingface.co host was deprecated in 2025 in favour
// of router.huggingface.co — see https://huggingface.co/docs/inference-providers
//
// NOTE: As of 2025 the free hf-inference provider no longer serves image→video
// or image→image models (e.g. SVD, cartoonizer return "Model not supported by
// provider hf-inference"). Only text→image models like FLUX.1-schnell remain
// available on the free tier. So the cartoon pipeline is:
//
//	product image + caption → text prompt → FLUX cartoon image → FFmpeg Ken Burns
//
// The first request to a cold model returns HTTP 503 with an estimated load
// time; we retry until the model is warm or the overall deadline elapses.
const (
	hfBaseURL = "https://router.huggingface.co/hf-inference/models/"

	// Text → image (cartoon/illustration). The only style available free.
	hfModelFlux = "black-forest-labs/FLUX.1-schnell"

	hfPollInterval = 8 * time.Second
	hfMaxWait      = 4 * time.Minute
)

// hfClient wraps the HTTP client and token for Hugging Face requests.
type hfClient struct {
	token      string
	httpClient *http.Client
}

func newHFClient(token string) *hfClient {
	return &hfClient{
		token:      token,
		httpClient: &http.Client{Timeout: 3 * time.Minute},
	}
}

// GenerateAIVideo produces an animated cartoon video using a two-stage pipeline:
//
//  1. FLUX.1-schnell generates a cartoon image from a text prompt derived from
//     the product (overlay text + style keywords).
//  2. The existing FFmpeg animated pipeline (Ken Burns zoom) turns that cartoon
//     image into a 9:16 MP4.
//
// Both StyleAIVideo and StyleAICartoon use this pipeline; the only difference is
// the prompt style. Requires a non-empty token.
func GenerateAIVideo(ctx context.Context, cfg VideoConfig, token string) error {
	if token == "" {
		return fmt.Errorf("huggingface token not configured")
	}
	if len(cfg.InputImages) == 0 {
		return fmt.Errorf("no input images provided")
	}

	// Stage 1: generate a cartoon image with FLUX.
	cartoonPath := cfg.OutputPath + ".cartoon.png"
	if err := GenerateAICharacterImage(ctx, cfg, token, cartoonPath); err != nil {
		return fmt.Errorf("flux generate step: %w", err)
	}
	defer os.Remove(cartoonPath)

	// Stage 2: animate the cartoon image with the FFmpeg Ken Burns pipeline.
	animCfg := cfg
	animCfg.InputImages = []string{cartoonPath}
	animCfg.AnimationStyle = StyleKenBurns // FFmpeg animated path
	if err := GenerateVideo(animCfg); err != nil {
		return fmt.Errorf("animate cartoon step: %w", err)
	}
	return nil
}

// GenerateAICharacterImage generates a single character/scene image with FLUX
// and writes it to destPath. Shared by the ai_cartoon/ai_video pipelines and the
// ai_talking lip-sync pipeline (which feeds the image to SadTalker).
func GenerateAICharacterImage(ctx context.Context, cfg VideoConfig, token, destPath string) error {
	if token == "" {
		return fmt.Errorf("huggingface token not configured")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// Prefer the explicit English scene prompt; fall back to overlay text.
	promptSubject := cfg.ScenePrompt
	if strings.TrimSpace(promptSubject) == "" {
		promptSubject = cfg.OverlayText
	}
	prompt := buildCartoonPrompt(promptSubject, cfg.AnimationStyle)

	imgBytes, err := newHFClient(token).textToImage(ctx, prompt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destPath, imgBytes, 0o644); err != nil {
		return fmt.Errorf("write character image: %w", err)
	}
	return nil
}

// buildCartoonPrompt turns a scene description into a FLUX text-to-image prompt.
//
// The subject should be an English scene/character description (e.g. "an angry
// germ monster wearing a crown"). Style keywords are tuned to match the punchy,
// dramatic look of Thai TikTok health/beauty ads — a single bold character,
// cinematic lighting, vivid colors, vertical 9:16 composition.
func buildCartoonPrompt(subject, style string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "a friendly product mascot character"
	}

	// ai_cartoon → dramatic 3D character-ad look (matches the reference style).
	styleKeywords := "highly detailed 3D Pixar-style character render, " +
		"dramatic cinematic lighting, bold vivid colors, expressive face, " +
		"eye-catching TikTok advertisement, dynamic close-up, vertical 9:16 composition, " +
		"depth of field, high detail, trending product ad"

	if style == StyleAIVideo {
		// ai_video → cleaner glossy product-render look.
		styleKeywords = "glossy 3D product render, studio lighting, vibrant colors, " +
			"floating product showcase, clean gradient background, dynamic angle, " +
			"vertical 9:16 composition, high detail, premium advertisement"
	}

	return fmt.Sprintf("%s, %s", subject, styleKeywords)
}

// textToImage calls a text-to-image model and returns the raw image bytes.
func (c *hfClient) textToImage(ctx context.Context, prompt string) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"inputs": prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	return c.infer(ctx, hfModelFlux, body, "application/json")
}

// infer POSTs a body to a Hugging Face model and returns the raw binary response
// (image bytes). It retries on HTTP 503 (model loading) until hfMaxWait elapses.
func (c *hfClient) infer(ctx context.Context, model string, body []byte, contentType string) ([]byte, error) {
	url := hfBaseURL + model
	deadline := time.Now().Add(hfMaxWait)

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Accept", "image/png")
		// Ask HF to block until the model is loaded when possible.
		req.Header.Set("X-Wait-For-Model", "true")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return respBody, nil

		case http.StatusServiceUnavailable:
			// Model is loading. Respect estimated_time if provided, else poll interval.
			wait := hfPollInterval
			var loadResp struct {
				EstimatedTime float64 `json:"estimated_time"`
			}
			if json.Unmarshal(respBody, &loadResp) == nil && loadResp.EstimatedTime > 0 {
				wait = time.Duration(loadResp.EstimatedTime) * time.Second
			}
			if time.Now().Add(wait).After(deadline) {
				return nil, fmt.Errorf("model %s still loading after %s", model, hfMaxWait)
			}
			log.Printf("[ai-video] model %s loading (attempt %d), waiting %s", model, attempt, wait)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}

		default:
			return nil, fmt.Errorf("huggingface %s returned %d: %s",
				model, resp.StatusCode, truncate(string(respBody), 300))
		}
	}
}

// truncate shortens a string for error messages.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
