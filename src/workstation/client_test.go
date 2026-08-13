package workstation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fixture(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(handler)
	c, err := NewClient(Config{URL: s.URL + "/api", Username: "u", Password: "p"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, s
}

func TestClient_GetDecodesJSON(t *testing.T) {
	c, s := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/vms" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "u" || p != "p" {
			t.Errorf("expected basic auth u/p, got %s/%s ok=%v", u, p, ok)
		}
		if r.Header.Get("Accept") != vmrestContentType {
			t.Errorf("expected Accept %s, got %s", vmrestContentType, r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", vmrestContentType)
		_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "vm-1", "path": "/tmp/vm-1.vmx"}})
	})
	defer s.Close()

	var out []map[string]string
	if err := c.Do(context.Background(), http.MethodGet, "/vms", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(out) != 1 || out[0]["id"] != "vm-1" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestClient_PostEncodesJSONBody(t *testing.T) {
	var gotBody map[string]interface{}
	c, s := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != vmrestContentType {
			t.Errorf("expected Content-Type %s, got %s", vmrestContentType, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "vm-2"})
	})
	defer s.Close()

	var out map[string]string
	err := c.Do(context.Background(), http.MethodPost, "/vms", map[string]string{"name": "clone1", "parentId": "vm-1"}, &out)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotBody["name"] != "clone1" || gotBody["parentId"] != "vm-1" {
		t.Fatalf("unexpected request body: %+v", gotBody)
	}
	if out["id"] != "vm-2" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestClient_DoRawBodySendsBareString(t *testing.T) {
	var gotBody []byte
	c, s := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != vmrestContentType {
			t.Errorf("expected Content-Type %s, got %s", vmrestContentType, r.Header.Get("Content-Type"))
		}
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		gotBody = buf[:n]
		w.Header().Set("Content-Type", vmrestContentType)
		_ = json.NewEncoder(w).Encode(map[string]string{"power_state": "poweredOn"})
	})
	defer s.Close()

	var out map[string]string
	err := c.DoRawBody(context.Background(), http.MethodPut, "/vms/vm-1/power", []byte("on"), &out)
	if err != nil {
		t.Fatalf("DoRawBody: %v", err)
	}
	if string(gotBody) != "on" {
		t.Fatalf("expected bare body \"on\", got %q", gotBody)
	}
	if out["power_state"] != "poweredOn" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestClient_ErrorModelDecoded(t *testing.T) {
	c, s := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Code": 0, "Message": "No such resource"})
	})
	defer s.Close()

	err := c.Do(context.Background(), http.MethodGet, "/vms/nope", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestClient_UnauthorizedNoBody(t *testing.T) {
	c, s := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	defer s.Close()

	err := c.Do(context.Background(), http.MethodGet, "/vms", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewClient_RejectsMalformedURL(t *testing.T) {
	if _, err := NewClient(Config{URL: "not-a-url"}); err == nil {
		t.Fatal("expected error for malformed URL")
	}
	if _, err := NewClient(Config{URL: ""}); err == nil {
		t.Fatal("expected error for empty URL")
	}
}
