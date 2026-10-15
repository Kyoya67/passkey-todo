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
	"github.com/jackc/pgx/v5"
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
	loginVerifier loginVerifier
	loginAccounts loginAccounts
	challenges    loginChallenges
	sessions      loginSessions
}

type ServiceOption func(*Service)

func WithLogin(verifier loginVerifier, accounts loginAccounts, challenges loginChallenges, sessions loginSessions) ServiceOption {
	return func(s *Service) {
		s.loginVerifier = verifier
		s.loginAccounts = accounts
		s.challenges = challenges
		s.sessions = sessions
	}
}

func NewService(verifier registrationVerifier, accounts accountRepository, registrations registrationStore, options ...ServiceOption) *Service {
	s := &Service{verifier: verifier, accounts: accounts, registrations: registrations}
	for _, option := range options {
		option(s)
	}
	return s
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

var ErrInvalidLogin = errors.New("invalid or expired login")

type loginVerifier interface {
	BeginDiscoverableLogin(...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
	ValidatePasskeyLogin(webauthn.DiscoverableUserHandler, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (webauthn.User, *webauthn.Credential, error)
}

type loginAccounts interface {
	FindCredential(context.Context, []byte) ([16]byte, webauthn.Credential, error)
	UpdateCredential(context.Context, [16]byte, webauthn.Credential) error
}

type loginChallenges interface {
	Save(context.Context, [32]byte, webauthn.SessionData) error
	Consume(context.Context, [32]byte) (webauthn.SessionData, error)
}

type loginSessions interface {
	Save(context.Context, [32]byte, [16]byte) error
}

func (s *Service) BeginLogin(ctx context.Context) (string, *protocol.CredentialAssertion, error) {
	options, session, err := s.loginVerifier.BeginDiscoverableLogin()
	if err != nil {
		return "", nil, fmt.Errorf("begin login: %w", err)
	}
	if options == nil || session == nil {
		return "", nil, errors.New("missing login options or session")
	}
	session.Expires = time.Now().Add(registrationLifetime)
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", nil, fmt.Errorf("generate login token: %w", err)
	}
	if err := s.challenges.Save(ctx, sha256.Sum256(token[:]), *session); err != nil {
		return "", nil, fmt.Errorf("save login challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), options, nil
}

func (s *Service) FinishLogin(ctx context.Context, token string, response *protocol.ParsedCredentialAssertionData) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 || response == nil {
		return "", ErrInvalidLogin
	}
	session, err := s.challenges.Consume(ctx, sha256.Sum256(decoded))
	if err != nil {
		return "", fmt.Errorf("load login challenge: %w", err)
	}
	if !session.Expires.After(time.Now()) || len(session.UserID) != 0 {
		return "", ErrInvalidLogin
	}
	var lookupErr error
	lookup := func(rawID, userHandle []byte) (webauthn.User, error) {
		id, credential, err := s.loginAccounts.FindCredential(ctx, rawID)
		if err != nil {
			lookupErr = err
			return nil, err
		}
		if !bytes.Equal(id[:], userHandle) {
			return nil, ErrInvalidLogin
		}
		return loginUser{id: id, credential: credential}, nil
	}
	user, credential, err := s.loginVerifier.ValidatePasskeyLogin(lookup, session, response)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return "", fmt.Errorf("lookup login credential: %w", lookupErr)
	}
	if err != nil || user == nil || credential == nil {
		return "", ErrInvalidLogin
	}
	userID := user.WebAuthnID()
	if len(userID) != 16 || !bytes.Equal(credential.ID, response.RawID) {
		return "", ErrInvalidLogin
	}
	var id [16]byte
	copy(id[:], userID)
	if err := s.loginAccounts.UpdateCredential(ctx, id, *credential); err != nil {
		return "", fmt.Errorf("update login credential: %w", err)
	}
	var sessionToken [32]byte
	if _, err := rand.Read(sessionToken[:]); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	if err := s.sessions.Save(ctx, sha256.Sum256(sessionToken[:]), id); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(sessionToken[:]), nil
}

type loginUser struct {
	id         [16]byte
	credential webauthn.Credential
}

func (u loginUser) WebAuthnID() []byte          { return u.id[:] }
func (u loginUser) WebAuthnName() string        { return fmt.Sprintf("todo-%x", u.id[:]) }
func (u loginUser) WebAuthnDisplayName() string { return u.WebAuthnName() }
func (u loginUser) WebAuthnCredentials() []webauthn.Credential {
	return []webauthn.Credential{u.credential}
}
