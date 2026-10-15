package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

const loginKeyPrefix = "webauthn:login:"
const sessionKeyPrefix = "session:"
const sessionLifetime = 24 * time.Hour

var ErrSessionNotFound = errors.New("session not found or expired")

type sessionRedis interface {
	SetNX(context.Context, string, interface{}, time.Duration) *redis.BoolCmd
	Get(context.Context, string) *redis.StringCmd
}

type LoginStore struct{ client challengeRedis }

func NewLoginStore(client challengeRedis) *LoginStore { return &LoginStore{client: client} }

func loginKey(hash [32]byte) string { return loginKeyPrefix + hex.EncodeToString(hash[:]) }

func (s *LoginStore) Save(ctx context.Context, hash [32]byte, session webauthn.SessionData) error {
	ttl := time.Until(session.Expires)
	if ttl <= 0 {
		return ErrInvalidLogin
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode login challenge: %w", err)
	}
	created, err := s.client.SetNX(ctx, loginKey(hash), data, ttl).Result()
	if err != nil {
		return fmt.Errorf("save login challenge: %w", err)
	}
	if !created {
		return fmt.Errorf("login token already exists")
	}
	return nil
}

func (s *LoginStore) Consume(ctx context.Context, hash [32]byte) (webauthn.SessionData, error) {
	data, err := s.client.GetDel(ctx, loginKey(hash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return webauthn.SessionData{}, ErrInvalidLogin
	}
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("consume login challenge: %w", err)
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return webauthn.SessionData{}, fmt.Errorf("decode login challenge: %w", err)
	}
	return session, nil
}

type SessionStore struct{ client sessionRedis }

func NewSessionStore(client sessionRedis) *SessionStore { return &SessionStore{client: client} }

func sessionKey(hash [32]byte) string { return sessionKeyPrefix + hex.EncodeToString(hash[:]) }

func (s *SessionStore) Save(ctx context.Context, hash [32]byte, userID [16]byte) error {
	created, err := s.client.SetNX(ctx, sessionKey(hash), userID[:], sessionLifetime).Result()
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if !created {
		return fmt.Errorf("session token already exists")
	}
	return nil
}

// Load validates the opaque cookie and returns the user ID stored in Redis.
func (s *SessionStore) Load(ctx context.Context, token string) ([16]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return [16]byte{}, ErrSessionNotFound
	}
	hash := sha256.Sum256(raw)
	data, err := s.client.Get(ctx, sessionKey(hash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return [16]byte{}, ErrSessionNotFound
	}
	if err != nil {
		return [16]byte{}, fmt.Errorf("load session: %w", err)
	}
	if len(data) != 16 {
		return [16]byte{}, ErrSessionNotFound
	}
	var userID [16]byte
	copy(userID[:], data)
	return userID, nil
}
