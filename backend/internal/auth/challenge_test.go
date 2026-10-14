package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	address := os.Getenv("REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Fatalf("ping Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func testTokenHash(t *testing.T) [32]byte {
	t.Helper()
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(token[:])
}

func TestChallengeStoreIntegration(t *testing.T) {
	ctx := context.Background()
	client := testRedis(t)
	store := NewChallengeStore(client)
	id := newTestUUID(t)
	registration := Registration{UserID: id.Bytes, Session: webauthn.SessionData{UserID: id.Bytes[:], Challenge: "challenge", Expires: time.Now().Add(time.Minute)}}
	hash := testTokenHash(t)
	t.Cleanup(func() { _ = client.Del(ctx, registrationKey(hash)).Err() })
	if err := store.Save(ctx, hash, registration); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, hash, registration); err == nil {
		t.Fatal("duplicate token should fail")
	}
	if ttl := client.TTL(ctx, registrationKey(hash)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected TTL: %s", ttl)
	}
	loaded, err := store.Consume(ctx, hash)
	if err != nil || loaded.UserID != registration.UserID || loaded.Session.Challenge != registration.Session.Challenge || !loaded.Session.Expires.Equal(registration.Session.Expires) {
		t.Fatalf("roundtrip failed: %+v, %v", loaded, err)
	}
	if _, err := store.Consume(ctx, hash); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("replay: %v", err)
	}
	registration.Session.Expires = time.Now().Add(-time.Second)
	if err := store.Save(ctx, hash, registration); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("expired registration saved: %v", err)
	}
	registration.Session.Expires = time.Now().Add(50 * time.Millisecond)
	if err := store.Save(ctx, hash, registration); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := store.Consume(ctx, hash); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("expired key returned: %v", err)
	}
	registration.Session.Expires = time.Now().Add(time.Minute)
	if err := store.Save(ctx, hash, registration); err != nil {
		t.Fatal(err)
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Consume(ctx, hash)
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, ErrInvalidRegistration) {
				t.Errorf("consume: %v", err)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("concurrent successes: %d", winners.Load())
	}
	if err := client.Set(ctx, registrationKey(hash), "corrupt", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(ctx, hash); err == nil || errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("expected decoding error, got %v", err)
	}
	closed := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	_ = closed.Close()
	closedStore := NewChallengeStore(closed)
	if err := closedStore.Save(ctx, hash, registration); err == nil {
		t.Fatal("expected storage error")
	}
	if _, err := closedStore.Consume(ctx, hash); err == nil {
		t.Fatal("expected storage error")
	}
}
