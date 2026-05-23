package twitch

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"TuberSwitch/internal/config"
)

func testClient(serverURL string) *Client {
	client := New(log.Default())
	client.authURL = serverURL
	client.apiURL = serverURL
	return client
}

func TestStartDeviceFlow(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/device" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		_ = json.NewEncoder(w).Encode(DeviceAuthorization{
			DeviceCode:      "device",
			UserCode:        "ABCD",
			VerificationURI: "https://twitch.tv/activate",
			ExpiresIn:       60,
			Interval:        1,
		})
	}))
	defer server.Close()

	device, err := testClient(server.URL).StartDeviceFlow(context.Background(), config.TwitchConfig{ClientID: "client"})
	if err != nil {
		t.Fatalf("StartDeviceFlow: %v", err)
	}
	if device.DeviceCode != "device" || device.UserCode != "ABCD" {
		t.Fatalf("device = %#v", device)
	}
	if gotForm.Get("client_id") != "client" {
		t.Fatalf("client_id = %q", gotForm.Get("client_id"))
	}
	if !strings.Contains(gotForm.Get("scopes"), "channel:manage:redemptions") {
		t.Fatalf("scopes = %q", gotForm.Get("scopes"))
	}
}

func TestFetchRewardsManageableFlagAndQuery(t *testing.T) {
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "1", "title": "Dance", "is_enabled": true},
			},
		})
	}))
	defer server.Close()

	cfg := config.TwitchConfig{ClientID: "client", AccessToken: "token", ChannelID: "channel"}
	rewards, err := testClient(server.URL).FetchManageableRewards(context.Background(), cfg)
	if err != nil {
		t.Fatalf("FetchManageableRewards: %v", err)
	}
	if !strings.Contains(rawQuery, "only_manageable_rewards=true") {
		t.Fatalf("query = %q", rawQuery)
	}
	if len(rewards) != 1 || !rewards[0].Manageable {
		t.Fatalf("rewards = %#v", rewards)
	}
}

func TestCreateReward(t *testing.T) {
	var method string
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.Header.Get("Client-Id") != "client" {
			t.Fatalf("Client-Id header = %q", r.Header.Get("Client-Id"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization header = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "new", "title": "Throw Tomato", "is_enabled": true},
			},
		})
	}))
	defer server.Close()

	cfg := config.TwitchConfig{ClientID: "client", AccessToken: "token", ChannelID: "channel"}
	reward, err := testClient(server.URL).CreateReward(context.Background(), cfg, "Throw Tomato", 500, "Aim carefully")
	if err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s", method)
	}
	if body["title"] != "Throw Tomato" || body["prompt"] != "Aim carefully" || body["cost"].(float64) != 500 {
		t.Fatalf("body = %#v", body)
	}
	if reward.ID != "new" || !reward.Manageable {
		t.Fatalf("reward = %#v", reward)
	}
}

func TestCreateRewardValidation(t *testing.T) {
	cfg := config.TwitchConfig{ClientID: "client", AccessToken: "token", ChannelID: "channel"}
	client := testClient("http://example.invalid")
	if _, err := client.CreateReward(context.Background(), cfg, "", 100, ""); err == nil {
		t.Fatalf("expected missing title error")
	}
	if _, err := client.CreateReward(context.Background(), cfg, "Reward", 0, ""); err == nil {
		t.Fatalf("expected cost error")
	}
}

func TestRefreshTokenWithPublicClient(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	cfg := config.TwitchConfig{ClientID: "client", RefreshToken: "refresh"}
	updated, err := testClient(server.URL).RefreshToken(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if gotForm.Get("client_secret") != "" {
		t.Fatalf("client_secret should be omitted, got %q", gotForm.Get("client_secret"))
	}
	if updated.AccessToken != "new-access" || updated.RefreshToken != "new-refresh" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestLoadUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "123", "display_name": "Streamer", "login": "streamer"},
			},
		})
	}))
	defer server.Close()

	cfg := config.TwitchConfig{ClientID: "client", AccessToken: "token"}
	updated, err := testClient(server.URL).LoadUser(context.Background(), cfg)
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if updated.ChannelID != "123" || updated.ChannelName != "Streamer" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestSetRewardEnabled(t *testing.T) {
	var method string
	var body map[string]bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.URL.Query().Get("id") != "reward" {
			t.Fatalf("reward id query = %q", r.URL.RawQuery)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := config.TwitchConfig{ClientID: "client", AccessToken: "token", ChannelID: "channel"}
	err := testClient(server.URL).SetRewardEnabled(context.Background(), cfg, "reward", false)
	if err != nil {
		t.Fatalf("SetRewardEnabled: %v", err)
	}
	if method != http.MethodPatch {
		t.Fatalf("method = %s", method)
	}
	if body["is_enabled"] {
		t.Fatalf("body = %#v", body)
	}
}

func TestAPIErrorIncludesBody(t *testing.T) {
	resp := &http.Response{
		Status:     "403 Forbidden",
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`{"message":"forbidden"}`)),
	}
	err := apiError(resp)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitForDeviceTokenExpires(t *testing.T) {
	_, err := testClient("http://example.invalid").WaitForDeviceToken(
		context.Background(),
		config.TwitchConfig{ClientID: "client"},
		DeviceAuthorization{DeviceCode: "device", ExpiresIn: -1},
	)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitForDeviceTokenPendingThenSuccess(t *testing.T) {
	tokenPolls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			tokenPolls++
			if tokenPolls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":400,"message":"authorization_pending"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "access",
				"refresh_token": "refresh",
				"expires_in":    3600,
			})
		case "/helix/users":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "123", "display_name": "Streamer"},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, err := testClient(server.URL).WaitForDeviceToken(
		context.Background(),
		config.TwitchConfig{ClientID: "client"},
		DeviceAuthorization{DeviceCode: "device", ExpiresIn: 5, Interval: 1},
	)
	if err != nil {
		t.Fatalf("WaitForDeviceToken: %v", err)
	}
	if tokenPolls != 2 {
		t.Fatalf("token polls = %d", tokenPolls)
	}
	if cfg.AccessToken != "access" || cfg.ChannelName != "Streamer" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestIsAuthorizationPending(t *testing.T) {
	if !isAuthorizationPending(fakeErr("Twitch API returned 400: authorization_pending")) {
		t.Fatalf("expected authorization pending")
	}
	if isAuthorizationPending(fakeErr("invalid_grant")) {
		t.Fatalf("unexpected authorization pending match")
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
