package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func injectSenderMetadata(raw []byte, userID string) ([]byte, bool) {
	userID = strings.TrimSpace(userID)
	if len(raw) == 0 || userID == "" {
		return raw, false
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return raw, false
	}
	meta, _ := root["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	if senderIDFromField(meta["sender"]) == userID {
		if _, ok := root["metadata"].(map[string]any); !ok {
			root["metadata"] = meta
		} else {
			return raw, false
		}
	} else {
		meta["sender"] = userID
		root["metadata"] = meta
	}
	out, err := json.Marshal(root)
	if err != nil {
		return raw, false
	}
	return out, true
}

func senderIDFromField(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if id, _ := t["userId"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
		if id, _ := t["id"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
		return ""
	default:
		return ""
	}
}

func isOpenCodePromptPath(path string) bool {
	const prefix = "/api/session/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "prompt"
}

func maybeEnrichPrompt(r *http.Request, path, userID string) {
	if r == nil || strings.TrimSpace(userID) == "" {
		return
	}
	if strings.ToUpper(strings.TrimSpace(r.Method)) != http.MethodPost {
		return
	}
	if !isOpenCodePromptPath(path) {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		r.ContentLength = 0
		return
	}
	out, changed := injectSenderMetadata(raw, userID)
	if !changed {
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		r.Header.Set("Content-Length", strconv.Itoa(len(raw)))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(out))
	r.ContentLength = int64(len(out))
	r.Header.Set("Content-Length", strconv.Itoa(len(out)))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(out)), nil
	}
}
