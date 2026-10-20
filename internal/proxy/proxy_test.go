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

func TestConnectRequiresPairUpstream(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sess := session.New("test-secret-key-at-least-16")
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer main.Close()
	px, err := New(main.URL, st, sess)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/x", nil)
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUnifiedConnectSetsCollabCookieAfterPairOK(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.UpsertUser(store.User{ID: "u-tom", Username: "tom", Nickname: "汤姆猫"})
	_ = st.PutBinding("unified", "u-tom", time.Minute)

	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "main", http.StatusTeapot)
	}))
	defer main.Close()
	pair := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"oc"}`))
	}))
	defer pair.Close()

	sess := session.New("test-secret-key-at-least-16")
	px, err := NewWithOptions(Options{Upstream: main.URL, UpstreamPair: pair.URL, Store: st, Sessions: sess})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/unified", nil)
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"token":"oc"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sid string
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("missing collab cookie on unified redeem")
	}
	if _, err := st.GetBinding("unified"); err != store.ErrNotFound {
		t.Fatalf("binding should be consumed, err=%v", err)
	}

	// enrich prompt with collab session
	req = httptest.NewRequest(http.MethodPost, "/api/session/ses_1/prompt", strings.NewReader(`{"text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sid})
	main.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	rec = httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prompt status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"sender":"u-tom"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestConnectWithoutBindingDoesNotSetCollabCookie(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "main", http.StatusTeapot)
	}))
	defer main.Close()
	pair := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			t.Fatal("unexpected collab cookie without binding")
		}
	}
}

func TestConnectPairFailureKeepsBinding(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.UpsertUser(store.User{ID: "u1", Username: "a"})
	_ = st.PutBinding("fake", "u1", time.Minute)

	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer main.Close()
	pair := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `expired`, http.StatusUnauthorized)
	}))
	defer pair.Close()

	sess := session.New("test-secret-key-at-least-16")
	px, err := NewWithOptions(Options{Upstream: main.URL, UpstreamPair: pair.URL, Store: st, Sessions: sess})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/fake", nil)
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			t.Fatal("must not set collab cookie on pair failure")
		}
	}
	if _, err := st.GetBinding("fake"); err != nil {
		t.Fatalf("binding should remain, err=%v", err)
	}
}

func TestHTMLInject(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.UpsertUser(store.User{ID: "u1", Username: "alice"})
	_ = st.PutBinding("c", "u1", time.Minute)

	pair := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"t"}`))
	}))
	defer pair.Close()
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>hi</body></html>`))
	}))
	defer main.Close()

	sess := session.New("test-secret-key-at-least-16")
	px, err := NewWithOptions(Options{Upstream: main.URL, UpstreamPair: pair.URL, Store: st, Sessions: sess})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/connect/c", nil)
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	var sid string
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("missing sid")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sid})
	rec = httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "opencode-collab-session-avatar") {
		t.Fatalf("missing inject: %s", rec.Body.String())
	}
}
