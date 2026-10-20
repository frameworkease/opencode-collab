package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Listen       string
	Upstream     string
	Database     string
	CookieSecret string
	BindToken    string
}

func Load() (Config, error) {
	cfg := Config{
		Listen:       envOr("OPENCODE_COLLAB_LISTEN", ":4095"),
		Upstream:     strings.TrimSpace(os.Getenv("OPENCODE_COLLAB_UPSTREAM")),
		Database:     envOr("OPENCODE_COLLAB_DATABASE", "./user.db"),
		CookieSecret: strings.TrimSpace(os.Getenv("OPENCODE_COLLAB_COOKIE_SECRET")),
		BindToken:    strings.TrimSpace(os.Getenv("OPENCODE_COLLAB_BIND_TOKEN")),
	}
	if cfg.Listen != "" && !strings.Contains(cfg.Listen, ":") {
		cfg.Listen = ":" + cfg.Listen
	}
	if cfg.Upstream == "" {
		return cfg, fmt.Errorf("OPENCODE_COLLAB_UPSTREAM is required")
	}
	if cfg.CookieSecret == "" {
		return cfg, fmt.Errorf("OPENCODE_COLLAB_COOKIE_SECRET is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
