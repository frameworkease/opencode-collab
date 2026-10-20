package inject

import (
	"strings"
	"testing"
)

func TestIntoHTML(t *testing.T) {
	html := []byte(`<!doctype html><html><head><title>t</title></head><body><div>ok</div></body></html>`)
	out := IntoHTML(html, &Actor{UserID: "u1", Username: "a", Nickname: "A"})
	s := string(out)
	if !strings.Contains(s, `id="opencode-collab-session-avatar-style"`) {
		t.Fatal("missing style")
	}
	if !strings.Contains(s, `/opencode-collab/assets/sse_tee.js`) {
		t.Fatal("missing sse tee src")
	}
	if !strings.Contains(s, `/opencode-collab/assets/session_avatar.js`) {
		t.Fatal("missing boot src")
	}
	if !strings.Contains(s, `id="opencode-collab-boot-config"`) {
		t.Fatal("missing boot config")
	}
	if !strings.Contains(s, `/opencode-collab/v1`) {
		t.Fatal("missing api base")
	}
	if !strings.Contains(s, `"userId":"u1"`) {
		t.Fatal("missing actor")
	}
	if strings.Contains(s, "opencode-collab-deleted") {
		t.Fatal("deleted-user CSS should be gone")
	}
	out2 := IntoHTML(out, nil)
	if string(out2) != s {
		t.Fatal("second inject should be no-op")
	}
}

func TestIntoHTMLNilActor(t *testing.T) {
	html := []byte(`<html><head></head><body></body></html>`)
	s := string(IntoHTML(html, nil))
	if !strings.Contains(s, `/opencode-collab/assets/session_avatar.js`) {
		t.Fatal("boot required even without actor")
	}
	if !strings.Contains(s, `"actor":null`) {
		t.Fatal("actor should be null")
	}
}

func TestSessionAvatarBundleJS(t *testing.T) {
	js := SessionAvatarBundleJS()
	if !strings.Contains(js, "senderIdFromRecord") {
		t.Fatal("bundle missing sender logic")
	}
	if !strings.Contains(js, "opencode-collab-boot-config") {
		t.Fatal("bundle must read boot config")
	}
}
