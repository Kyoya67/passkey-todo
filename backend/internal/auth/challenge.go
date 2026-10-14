package auth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const registrationKeyPrefix = "webauthn:registration:"

type challengeRedis interface {
	SetNX(context.Context, string, interface{}, time.Duration) *redis.BoolCmd
	GetDel(context.Context, string) *redis.StringCmd
}

// ChallengeStore keeps ceremony state across backend restarts and tasks.
type ChallengeStore struct{ client challengeRedis }

func NewChallengeStore(client challengeRedis) *ChallengeStore {
	return &ChallengeStore{client: client}
}

func registrationKey(tokenHash [32]byte) string {
	return registrationKeyPrefix + hex.EncodeToString(tokenHash[:])
}

func (s *ChallengeStore) Save(ctx context.Context, tokenHash [32]byte, registration Registration) error {
	ttl := time.Until(registration.Session.Expires)
	if ttl <= 0 {
		return ErrInvalidRegistration
	}
	data, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("encode registration: %w", err)
	}
	created, err := s.client.SetNX(ctx, registrationKey(tokenHash), data, ttl).Result()
	if err != nil {
		return fmt.Errorf("save registration: %w", err)
	}
	if !created {
		return fmt.Errorf("registration token already exists")
	}
	return nil
}

func (s *ChallengeStore) Consume(ctx context.Context, tokenHash [32]byte) (Registration, error) {
	data, err := s.client.GetDel(ctx, registrationKey(tokenHash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Registration{}, ErrInvalidRegistration
	}
	if err != nil {
		return Registration{}, fmt.Errorf("consume registration: %w", err)
	}
	var registration Registration
	if err := json.Unmarshal(data, &registration); err != nil {
		return Registration{}, fmt.Errorf("decode registration: %w", err)
	}
	return registration, nil
}
