package store

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const MaxAvatarBytes = 512 << 10

var (
	ErrNotFound = errors.New("not found")
	ErrExpired  = errors.New("expired")
)

type User struct {
	ID         string
	Username   string
	Nickname   string
	AvatarData []byte
	AvatarMIME string
	CreatedAt  time.Time
}

type Pair struct {
	Code      string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Session struct {
	SID       string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  avatar_data BLOB,
  avatar_mime TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS pairs (
  code TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  sid TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pairs_user ON pairs(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`)
	return err
}

func (s *Store) UpsertUser(u User) error {
	u.ID = strings.TrimSpace(u.ID)
	if u.ID == "" {
		return fmt.Errorf("user id required")
	}
	if len(u.AvatarData) > MaxAvatarBytes {
		return fmt.Errorf("avatar_data exceeds %d bytes", MaxAvatarBytes)
	}
	now := time.Now().UTC().Unix()
	_, err := s.db.Exec(`
INSERT INTO users (id, username, nickname, avatar_data, avatar_mime, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  username=excluded.username,
  nickname=excluded.nickname,
  avatar_data=excluded.avatar_data,
  avatar_mime=excluded.avatar_mime
`, u.ID, u.Username, u.Nickname, u.AvatarData, u.AvatarMIME, now)
	return err
}

func (s *Store) GetUser(id string) (User, error) {
	id = strings.TrimSpace(id)
	var u User
	var created int64
	var avatar []byte
	err := s.db.QueryRow(`
SELECT id, username, nickname, avatar_data, avatar_mime, created_at
FROM users WHERE id=?`, id).Scan(
		&u.ID, &u.Username, &u.Nickname, &avatar, &u.AvatarMIME, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.AvatarData = avatar
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, nil
}

func (s *Store) PutPair(code, userID string, expiresIn time.Duration) error {
	code = strings.TrimSpace(code)
	userID = strings.TrimSpace(userID)
	if code == "" || userID == "" {
		return fmt.Errorf("code and user_id required")
	}
	if expiresIn <= 0 {
		expiresIn = 5 * time.Minute
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(`
INSERT INTO pairs (code, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(code) DO UPDATE SET
  user_id=excluded.user_id,
  expires_at=excluded.expires_at
`, code, userID, now.Add(expiresIn).Unix(), now.Unix())
	return err
}

func (s *Store) GetPair(code string) (Pair, error) {
	code = strings.TrimSpace(code)
	var p Pair
	var exp, created int64
	err := s.db.QueryRow(`
SELECT code, user_id, expires_at, created_at FROM pairs WHERE code=?`, code).
		Scan(&p.Code, &p.UserID, &exp, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Pair{}, ErrNotFound
	}
	if err != nil {
		return Pair{}, err
	}
	p.ExpiresAt = time.Unix(exp, 0).UTC()
	p.CreatedAt = time.Unix(created, 0).UTC()
	if time.Now().UTC().After(p.ExpiresAt) {
		_, _ = s.db.Exec(`DELETE FROM pairs WHERE code=?`, code)
		return Pair{}, ErrExpired
	}
	return p, nil
}

func (s *Store) DeletePair(code string) error {
	_, err := s.db.Exec(`DELETE FROM pairs WHERE code=?`, strings.TrimSpace(code))
	return err
}

func (s *Store) PutSession(sid, userID string, expiresAt time.Time) error {
	sid = strings.TrimSpace(sid)
	userID = strings.TrimSpace(userID)
	if sid == "" || userID == "" {
		return fmt.Errorf("sid and user_id required")
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(`
INSERT INTO sessions (sid, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(sid) DO UPDATE SET
  user_id=excluded.user_id,
  expires_at=excluded.expires_at
`, sid, userID, expiresAt.Unix(), now.Unix())
	return err
}

func (s *Store) GetSession(sid string) (Session, error) {
	sid = strings.TrimSpace(sid)
	var sess Session
	var exp, created int64
	err := s.db.QueryRow(`
SELECT sid, user_id, expires_at, created_at FROM sessions WHERE sid=?`, sid).
		Scan(&sess.SID, &sess.UserID, &exp, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	sess.ExpiresAt = time.Unix(exp, 0).UTC()
	sess.CreatedAt = time.Unix(created, 0).UTC()
	if time.Now().UTC().After(sess.ExpiresAt) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE sid=?`, sid)
		return Session{}, ErrExpired
	}
	return sess, nil
}

func (s *Store) UserForSession(sid string) (User, error) {
	sess, err := s.GetSession(sid)
	if err != nil {
		return User{}, err
	}
	return s.GetUser(sess.UserID)
}

// DecodeAvatarData accepts raw bytes, data URLs, or base64.
func DecodeAvatarData(raw string) (data []byte, mime string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid data URL")
		}
		meta := raw[5:comma]
		payload := raw[comma+1:]
		mime = "application/octet-stream"
		if i := strings.Index(meta, ";"); i >= 0 {
			mime = meta[:i]
		} else if meta != "" {
			mime = meta
		}
		if strings.Contains(meta, ";base64") {
			data, err = base64.StdEncoding.DecodeString(payload)
			return data, mime, err
		}
		return []byte(payload), mime, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) > 0 {
		return decoded, sniffMIME(decoded), nil
	}
	b := []byte(raw)
	return b, sniffMIME(b), nil
}

func sniffMIME(b []byte) string {
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return "image/jpeg"
	}
	if len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return "image/png"
	}
	if len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "application/octet-stream"
}
