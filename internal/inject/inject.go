package inject

import (
	"bytes"
	"encoding/json"
	"strings"
)

const apiBase = "/opencode-collab/v1"

// IntoHTML injects avatar CSS + external scripts (OpenCode CSP blocks inline JS).
func IntoHTML(html []byte, actor *Actor) []byte {
	if bytes.Contains(html, []byte(`id="opencode-collab-boot-config"`)) {
		return html
	}

	head := []struct{ marker, snippet string }{
		{`id="opencode-collab-session-avatar-style"`, headStyle()},
		{`id="opencode-collab-session-avatar-sse-tee"`, `<script id="opencode-collab-session-avatar-sse-tee" src="/opencode-collab/assets/sse_tee.js"></script>`},
	}
	// injectIntoHeadOnce prepends after <head>, so call in reverse for document order style → tee.
	for i := len(head) - 1; i >= 0; i-- {
		html = injectIntoHeadOnce(html, head[i].marker, []byte(head[i].snippet))
	}

	boot := bootTags(actor)
	lower := bytes.ToLower(html)
	if i := bytes.LastIndex(lower, []byte("</body>")); i >= 0 {
		out := make([]byte, 0, len(html)+len(boot))
		out = append(out, html[:i]...)
		out = append(out, boot...)
		out = append(out, html[i:]...)
		return out
	}
	return append(html, boot...)
}

func injectIntoHeadOnce(html []byte, marker string, snippet []byte) []byte {
	if marker != "" && bytes.Contains(html, []byte(marker)) {
		return html
	}
	lower := bytes.ToLower(html)
	idx := bytes.Index(lower, []byte("<head"))
	if idx < 0 {
		out := make([]byte, 0, len(snippet)+len(html))
		out = append(out, snippet...)
		out = append(out, html...)
		return out
	}
	gt := bytes.IndexByte(html[idx:], '>')
	if gt < 0 {
		return html
	}
	insert := idx + gt + 1
	out := make([]byte, 0, len(html)+len(snippet))
	out = append(out, html[:insert]...)
	out = append(out, snippet...)
	out = append(out, html[insert:]...)
	return out
}

