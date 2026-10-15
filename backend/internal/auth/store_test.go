package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

type fakeChallengeRedis struct {
	values map[string]string
	ttl    map[string]time.Duration
}

func (f *fakeChallengeRedis) SetNX(_ context.Context, key string, value interface{}, ttl time.Duration) *redis.BoolCmd {
	if _, found := f.values[key]; found {
		return redis.NewBoolResult(false, nil)
	}
	switch v := value.(type) {
	case []byte:
		f.values[key] = string(v)
	case string:
		f.values[key] = v
	default:
		return redis.NewBoolResult(false, errors.New("unexpected value"))
	}
	f.ttl[key] = ttl
	return redis.NewBoolResult(true, nil)
}

func (f *fakeChallengeRedis) GetDel(_ context.Context, key string) *redis.StringCmd {
	v, found := f.values[key]
	delete(f.values, key)
	if !found {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(v, nil)
}

func (f *fakeChallengeRedis) Get(_ context.Context, key string) *redis.StringCmd {
	value, found := f.values[key]
	if !found {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(value, nil)
}

func TestLoginStores(t *testing.T) {
	ctx := context.Background()
	client := &fakeChallengeRedis{values: map[string]string{}, ttl: map[string]time.Duration{}}
	login := NewLoginStore(client)
	hash := testTokenHash(t)
	data := webauthn.SessionData{Challenge: "challenge", Expires: time.Now().Add(time.Minute)}
	if err := login.Save(ctx, hash, data); err != nil {
		t.Fatal(err)
	}
	if ttl := client.ttl[loginKey(hash)]; ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL %v", ttl)
	}
	if err := login.Save(ctx, hash, data); err == nil {
		t.Fatal("duplicate accepted")
	}
	got, err := login.Consume(ctx, hash)
	if err != nil || got.Challenge != data.Challenge {
		t.Fatalf("challenge: %+v, %v", got, err)
	}
	if _, err := login.Consume(ctx, hash); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("replay: %v", err)
	}
	data.Expires = time.Now().Add(-time.Second)
	if err := login.Save(ctx, hash, data); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("expired: %v", err)
	}

	sessions := NewSessionStore(client)
	userID := [16]byte{1, 2, 3}
	if err := sessions.Save(ctx, hash, userID); err != nil {
		t.Fatal(err)
	}
	if client.values[sessionKey(hash)] != string(userID[:]) || client.ttl[sessionKey(hash)] != sessionLifetime {
		t.Fatal("wrong session")
	}
	if err := sessions.Save(ctx, hash, userID); err == nil {
		t.Fatal("duplicate session accepted")
	}
	raw := [32]byte{9, 8, 7}
	sessionHash := sha256.Sum256(raw[:])
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	if err := sessions.Save(ctx, sessionHash, userID); err != nil {
		t.Fatal(err)
	}
	if got, err := sessions.Load(ctx, token); err != nil || got != userID {
		t.Fatalf("load: id=%v err=%v", got, err)
	}
	for _, invalid := range []string{"", "!", "YQ"} {
		if _, err := sessions.Load(ctx, invalid); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("invalid token %q: %v", invalid, err)
		}
	}
	delete(client.values, sessionKey(sessionHash))
	if _, err := sessions.Load(ctx, token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("missing session: %v", err)
	}
	client.values[sessionKey(sessionHash)] = "bad"
	if _, err := sessions.Load(ctx, token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("corrupt session: %v", err)
	}
}

func TestLoginStoresIntegration(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	loginStore := NewLoginStore(client)
	hash := testTokenHash(t)
	t.Cleanup(func() { _ = client.Del(ctx, loginKey(hash)).Err() })
	challenge := webauthn.SessionData{Challenge: "login-challenge", Expires: time.Now().Add(time.Minute)}
	if err := loginStore.Save(ctx, hash, challenge); err != nil {
		t.Fatal(err)
	}
	if err := loginStore.Save(ctx, hash, challenge); err == nil {
		t.Fatal("duplicate challenge accepted")
	}
	if ttl := client.TTL(ctx, loginKey(hash)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL: %v", ttl)
	}
	loaded, err := loginStore.Consume(ctx, hash)
	if err != nil || loaded.Challenge != challenge.Challenge {
		t.Fatalf("challenge: %+v %v", loaded, err)
	}
	if _, err := loginStore.Consume(ctx, hash); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("replay: %v", err)
	}

	sessionStore := NewSessionStore(client)
	sessionHash := testTokenHash(t)
	t.Cleanup(func() { _ = client.Del(ctx, sessionKey(sessionHash)).Err() })
	id := [16]byte{1, 2, 3}
	if err := sessionStore.Save(ctx, sessionHash, id); err != nil {
		t.Fatal(err)
	}
	if err := sessionStore.Save(ctx, sessionHash, id); err == nil {
		t.Fatal("duplicate session accepted")
	}
	if got := client.Get(ctx, sessionKey(sessionHash)).Val(); got != string(id[:]) {
		t.Fatal("wrong session owner")
	}
	if ttl := client.TTL(ctx, sessionKey(sessionHash)).Val(); ttl <= 0 || ttl > sessionLifetime {
		t.Fatalf("session TTL: %v", ttl)
	}
	raw := [32]byte{6, 5, 4}
	lookupHash := sha256.Sum256(raw[:])
	lookupToken := base64.RawURLEncoding.EncodeToString(raw[:])
	t.Cleanup(func() { _ = client.Del(ctx, sessionKey(lookupHash)).Err() })
	if err := sessionStore.Save(ctx, lookupHash, id); err != nil {
		t.Fatal(err)
	}
	if got, err := sessionStore.Load(ctx, lookupToken); err != nil || got != id {
		t.Fatalf("load: id=%v err=%v", got, err)
	}
}
