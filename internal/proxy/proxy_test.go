package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/frameworkease/opencode-collab/internal/session"
	"github.com/frameworkease/opencode-collab/internal/store"
)

func TestDirectorPreservesClientHost(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var sawHost string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawHost = r.Host
		w.WriteHeader(http.StatusNoContent)
	}))
	defer up.Close()
	sess := session.New("test-secret-key-at-least-16")
	px, err := New(up.URL, st, sess)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.Host = "127.0.0.1:4095"
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if sawHost != "127.0.0.1:4095" {
		t.Fatalf("upstream Host=%q, want client host", sawHost)
	}
}

func TestRedeemAndEnrich(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.UpsertUser(store.User{ID: "u1", Username: "alice", Nickname: "Alice"})
	_ = st.PutPair("paircode", "u1", time.Minute)

	connectHits := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/auth/connect/"):
			connectHits++
			http.Error(w, `expired`, http.StatusUnauthorized)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		case r.URL.Path == "/" || r.URL.Path == "":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head></head><body>hi</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer up.Close()

	sess := session.New("test-secret-key-at-least-16")
	px, err := New(up.URL, st, sess)
	if err != nil {
		t.Fatal(err)
	}

	// redeem
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/paircode", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("connect status=%d body=%s", rec.Code, rec.Body.String())
	}
	if connectHits != 0 {
		t.Fatalf("collab pair must not hit upstream connect, hits=%d", connectHits)
	}
	var sid string
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("missing opencode_collab_sid cookie")
	}
	if _, err := st.GetPair("paircode"); err != store.ErrNotFound {
		t.Fatalf("pair should be consumed, err=%v", err)
	}

	// enrich prompt
	req = httptest.NewRequest(http.MethodPost, "/api/session/ses_1/prompt", strings.NewReader(`{"text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sid})
	rec = httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prompt status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"sender":"u1"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}

	// HTML inject
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sid})
	rec = httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "opencode-collab-session-avatar") {
		t.Fatalf("missing inject: %s", rec.Body.String())
	}
}

func TestConnectFallsThroughToPairUpstream(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mainHits, pairHits := 0, 0
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHits++
		http.Error(w, "main", http.StatusTeapot)
	}))
	defer main.Close()
	pair := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pairHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"t"}`))
	}))
	defer pair.Close()

	sess := session.New("test-secret-key-at-least-16")
	px, err := NewWithOptions(Options{Upstream: main.URL, UpstreamPair: pair.URL, Store: st, Sessions: sess})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/opencodecode", nil)
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if pairHits != 1 || mainHits != 0 {
		t.Fatalf("pairHits=%d mainHits=%d", pairHits, mainHits)
	}
}
