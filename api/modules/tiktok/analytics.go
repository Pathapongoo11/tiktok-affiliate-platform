package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type VideoMetrics struct {
	VideoID    string
	Views      int64
	Likes      int64
	Comments   int64
	Shares     int64
	RecordedAt time.Time
}

func (c *Client) GetVideoMetrics(ctx context.Context, accessToken, videoID string) (*VideoMetrics, error) {
	if c.mockMode {
		seed := time.Now().Unix()
		return &VideoMetrics{
			VideoID:    videoID,
			Views:      1000 + seed%5000,
			Likes:      50 + seed%500,
			Comments:   5 + seed%50,
			Shares:     2 + seed%20,
			RecordedAt: time.Now(),
		}, nil
	}
	body := fmt.Sprintf(`{"filters":{"video_ids":["%s"]}}`, videoID)
	apiURL := BaseURL + "/video/query/?fields=id,like_count,comment_count,share_count,view_count"
	resp, err := c.doRequest(ctx, "POST", apiURL, strings.NewReader(body), accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Data struct {
			Videos []struct {
				ID           string `json:"id"`
				LikeCount    int64  `json:"like_count"`
				CommentCount int64  `json:"comment_count"`
				ShareCount   int64  `json:"share_count"`
				ViewCount    int64  `json:"view_count"`
			} `json:"videos"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode metrics response: %w", err)
	}
	if len(result.Data.Videos) == 0 {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}
	v := result.Data.Videos[0]
	return &VideoMetrics{
		VideoID:    v.ID,
		Views:      v.ViewCount,
		Likes:      v.LikeCount,
		Comments:   v.CommentCount,
		Shares:     v.ShareCount,
		RecordedAt: time.Now(),
	}, nil
}
