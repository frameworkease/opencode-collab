package main

import (
	"log"
	"net/http"
	"os"

	"github.com/frameworkease/opencode-collab/internal/api"
	"github.com/frameworkease/opencode-collab/internal/config"
	"github.com/frameworkease/opencode-collab/internal/proxy"
	"github.com/frameworkease/opencode-collab/internal/session"
	"github.com/frameworkease/opencode-collab/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	sessions := session.New(cfg.CookieSecret)
	px, err := proxy.NewWithOptions(proxy.Options{
		Upstream:      cfg.Upstream,
		UpstreamPair:  cfg.UpstreamPair,
		UpstreamBasic: cfg.UpstreamBasic,
		Store:         st,
		Sessions:      sessions,
	})
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}

	apiH := &api.Handler{Store: st, BindToken: cfg.BindToken}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Mount("/opencode-collab", apiH.Routes())
	// Everything else → OpenCode reverse proxy (incl. "/", SSE, redeem).
	r.NotFound(px.ServeHTTP)
	r.MethodNotAllowed(px.ServeHTTP)

	log.Printf("opencode-collab listening on %s → %s (db=%s)", cfg.Listen, cfg.Upstream, cfg.Database)
	if err := http.ListenAndServe(cfg.Listen, r); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
