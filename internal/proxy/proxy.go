package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/frameworkease/opencode-collab/internal/inject"
	"github.com/frameworkease/opencode-collab/internal/session"
	"github.com/frameworkease/opencode-collab/internal/store"
)

type Proxy struct {
	Upstream      *url.URL
	UpstreamBasic string
	Store         *store.Store
	Sessions      *session.Manager
	RP            *httputil.ReverseProxy
	PairRP        *httputil.ReverseProxy // optional; OpenCode pair service
}

type Options struct {
	Upstream      string
	UpstreamPair  string
	UpstreamBasic string
	Store         *store.Store
	Sessions      *session.Manager
}

func New(upstream string, st *store.Store, sessions *session.Manager) (*Proxy, error) {
	return NewWithOptions(Options{Upstream: upstream, Store: st, Sessions: sessions})
}

func NewWithOptions(opt Options) (*Proxy, error) {
	u, err := url.Parse(opt.Upstream)
	if err != nil {
		return nil, err
	}
	p := &Proxy{Upstream: u, UpstreamBasic: strings.TrimSpace(opt.UpstreamBasic), Store: opt.Store, Sessions: opt.Sessions}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.FlushInterval = -1
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		// Preserve browser Host (e.g. 127.0.0.1:4095). OpenCode names the
		// session cookie from Host port and rejects cookies when Origin host
		// != request Host — rewriting to upstream (opencode:4096) breaks SPA auth.
		clientHost := req.Host
		origDirector(req)
		if clientHost != "" {
			req.Host = clientHost
		}
		req.Header.Del("Accept-Encoding") // simplify HTML inject; upstream may still gzip
		if p.UpstreamBasic != "" {
			if user, pass, ok := strings.Cut(p.UpstreamBasic, ":"); ok {
				req.SetBasicAuth(user, pass)
			}
		}
	}
	rp.ModifyResponse = p.modifyResponse
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("upstream error: %v", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}
	p.RP = rp

	if pair := strings.TrimSpace(opt.UpstreamPair); pair != "" {
		pu, err := url.Parse(pair)
		if err != nil {
			return nil, err
		}
		prp := httputil.NewSingleHostReverseProxy(pu)
		prp.FlushInterval = -1
		origPair := prp.Director
		prp.Director = func(req *http.Request) {
			clientHost := req.Host
			origPair(req)
			if clientHost != "" {
				req.Host = clientHost
			}
			req.Header.Del("Accept-Encoding")
		}
		prp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("pair upstream error: %v", err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}
		p.PairRP = prp
	}
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	userID := p.userIDFromRequest(r)

	if strings.HasPrefix(path, "/auth/connect/") {
		p.handleConnect(w, r)
		return
	}

	maybeEnrichPrompt(r, path, userID)
	p.RP.ServeHTTP(w, r)
}

func (p *Proxy) userIDFromRequest(r *http.Request) string {
	c, err := r.Cookie(session.CookieName)
	if err != nil || c == nil || strings.TrimSpace(c.Value) == "" {
		return ""
	}
	if !p.Sessions.Verify(c.Value) {
		return ""
	}
	u, err := p.Store.UserForSession(c.Value)
	if err != nil {
		return ""
	}
	return u.ID
}

func (p *Proxy) actorFromRequest(r *http.Request) *inject.Actor {
	c, err := r.Cookie(session.CookieName)
	if err != nil || c == nil {
		return nil
	}
	if !p.Sessions.Verify(c.Value) {
		return nil
	}
	u, err := p.Store.UserForSession(c.Value)
	if err != nil {
		return nil
	}
	avatarURL := ""
	if len(u.AvatarData) > 0 {
		avatarURL = "/opencode-collab/v1/avatar/" + u.ID
	}
	return &inject.Actor{
		UserID:    u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		AvatarURL: avatarURL,
	}
}

// handleConnect redeems GET /auth/connect/{code}.
//
// Always goes through the OpenCode pair service (OPENCODE_COLLAB_UPSTREAM_PAIR).
// If collab has a binding for the same code and OpenCode accepts it, also set
// opencode_collab_sid so later prompts get metadata.sender.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/auth/connect/")
	code = strings.Trim(code, "/")
	if i := strings.Index(code, "/"); i >= 0 {
		code = code[:i]
	}
	code = strings.TrimSpace(code)

	if p.PairRP == nil {
		http.Error(w, "OPENCODE_COLLAB_UPSTREAM_PAIR is required for /auth/connect", http.StatusBadGateway)
		return
	}

	binding, bindingErr := p.Store.GetBinding(code)

	rec := &captureResponse{header: make(http.Header)}
	p.PairRP.ServeHTTP(rec, r)

	if bindingErr == nil && rec.status >= 200 && rec.status < 400 {
		sid, exp, err := p.Sessions.Issue()
		if err == nil {
			if err := p.Store.PutSession(sid, binding.UserID, exp); err == nil {
				_ = p.Store.DeleteBinding(code)
				http.SetCookie(w, &http.Cookie{
					Name:     session.CookieName,
					Value:    sid,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   int(time.Until(exp).Seconds()),
				})
			}
		}
	}

	for k, vv := range rec.header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	w.WriteHeader(rec.status)
	_, _ = w.Write(rec.body)
}

type captureResponse struct {
	header http.Header
	status int
	body   []byte
}

func (c *captureResponse) Header() http.Header { return c.header }
func (c *captureResponse) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	return len(b), nil
}
func (c *captureResponse) WriteHeader(status int) { c.status = status }

func (p *Proxy) modifyResponse(res *http.Response) error {
	if res == nil || res.Request == nil {
		return nil
	}
	path := res.Request.URL.Path
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/opencode-collab/") {
		return nil
	}
	ct := res.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "text/html") {
		return nil
	}
	body, err := readResponseBody(res)
	if err != nil {
		return err
	}
	actor := p.actorFromRequest(res.Request)
	out := inject.IntoHTML(body, actor)
	res.Body = io.NopCloser(bytes.NewReader(out))
	res.ContentLength = int64(len(out))
	res.Header.Set("Content-Length", strconv.Itoa(len(out)))
	res.Header.Del("Content-Encoding")
	return nil
}

func readResponseBody(res *http.Response) ([]byte, error) {
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	enc := strings.ToLower(res.Header.Get("Content-Encoding"))
	if enc == "gzip" || (len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b) {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return raw, nil
		}
		defer zr.Close()
		decoded, err := io.ReadAll(io.LimitReader(zr, 8<<20))
		if err != nil {
			return raw, nil
		}
		return decoded, nil
	}
	return raw, nil
}
