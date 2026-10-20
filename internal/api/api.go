package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/frameworkease/opencode-collab/internal/inject"
	"github.com/frameworkease/opencode-collab/internal/session"
	"github.com/frameworkease/opencode-collab/internal/store"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	Store     *store.Store
	BindToken string
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Mount("/assets", inject.AssetRoutes())
	r.Route("/v1", func(r chi.Router) {
		r.With(h.requireBindToken).Post("/bind", h.bind)
		r.Get("/profile/{userId}", h.profile)
		r.Get("/avatar/{userId}", h.avatar)
	})
	return r
}

func (h *Handler) requireBindToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := session.CheckBindToken(h.BindToken, r.Header.Get("Authorization"), r.Header.Get(session.HeaderToken)); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type bindRequest struct {
	Code      string `json:"code"`
	ExpiresIn int    `json:"expiresIn"`
	User      struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		Nickname   string `json:"nickname"`
		AvatarData string `json:"avatarData"`
	} `json:"user"`
}

func (h *Handler) bind(w http.ResponseWriter, r *http.Request) {
	var req bindRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(req.Code)
	uid := strings.TrimSpace(req.User.ID)
	if code == "" || uid == "" {
		http.Error(w, "code and user.id required", http.StatusBadRequest)
		return
	}
	avatar, mime, err := store.DecodeAvatarData(req.User.AvatarData)
	if err != nil {
		http.Error(w, "invalid avatarData", http.StatusBadRequest)
		return
	}
	if len(avatar) > store.MaxAvatarBytes {
		http.Error(w, "avatarData too large", http.StatusRequestEntityTooLarge)
		return
	}
	u := store.User{
		ID:         uid,
		Username:   strings.TrimSpace(req.User.Username),
		Nickname:   strings.TrimSpace(req.User.Nickname),
		AvatarData: avatar,
		AvatarMIME: mime,
	}
	if err := h.Store.UpsertUser(u); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ttl := time.Duration(req.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if err := h.Store.PutPair(code, uid, ttl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"user": map[string]string{
			"id":       u.ID,
			"username": u.Username,
			"nickname": u.Nickname,
		},
	})
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(chi.URLParam(r, "userId"))
	u, err := h.Store.GetUser(uid)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	avatarURL := ""
	if len(u.AvatarData) > 0 {
		avatarURL = "/opencode-collab/v1/avatar/" + u.ID
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":        u.ID,
			"username":  u.Username,
			"nickname":  u.Nickname,
			"avatarUrl": avatarURL,
		},
	})
}

func (h *Handler) avatar(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(chi.URLParam(r, "userId"))
	u, err := h.Store.GetUser(uid)
	if errors.Is(err, store.ErrNotFound) || len(u.AvatarData) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	mime := u.AvatarMIME
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=600")
	_, _ = w.Write(u.AvatarData)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