func headStyle() string {
	return `<style id="opencode-collab-session-avatar-style">` +
		`:root{--opencode-collab-avatar-ring:0 0 0 1.2px color-mix(in srgb,#1b1b1b 20%,transparent);` +
		`--opencode-collab-avatar-face-inset:inset 0 0 0 1px color-mix(in srgb,#1b1b1b 12%,transparent);` +
		`--opencode-collab-avatar-face-bg:#f7f7f7}` +
		`html[data-color-scheme="dark"]{` +
		`--opencode-collab-avatar-ring:0 0 0 1.2px color-mix(in srgb,#f5f5f5 20%,transparent);` +
		`--opencode-collab-avatar-face-inset:inset 0 0 0 1px color-mix(in srgb,#f5f5f5 12%,transparent);` +
		`--opencode-collab-avatar-face-bg:#141414}` +
		`[data-timeline-row="UserMessage"],` +
		`[data-component="user-message"],` +
		`[data-component="user-message"] [data-slot="user-message-body"],` +
		`[data-component="user-message"] [data-slot="user-message-attachments"]{` +
		`overflow:visible!important;` +
		`}` +
		`[data-timeline-row="UserMessage"][data-opencode-collab-dup-user]{display:none!important;}` +
		`[data-component="user-message"] [data-slot="user-message-body"],` +
		`[data-component="user-message"]:not(:has([data-slot="user-message-body"])) [data-slot="user-message-attachments"]{` +
		`position:relative!important;` +
		`padding-inline-end:calc(2rem + 4px)!important;` +
		`}` +
		`[data-component="user-message"] [data-slot="user-message-body"]::after,` +
		`[data-component="user-message"]:not(:has([data-slot="user-message-body"])) [data-slot="user-message-attachments"]::after{` +
		`content:""!important;position:absolute!important;right:0!important;top:2px!important;` +
		`width:2rem!important;height:2rem!important;border-radius:50%!important;` +
		`box-sizing:border-box!important;` +
		`background-color:var(--opencode-collab-avatar-face-bg)!important;` +
		`box-shadow:var(--opencode-collab-avatar-ring),var(--opencode-collab-avatar-face-inset)!important;` +
		`pointer-events:none!important;z-index:0!important;` +
		`}` +
		`[data-component="user-message"] [data-slot="user-message-body"]:has(>[data-opencode-collab-avatar-ready])::after,` +
		`[data-component="user-message"] [data-slot="user-message-attachments"]:has(>[data-opencode-collab-avatar-ready])::after{` +
		`content:none!important;display:none!important;` +
		`}` +
		`[data-timeline-row="UserMessage"] [data-opencode-collab-avatar-slot],` +
		`[data-timeline-row="UserMessage"] [data-opencode-collab-avatar-applied]{` +
		`display:flex!important;align-items:center!important;justify-content:center!important;` +
		`position:absolute!important;right:0!important;top:2px!important;z-index:1!important;` +
		`border:none!important;box-sizing:border-box!important;overflow:visible!important;` +
		`padding:0!important;margin:0!important;flex-shrink:0!important;border-radius:50%!important;` +
		`width:2rem!important;height:2rem!important;color:transparent!important;` +
		`-webkit-text-fill-color:transparent!important;font-size:0!important;` +
		`opacity:0!important;transition:opacity .4s ease!important;` +
		`background-color:var(--opencode-collab-avatar-face-bg)!important;` +
		`background-size:cover!important;background-position:center!important;background-repeat:no-repeat!important;` +
		`box-shadow:var(--opencode-collab-avatar-ring),var(--opencode-collab-avatar-face-inset)!important;` +
		`}` +
		`[data-timeline-row="UserMessage"] [data-opencode-collab-avatar-ready]{opacity:1!important;}` +
		`@media (prefers-reduced-motion:reduce){` +
		`[data-timeline-row="UserMessage"] [data-opencode-collab-avatar-slot],` +
		`[data-timeline-row="UserMessage"] [data-opencode-collab-avatar-applied]{transition:none!important;}` +
		`}` +
		`.opencode-collab-avatar-tip{` +
		`position:fixed;z-index:2147483000;pointer-events:none;` +
		`min-width:0;max-width:min(240px,70vw);padding:6px 8px;border-radius:8px;` +
		`background:#f7f7f7;color:#1b1b1b;` +
		`border:1px solid color-mix(in srgb,#666 35%,#e8e8e8);` +
		`box-shadow:0 8px 24px rgba(0,0,0,.14);` +
		`font:500 13px/1.25 ui-sans-serif,system-ui,-apple-system,sans-serif;` +
		`}` +
		`.opencode-collab-avatar-tip .opencode-collab-avatar-tip__name{` +
		`display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;` +
		`}` +
		`.opencode-collab-avatar-tip .opencode-collab-avatar-tip__user{` +
		`display:block;margin-top:2px;font-size:11px;font-weight:400;color:#666;` +
		`white-space:nowrap;overflow:hidden;text-overflow:ellipsis;` +
		`}` +
		`html[data-color-scheme="dark"] .opencode-collab-avatar-tip{` +
		`background:#141414;color:#f5f5f5;border-color:#2a2a2a;` +
		`box-shadow:0 8px 24px rgba(0,0,0,.45)}` +
		`html[data-color-scheme="dark"] .opencode-collab-avatar-tip .opencode-collab-avatar-tip__user{color:#a3a3a3}` +
		`</style>`
}

func bootTags(actor *Actor) []byte {
	cfg := map[string]any{
		"api":   apiBase,
		"actor": actor,
	}
	if actor == nil || strings.TrimSpace(actor.UserID) == "" {
		cfg["actor"] = nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		raw = []byte(`{"api":"/opencode-collab/v1","actor":null}`)
	}
	var b bytes.Buffer
	b.WriteString(`<script type="application/json" id="opencode-collab-boot-config">`)
	b.Write(raw)
	b.WriteString(`</script>`)
	b.WriteString(`<script id="opencode-collab-session-avatar" src="/opencode-collab/assets/session_avatar.js"></script>`)
	return b.Bytes()
}
