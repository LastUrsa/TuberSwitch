package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"TuberSwitch/internal/config"
)

const rewardScopes = "channel:read:redemptions channel:manage:redemptions"

type Client struct {
	logger  *log.Logger
	http    *http.Client
	authURL string
	apiURL  string
}

type Reward struct {
	ID         string
	Title      string
	Enabled    bool
	Manageable bool
}

func New(logger *log.Logger) *Client {
	return &Client{
		logger:  logger,
		http:    &http.Client{Timeout: 20 * time.Second},
		authURL: "https://id.twitch.tv",
		apiURL:  "https://api.twitch.tv",
	}
}

func (c *Client) StartDeviceFlow(ctx context.Context, cfg config.TwitchConfig) (DeviceAuthorization, error) {
	if cfg.ClientID == "" {
		return DeviceAuthorization{}, fmt.Errorf("Twitch client ID is required")
	}
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("scopes", rewardScopes)
	var device DeviceAuthorization
	if err := c.form(ctx, c.authURL+"/oauth2/device", values, &device); err != nil {
		return DeviceAuthorization{}, err
	}
	return device, nil
}

func (c *Client) WaitForDeviceToken(ctx context.Context, cfg config.TwitchConfig, device DeviceAuthorization) (config.TwitchConfig, error) {
	if cfg.ClientID == "" {
		return cfg, fmt.Errorf("Twitch client ID is required")
	}
	interval := time.Duration(device.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	for {
		if time.Now().After(deadline) {
			return cfg, fmt.Errorf("Twitch device login expired")
		}
		select {
		case <-ctx.Done():
			return cfg, ctx.Err()
		case <-time.After(interval):
		}

		values := url.Values{}
		values.Set("client_id", cfg.ClientID)
		values.Set("scopes", rewardScopes)
		values.Set("device_code", device.DeviceCode)
		values.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		var token tokenResponse
		err := c.form(ctx, c.authURL+"/oauth2/token", values, &token)
		if err != nil {
			if isAuthorizationPending(err) {
				continue
			}
			return cfg, err
		}
		cfg.AccessToken = token.AccessToken
		cfg.RefreshToken = token.RefreshToken
		cfg.TokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339)
		return c.LoadUser(ctx, cfg)
	}
}

func (c *Client) RefreshToken(ctx context.Context, cfg config.TwitchConfig) (config.TwitchConfig, error) {
	if cfg.ClientID == "" || cfg.RefreshToken == "" {
		return cfg, fmt.Errorf("Twitch refresh requires client ID and refresh token")
	}
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", cfg.RefreshToken)
	var token tokenResponse
	if err := c.form(ctx, c.authURL+"/oauth2/token", values, &token); err != nil {
		return cfg, err
	}
	cfg.AccessToken = token.AccessToken
	if token.RefreshToken != "" {
		cfg.RefreshToken = token.RefreshToken
	}
	cfg.TokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339)
	return cfg, nil
}

func (c *Client) EnsureToken(ctx context.Context, cfg config.TwitchConfig) (config.TwitchConfig, error) {
	if cfg.AccessToken == "" {
		return cfg, fmt.Errorf("Twitch is not authenticated")
	}
	expiry, err := time.Parse(time.RFC3339, cfg.TokenExpiry)
	if err == nil && time.Until(expiry) > 2*time.Minute {
		return cfg, nil
	}
	return c.RefreshToken(ctx, cfg)
}

func (c *Client) LoadUser(ctx context.Context, cfg config.TwitchConfig) (config.TwitchConfig, error) {
	var response struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Login       string `json:"login"`
		} `json:"data"`
	}
	if err := c.get(ctx, cfg, c.apiURL+"/helix/users", &response); err != nil {
		return cfg, err
	}
	if len(response.Data) == 0 {
		return cfg, fmt.Errorf("Twitch returned no authenticated user")
	}
	cfg.ChannelID = response.Data[0].ID
	cfg.ChannelName = response.Data[0].DisplayName
	if cfg.ChannelName == "" {
		cfg.ChannelName = response.Data[0].Login
	}
	return cfg, nil
}

