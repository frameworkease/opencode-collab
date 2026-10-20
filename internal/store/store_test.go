package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBindPairSession(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u := User{ID: "u1", Username: "alice", Nickname: "Alice"}
	if err := st.UpsertUser(u); err != nil {
		t.Fatal(err)
	}
	if err := st.PutPair("code1", "u1", time.Minute); err != nil {
		t.Fatal(err)
	}
	p, err := st.GetPair("code1")
	if err != nil || p.UserID != "u1" {
		t.Fatalf("pair: %+v %v", p, err)
	}
	exp := time.Now().UTC().Add(time.Hour)
	if err := st.PutSession("sid1", "u1", exp); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserForSession("sid1")
	if err != nil || got.ID != "u1" || got.Nickname != "Alice" {
		t.Fatalf("user: %+v %v", got, err)
	}
	_ = st.DeletePair("code1")
	if _, err := st.GetPair("code1"); err != ErrNotFound {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestDecodeAvatarDataURL(t *testing.T) {
	data, mime, err := DecodeAvatarData("data:image/png;base64,iVBORw0KGgo=")
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" || len(data) == 0 {
		t.Fatalf("mime=%s len=%d", mime, len(data))
	}
}
