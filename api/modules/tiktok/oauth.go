package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	OpenID       string `json:"open_id"`
	Scope        string `json:"scope"`
}

func (c *Client) GetAuthURL(state string) string {
	if c.mockMode {
		return "/api/tiktok/mock-oauth-callback?state=" + state + "&code=mock_code_12345"
	}
	params := url.Values{
		"client_key":    {c.cfg.TikTokClientKey},
		"scope":         {"user.info.basic,video.publish,video.upload"},
		"response_type": {"code"},
		"redirect_uri":  {c.cfg.TikTokRedirectURI},
		"state":         {state},
	}
	return AuthURL + "?" + params.Encode()
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	if c.mockMode {
		return &TokenResponse{
			AccessToken:  "mock_access_token_" + code,
			RefreshToken: "mock_refresh_token",
			ExpiresIn:    86400,
			OpenID:       "mock_tiktok_user_id_001",
			Scope:        "user.info.basic,video.publish,video.upload",
		}, nil
	}
	params := url.Values{
		"client_key":    {c.cfg.TikTokClientKey},
		"client_secret": {c.cfg.TikTokClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.cfg.TikTokRedirectURI},
	}
	resp, err := c.doRequest(ctx, "POST", TokenURL, strings.NewReader(params.Encode()), "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &token, nil
}

func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if c.mockMode {
		return &TokenResponse{
			AccessToken:  "mock_access_token_refreshed_" + fmt.Sprint(time.Now().Unix()),
			RefreshToken: refreshToken,
			ExpiresIn:    86400,
		}, nil
	}
	params := url.Values{
		"client_key":    {c.cfg.TikTokClientKey},
		"client_secret": {c.cfg.TikTokClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	resp, err := c.doRequest(ctx, "POST", TokenURL, strings.NewReader(params.Encode()), "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	return &token, nil
}
