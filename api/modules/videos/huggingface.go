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
	"time"
)

// Hugging Face Inference API endpoints and models.
//
// Both models run on the free Hugging Face Inference API via the hf-inference
// provider. The legacy api-inference.huggingface.co host was deprecated in 2025
// in favour of router.huggingface.co — see
// https://huggingface.co/docs/inference-providers/index
//
// The first request to a cold model returns HTTP 503 with an estimated load
// time; we retry until the model is warm or the overall deadline elapses.
const (
	hfBaseURL = "https://router.huggingface.co/hf-inference/models/"

	// Image → animated video (realistic motion). Outputs MP4 bytes directly.
	hfModelSVD = "stabilityai/stable-video-diffusion-img2vid-xt"

	// Image → cartoon image (img2img). Used as step 1 of the ai_cartoon pipeline.
	hfModelCartoonize = "instruction-tuning-sd/cartoonizer"

	hfPollInterval = 8 * time.Second
	hfMaxWait      = 5 * time.Minute
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

// GenerateAIVideo produces a video from the first input image using the
// Hugging Face Inference API. The pipeline depends on cfg.AnimationStyle:
//
//	StyleAIVideo   → SVD animates the photo directly
//	StyleAICartoon → cartoonize the photo, then SVD animates the cartoon
//
// Requires a non-empty token. The output is written to cfg.OutputPath as MP4.
func GenerateAIVideo(ctx context.Context, cfg VideoConfig, token string) error {
	if token == "" {
		return fmt.Errorf("huggingface token not configured")
	}
	if len(cfg.InputImages) == 0 {
		return fmt.Errorf("no input images provided")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	client := newHFClient(token)

	// The image we animate. For ai_cartoon we first replace it with a cartoonized version.
	sourceImage := cfg.InputImages[0]

	if cfg.AnimationStyle == StyleAICartoon {
		cartoonPath, err := client.cartoonize(ctx, sourceImage, cfg.OutputPath)
		if err != nil {
			return fmt.Errorf("cartoonize step: %w", err)
		}
		defer os.Remove(cartoonPath)
		sourceImage = cartoonPath
	}

	videoBytes, err := client.animate(ctx, sourceImage)
	if err != nil {
		return fmt.Errorf("animate step: %w", err)
	}

	if err := os.WriteFile(cfg.OutputPath, videoBytes, 0o644); err != nil {
		return fmt.Errorf("write output video: %w", err)
	}
	return nil
}

// cartoonize sends the image to the cartoonizer model and saves the result as a
// PNG next to the output path. Returns the cartoon image path.
func (c *hfClient) cartoonize(ctx context.Context, imagePath, outputPath string) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read input image: %w", err)
	}

	respBytes, err := c.inferBinary(ctx, hfModelCartoonize, imgData)
	if err != nil {
		return "", err
	}

	cartoonPath := outputPath + ".cartoon.png"
	if err := os.WriteFile(cartoonPath, respBytes, 0o644); err != nil {
		return "", fmt.Errorf("write cartoon image: %w", err)
	}
	return cartoonPath, nil
}

// animate sends the image to the SVD model and returns the resulting MP4 bytes.
func (c *hfClient) animate(ctx context.Context, imagePath string) ([]byte, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read image to animate: %w", err)
	}
	return c.inferBinary(ctx, hfModelSVD, imgData)
}

// inferBinary POSTs raw image bytes to a Hugging Face model and returns the raw
// binary response (image or video bytes). It retries on HTTP 503 (model loading)
// until hfMaxWait elapses.
func (c *hfClient) inferBinary(ctx context.Context, model string, body []byte) ([]byte, error) {
	url := hfBaseURL + model
	deadline := time.Now().Add(hfMaxWait)

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/octet-stream")
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
