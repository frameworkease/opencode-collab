package inject

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// AssetRoutes mounts inject JS (CSP script-src 'self' allows same-origin src).
func AssetRoutes() http.Handler {
	r := chi.NewRouter()
	r.Get("/sse_tee.js", func(w http.ResponseWriter, _ *http.Request) {
		writeJS(w, SessionAvatarSseTeeEarlyJS)
	})
	r.Get("/session_avatar.js", func(w http.ResponseWriter, _ *http.Request) {
		writeJS(w, SessionAvatarBundleJS())
	})
	return r
}

func SessionAvatarBundleJS() string {
	var b strings.Builder
	b.WriteString(`(function(){`)
	b.WriteString(`if(window.__OPENCODE_COLLAB_SESSION_AVATAR__)return;window.__OPENCODE_COLLAB_SESSION_AVATAR__=1;`)
	b.WriteString(`var cfg={};try{var el=document.getElementById("opencode-collab-boot-config");if(el)cfg=JSON.parse(el.textContent||"{}");}catch(e){}`)
	b.WriteString(`var API=cfg.api||"/opencode-collab/v1";`)
	b.WriteString(`var ACTOR=cfg.actor||null;`)
	b.WriteString(`window.__OPENCODE_COLLAB_ACTOR__=ACTOR;`)
	b.WriteString(SessionAvatarRuntimeJS)
	b.WriteString(SessionAvatarPaintRuntimeJS)
	b.WriteString(`})();`)
	return b.String()
}

func writeJS(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(body))
}