func (c *Client) FetchRewards(ctx context.Context, cfg config.TwitchConfig) ([]Reward, error) {
	return c.fetchRewards(ctx, cfg, false)
}

func (c *Client) FetchManageableRewards(ctx context.Context, cfg config.TwitchConfig) ([]Reward, error) {
	return c.fetchRewards(ctx, cfg, true)
}

func (c *Client) fetchRewards(ctx context.Context, cfg config.TwitchConfig, onlyManageable bool) ([]Reward, error) {
	if cfg.ChannelID == "" {
		return nil, fmt.Errorf("Twitch channel ID is missing")
	}
	endpoint := c.apiURL + "/helix/channel_points/custom_rewards?broadcaster_id=" + url.QueryEscape(cfg.ChannelID)
	if onlyManageable {
		endpoint += "&only_manageable_rewards=true"
	}
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Enabled bool   `json:"is_enabled"`
		} `json:"data"`
	}
	if err := c.get(ctx, cfg, endpoint, &response); err != nil {
		return nil, err
	}
	rewards := make([]Reward, 0, len(response.Data))
	for _, item := range response.Data {
		rewards = append(rewards, Reward{ID: item.ID, Title: item.Title, Enabled: item.Enabled, Manageable: onlyManageable})
	}
	c.logger.Printf("Twitch rewards fetched: %d", len(rewards))
	return rewards, nil
}

func (c *Client) CreateReward(ctx context.Context, cfg config.TwitchConfig, title string, cost int, prompt string) (Reward, error) {
	if cfg.ChannelID == "" {
		return Reward{}, fmt.Errorf("Twitch channel ID is missing")
	}
	if strings.TrimSpace(title) == "" {
		return Reward{}, fmt.Errorf("reward title is required")
	}
	if cost < 1 {
		return Reward{}, fmt.Errorf("reward cost must be at least 1")
	}
	endpoint := c.apiURL + "/helix/channel_points/custom_rewards?broadcaster_id=" + url.QueryEscape(cfg.ChannelID)
	body := map[string]interface{}{
		"title": title,
		"cost":  cost,
	}
	if strings.TrimSpace(prompt) != "" {
		body["prompt"] = prompt
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Reward{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return Reward{}, err
	}
	c.authHeaders(req, cfg)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Reward{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Reward{}, apiError(resp)
	}
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Enabled bool   `json:"is_enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return Reward{}, err
	}
	if len(response.Data) == 0 {
		return Reward{}, fmt.Errorf("Twitch returned no created reward")
	}
	reward := response.Data[0]
	return Reward{ID: reward.ID, Title: reward.Title, Enabled: reward.Enabled, Manageable: true}, nil
}

func (c *Client) SetRewardEnabled(ctx context.Context, cfg config.TwitchConfig, rewardID string, enabled bool) error {
	if cfg.ChannelID == "" {
		return fmt.Errorf("Twitch channel ID is missing")
	}
	endpoint := c.apiURL + "/helix/channel_points/custom_rewards?broadcaster_id=" +
		url.QueryEscape(cfg.ChannelID) + "&id=" + url.QueryEscape(rewardID)
	body := map[string]bool{"is_enabled": enabled}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.authHeaders(req, cfg)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}
	return nil
}

func (c *Client) get(ctx context.Context, cfg config.TwitchConfig, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.authHeaders(req, cfg)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) form(ctx context.Context, endpoint string, values url.Values, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) authHeaders(req *http.Request, cfg config.TwitchConfig) {
	req.Header.Set("Client-Id", cfg.ClientID)
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
}

func apiError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(data) == 0 {
		return fmt.Errorf("Twitch API returned %s", resp.Status)
	}
	return fmt.Errorf("Twitch API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
}

func isAuthorizationPending(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "authorization_pending") || strings.Contains(msg, "authorization pending")
}

type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
