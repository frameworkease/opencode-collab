package proxy

import (
	"encoding/json"
	"testing"
)

func TestInjectSenderMetadata(t *testing.T) {
	raw := []byte(`{"text":"hi","metadata":{"displayText":"hi"}}`)
	out, changed := injectSenderMetadata(raw, "u1")
	if !changed {
		t.Fatal("expected change")
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	meta := root["metadata"].(map[string]any)
	if meta["sender"] != "u1" {
		t.Fatalf("sender=%v", meta["sender"])
	}
	if meta["displayText"] != "hi" {
		t.Fatalf("displayText lost")
	}
	out2, changed2 := injectSenderMetadata(out, "u1")
	if changed2 {
		t.Fatalf("idempotent expected, got %s", out2)
	}
}

func TestIsOpenCodePromptPath(t *testing.T) {
	if !isOpenCodePromptPath("/api/session/ses_1/prompt") {
		t.Fatal("expected match")
	}
	if isOpenCodePromptPath("/api/session/ses_1/prompt_async") {
		t.Fatal("prompt_async must not match")
	}
	if isOpenCodePromptPath("/api/session/ses_1/message") {
		t.Fatal("message must not match")
	}
}
