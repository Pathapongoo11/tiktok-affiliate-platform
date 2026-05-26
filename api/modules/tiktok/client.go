package tiktok

import (
	"context"
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
