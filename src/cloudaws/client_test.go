package cloudaws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func fixture(t *testing.T, authHandler, apiHandler http.HandlerFunc) (*Client, *httptest.Server, *httptest.Server) {
	t.Helper()
	authSrv := httptest.NewServer(authHandler)
	apiSrv := httptest.NewServer(apiHandler)
	c, err := NewClient(Config{RefreshToken: "refresh-abc", AuthURL: authSrv.URL, BaseURL: apiSrv.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, authSrv, apiSrv
}

func TestClient_TokenExchangeThenAPICall(t *testing.T) {
	var authCalls, apiCalls int
	c, authSrv, apiSrv := fixture(t,
		func(w http.ResponseWriter, r *http.Request) {
			authCalls++
			if r.URL.Query().Get("refresh_token") != "refresh-abc" {
				t.Errorf("expected refresh_token=refresh-abc, got %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access-1", "expires_in": 1800})
		},
		func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer access-1" {
				t.Errorf("expected Authorization: Bearer access-1, got %s", got)
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "org-1"}})
		})
	defer authSrv.Close()
	defer apiSrv.Close()

	var out []map[string]string
	if err := c.Do(context.Background(), http.MethodGet, "/vmc/api/orgs", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(out) != 1 || out[0]["id"] != "org-1" {
		t.Fatalf("unexpected result: %+v", out)
	}
	if authCalls != 1 {
		t.Fatalf("expected exactly 1 token exchange, got %d", authCalls)
	}
	if apiCalls != 1 {
		t.Fatalf("expected exactly 1 API call, got %d", apiCalls)
	}

	// Second call reuses the cached token — no second auth round-trip.
	if err := c.Do(context.Background(), http.MethodGet, "/vmc/api/orgs", nil, &out); err != nil {
		t.Fatalf("Do (2nd): %v", err)
	}
	if authCalls != 1 {
		t.Fatalf("expected token to be cached (still 1 exchange), got %d", authCalls)
	}
	if apiCalls != 2 {
		t.Fatalf("expected 2 API calls, got %d", apiCalls)
	}
}

func TestClient_ExpiredTokenTriggersRefresh(t *testing.T) {
	var authCalls int
	c, authSrv, apiSrv := fixture(t,
		func(w http.ResponseWriter, r *http.Request) {
			authCalls++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access-1", "expires_in": 1})
		},
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
		})
	defer authSrv.Close()
	defer apiSrv.Close()

	if err := c.Do(context.Background(), http.MethodGet, "/vmc/api/orgs", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	time.Sleep(1100 * time.Millisecond) // let the 1s token + 60s-clamped buffer logic force expiry (ttl clamped to buffer, already in the past after sleep)

	if err := c.Do(context.Background(), http.MethodGet, "/vmc/api/orgs", nil, nil); err != nil {
		t.Fatalf("Do (after expiry): %v", err)
	}
	if authCalls < 2 {
		t.Fatalf("expected a 2nd token exchange after expiry, got %d total", authCalls)
	}
}

func TestClient_401TriggersOneRetryThenFails(t *testing.T) {
	var authCalls, apiCalls int
	c, authSrv, apiSrv := fixture(t,
		func(w http.ResponseWriter, r *http.Request) {
			authCalls++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access-1", "expires_in": 1800})
		},
		func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.WriteHeader(http.StatusUnauthorized)
		})
	defer authSrv.Close()
	defer apiSrv.Close()

	err := c.Do(context.Background(), http.MethodGet, "/vmc/api/orgs", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if authCalls != 2 {
		t.Fatalf("expected exactly 2 token exchanges (initial + 1 forced retry), got %d", authCalls)
	}
	if apiCalls != 2 {
		t.Fatalf("expected exactly 2 API attempts (original + 1 retry), got %d", apiCalls)
	}
}

func TestClient_ErrorModelDecoded(t *testing.T) {
	c, authSrv, apiSrv := fixture(t,
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access-1", "expires_in": 1800})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error_code": "SDDC_LIMIT", "error_message": "SDDC limit reached"})
		})
	defer authSrv.Close()
	defer apiSrv.Close()

	err := c.Do(context.Background(), http.MethodPost, "/vmc/api/orgs/org-1/sddcs", map[string]string{"name": "x"}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewClient_RequiresRefreshToken(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("expected error for missing RefreshToken")
	}
}
