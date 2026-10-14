package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var ErrInvalidRegistration = errors.New("invalid or expired registration")

const registrationLifetime = 5 * time.Minute

type registrationVerifier interface {
	BeginRegistration(webauthn.User, ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error)
	CreateCredential(webauthn.User, webauthn.SessionData, *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error)
}

type accountRepository interface {
	CreateAccount(context.Context, [16]byte, webauthn.Credential) error
}

type registrationStore interface {
	Save(context.Context, [32]byte, Registration) error
	Consume(context.Context, [32]byte) (Registration, error)
}

// Registration is server-side ceremony state, never accepted from a client.
type Registration struct {
	UserID  [16]byte
	Session webauthn.SessionData
}

type Service struct {
	verifier      registrationVerifier
	accounts      accountRepository
	registrations registrationStore
}

func NewService(verifier registrationVerifier, accounts accountRepository, registrations registrationStore) *Service {
	return &Service{verifier: verifier, accounts: accounts, registrations: registrations}
}

func (s *Service) BeginRegistration(ctx context.Context) (string, *protocol.CredentialCreation, error) {
	var user registrationUser
	if _, err := rand.Read(user[:]); err != nil {
		return "", nil, fmt.Errorf("generate user ID: %w", err)
	}
	user[6] = (user[6] & 0x0f) | 0x40
	user[8] = (user[8] & 0x3f) | 0x80
	options, session, err := s.verifier.BeginRegistration(user)
	if err != nil {
		return "", nil, fmt.Errorf("begin registration: %w", err)
	}
	if options == nil || session == nil {
		return "", nil, errors.New("missing registration options or session")
	}
	session.Expires = time.Now().Add(registrationLifetime)
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", nil, fmt.Errorf("generate registration token: %w", err)
	}
	if err := s.registrations.Save(ctx, sha256.Sum256(token[:]), Registration{UserID: user, Session: *session}); err != nil {
		return "", nil, fmt.Errorf("save registration: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), options, nil
}

func (s *Service) FinishRegistration(ctx context.Context, token string, response *protocol.ParsedCredentialCreationData) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 || response == nil {
		return ErrInvalidRegistration
	}
	// Redis GETDEL makes the token single-use, including failed attempts.
	registration, err := s.registrations.Consume(ctx, sha256.Sum256(decoded))
	if err != nil {
		return fmt.Errorf("load registration: %w", err)
	}
	if registration.UserID == [16]byte{} || !registration.Session.Expires.After(time.Now()) ||
		!bytes.Equal(registration.UserID[:], registration.Session.UserID) {
		return ErrInvalidRegistration
	}
	credential, err := s.verifier.CreateCredential(registrationUser(registration.UserID), registration.Session, response)
	if err != nil {
		return ErrInvalidRegistration
	}
	if credential == nil || len(credential.ID) == 0 || len(credential.PublicKey) == 0 {
		return ErrInvalidRegistration
	}
	if err := s.accounts.CreateAccount(ctx, registration.UserID, *credential); err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	return nil
}

type registrationUser [16]byte

func (u registrationUser) WebAuthnID() []byte                         { return u[:] }
func (u registrationUser) WebAuthnName() string                       { return fmt.Sprintf("todo-%x", u[:]) }
func (u registrationUser) WebAuthnDisplayName() string                { return u.WebAuthnName() }
func (u registrationUser) WebAuthnCredentials() []webauthn.Credential { return nil }
