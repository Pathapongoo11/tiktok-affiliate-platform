package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	return &PostVideoResponse{PublishID: initResp.Data.PublishID, Status: "uploading"}, nil
}
