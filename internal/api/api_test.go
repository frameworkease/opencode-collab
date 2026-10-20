package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/frameworkease/opencode-collab/internal/store"
)

func TestBindAuthAndProfile(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := &Handler{Store: st, BindToken: "secret"}
	mux := h.Routes()

	body := []byte(`{"code":"abc","user":{"id":"u1","username":"alice","nickname":"Alice"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/bind", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/bind", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/profile/u1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile want 200, got %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	user := got["user"].(map[string]any)
	if user["id"] != "u1" || user["nickname"] != "Alice" {
		t.Fatalf("user=%v", user)
	}
}

func TestBindWithoutTokenConfigured(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &Handler{Store: st, BindToken: ""}
	mux := h.Routes()
	body := []byte(`{"code":"c","user":{"id":"u2","username":"b"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/bind", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
}
