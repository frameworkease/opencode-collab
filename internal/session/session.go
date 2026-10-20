package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName         = "opencode_collab_sid"
	DefaultTTL         = 30 * 24 * time.Hour
	HeaderToken        = "X-Opencode-User-Token"
)

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string) *Manager {
	return &Manager{
		secret: []byte(secret),
		ttl:    DefaultTTL,
	}
}

func (m *Manager) Issue() (sid string, expiresAt time.Time, err error) {
	expiresAt = time.Now().UTC().Add(m.ttl)
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return "", time.Time{}, err
	}
	exp := strconv.FormatInt(expiresAt.Unix(), 10)
	payload := exp + "." + hex.EncodeToString(nonce)
	sig := m.sign(payload)
	sid = payload + "." + base64.RawURLEncoding.EncodeToString(sig)
	return sid, expiresAt, nil
}

func (m *Manager) Verify(sid string) bool {
	sid = strings.TrimSpace(sid)
	parts := strings.Split(sid, ".")
	if len(parts) != 3 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().UTC().Unix() >= exp {
		return false
	}
	payload := parts[0] + "." + parts[1]
	want, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := m.sign(payload)
	return subtle.ConstantTimeCompare(want, got) == 1
}

func (m *Manager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte("opencode-collab-session-v1"))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func CheckBindToken(configured, bearerHeader, altHeader string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return nil
	}
	got := strings.TrimSpace(altHeader)
	if got == "" {
		auth := strings.TrimSpace(bearerHeader)
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			got = strings.TrimSpace(auth[7:])
		}
	}
	if got == "" {
		return fmt.Errorf("unauthorized")
	}
	a := []byte(configured)
	b := []byte(got)
	if len(a) != len(b) || !hmac.Equal(a, b) {
		return fmt.Errorf("unauthorized")
	}
	return nil
}
