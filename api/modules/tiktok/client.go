package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/config"
)

const (
	BaseURL  = "https://open.tiktokapis.com/v2"
	AuthURL  = "https://www.tiktok.com/v2/auth/authorize"
	TokenURL = "https://open.tiktokapis.com/v2/oauth/token"
)

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	mockMode   bool
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		mockMode:   cfg.TikTokClientKey == "" || cfg.TikTokClientSecret == "",
	}
}

func (c *Client) IsMockMode() bool { return c.mockMode }

// UserInfo holds the subset of TikTok user profile data returned by /v2/user/info/.
type UserInfo struct {
	OpenID      string `json:"open_id"`
	UnionID     string `json:"union_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

// GetUserInfo fetches the authenticated user's TikTok profile.
// In mock mode it returns a placeholder; in real mode it calls /v2/user/info/.
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	if c.mockMode {
		return &UserInfo{
			OpenID:      "mock_tiktok_user_id_001",
			DisplayName: "Mock TikTok User",
		}, nil
	}

	resp, err := c.doRequest(ctx, "GET",
		BaseURL+"/user/info/?fields=open_id,union_id,display_name,avatar_url",
		nil, accessToken)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			User UserInfo `json:"user"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}
	if result.Error.Code != "" && result.Error.Code != "ok" {
		return nil, fmt.Errorf("tiktok error: %s — %s", result.Error.Code, result.Error.Message)
	}
	return &result.Data.User, nil
}

func (c *Client) doRequest(ctx context.Context, method, url string, body io.Reader, accessToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return c.httpClient.Do(req)
}
