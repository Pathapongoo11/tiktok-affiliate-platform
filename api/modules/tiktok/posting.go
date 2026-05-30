package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type PostVideoRequest struct {
	AccessToken  string
	VideoPath    string
	Caption      string
	Hashtags     []string
	PrivacyLevel string
}

type PostVideoResponse struct {
	PublishID     string
	TikTokVideoID string
	Status        string
}

func (c *Client) PostVideo(ctx context.Context, req PostVideoRequest) (*PostVideoResponse, error) {
	if c.mockMode {
		return &PostVideoResponse{
			PublishID:     fmt.Sprintf("mock_publish_%d", time.Now().Unix()),
			TikTokVideoID: fmt.Sprintf("mock_video_%d", time.Now().Unix()),
			Status:        "published_to_draft",
		}, nil
	}

	caption := req.Caption
	for _, tag := range req.Hashtags {
		caption += " #" + tag
	}

	info, _ := os.Stat(req.VideoPath)
	fileSize := int64(0)
	if info != nil {
		fileSize = info.Size()
	}

	initBody := map[string]any{
		"post_info": map[string]any{
			"title":           caption,
			"privacy_level":   req.PrivacyLevel,
			"disable_duet":    false,
			"disable_stitch":  false,
			"disable_comment": false,
		},
		"source_info": map[string]any{
			"source":            "FILE_UPLOAD",
			"video_size":        fileSize,
			"chunk_size":        fileSize,
			"total_chunk_count": 1,
		},
	}
	bodyBytes, _ := json.Marshal(initBody)
	resp, err := c.doRequest(ctx, "POST", BaseURL+"/post/publish/video/init/", bytes.NewReader(bodyBytes), req.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("init upload: %w", err)
	}
	defer resp.Body.Close()

	var initResp struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return nil, fmt.Errorf("decode init response: %w", err)
	}
	if initResp.Error.Code != "" && initResp.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok error: %s — %s", initResp.Error.Code, initResp.Error.Message)
	}

	videoData, err := os.ReadFile(req.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("read video: %w", err)
	}

	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", initResp.Data.UploadURL, bytes.NewReader(videoData))
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", "video/mp4")
	uploadReq.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(videoData)-1, len(videoData)))
	uploadResp, err := c.httpClient.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("upload video: %w", err)
	}
	if uploadResp != nil {
		uploadResp.Body.Close()
	}

	// TikTok video publishing is async: the upload gives us a publish_id, and we
	// must poll /v2/post/publish/status/fetch/ until PUBLISH_COMPLETE to get the
	// real video_id that can be used to query analytics.
	videoID, err := c.pollPublishStatus(ctx, req.AccessToken, initResp.Data.PublishID)
	if err != nil {
		// Return partial success with publish_id so the caller can log it,
		// but surface the error so the post status can be set to "failed".
		return nil, fmt.Errorf("publish status: %w", err)
	}

	return &PostVideoResponse{
		PublishID:     initResp.Data.PublishID,
		TikTokVideoID: videoID,
		Status:        "published",
	}, nil
}

// pollPublishStatus polls the TikTok publish-status endpoint until the video
// reaches PUBLISH_COMPLETE (returns video_id) or PUBLISH_FAILED (returns error).
// It gives up after 90 s of polling in 5 s intervals.
func (c *Client) pollPublishStatus(ctx context.Context, accessToken, publishID string) (string, error) {
	const (
		pollInterval = 5 * time.Second
		timeout      = 90 * time.Second
	)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		body := fmt.Sprintf(`{"publish_id":%q}`, publishID)
		resp, err := c.doRequest(ctx, "POST",
			BaseURL+"/post/publish/status/fetch/",
			strings.NewReader(body), accessToken)
		if err != nil {
			return "", fmt.Errorf("fetch status: %w", err)
		}

		var statusResp struct {
			Data struct {
				Status           string `json:"status"`
				PublishedVideoID string `json:"published_video_id"`
				FailReason       string `json:"fail_reason"`
			} `json:"data"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&statusResp)
		resp.Body.Close()

		if decodeErr != nil {
			return "", fmt.Errorf("decode status: %w", decodeErr)
		}
		if statusResp.Error.Code != "" && statusResp.Error.Code != "ok" {
			return "", fmt.Errorf("tiktok error: %s — %s",
				statusResp.Error.Code, statusResp.Error.Message)
		}

		switch statusResp.Data.Status {
		case "PUBLISH_COMPLETE":
			return statusResp.Data.PublishedVideoID, nil
		case "PUBLISH_FAILED":
			return "", fmt.Errorf("TikTok publish failed: %s", statusResp.Data.FailReason)
		}
		// Other statuses (PROCESSING_DOWNLOAD, IN_REVIEW, AWAITING_SCHEDULING, …) → keep polling

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return "", fmt.Errorf("publish status polling timed out after %s (publish_id: %s)", timeout, publishID)
}
