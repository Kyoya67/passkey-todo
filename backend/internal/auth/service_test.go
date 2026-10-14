package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type fakeAccounts struct {
	calls      int
	err        error
	userID     [16]byte
	credential webauthn.Credential
}

func (f *fakeAccounts) CreateAccount(_ context.Context, id [16]byte, credential webauthn.Credential) error {
	f.calls++
	f.userID = id
	f.credential = credential
	return f.err
}

type fakeRegistrations struct {
	mu                  sync.Mutex
	entries             map[[32]byte]Registration
	saveErr, consumeErr error
}

func (f *fakeRegistrations) Save(_ context.Context, hash [32]byte, registration Registration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.entries[hash] = registration
	return nil
}
func (f *fakeRegistrations) Consume(_ context.Context, hash [32]byte) (Registration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.consumeErr != nil {
		return Registration{}, f.consumeErr
	}
	r, ok := f.entries[hash]
	delete(f.entries, hash)
	if !ok {
		return Registration{}, ErrInvalidRegistration
	}
	return r, nil
}

func newTestService(t *testing.T) (*Service, *fakeAccounts, *fakeRegistrations) {
	t.Helper()
	w, err := NewWebAuthn("localhost", "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	a := &fakeAccounts{}
	r := &fakeRegistrations{entries: make(map[[32]byte]Registration)}
	return NewService(w, a, r), a, r
}

// Synthetic "none" attestation exercises the real library without a physical authenticator.
func registrationResponse(t *testing.T, challenge, origin, rpID string, flags byte) protocol.CredentialCreationResponse {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte(rpID))
	data := append([]byte{}, rpHash[:]...)
	data = append(data, flags, 0, 0, 0, 0)
	data = append(data, make([]byte, 16)...)
	credentialID := make([]byte, 32)
	if _, err := rand.Read(credentialID); err != nil {
		t.Fatal(err)
	}
	data = append(data, 0, byte(len(credentialID)))
	data = append(data, credentialID...)
	data = append(data, cose...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": data, "attStmt": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	client, err := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": challenge, "origin": origin})
	if err != nil {
		t.Fatal(err)
	}
	var response protocol.CredentialCreationResponse
	response.ID = base64.RawURLEncoding.EncodeToString(credentialID)
	response.RawID = credentialID
	response.Type = "public-key"
	response.AttestationResponse.ClientDataJSON = client
	response.AttestationResponse.AttestationObject = attestation
	return response
}

func TestRegistrationService(t *testing.T) {
	for _, mode := range []string{"success", "wrong challenge", "wrong origin", "wrong RP ID", "no user verification", "expired", "wrong user", "zero user", "replayed", "save failed", "consume failed", "account failed"} {
		t.Run(mode, func(t *testing.T) {
			s, accounts, store := newTestService(t)
			failure := errors.New("database unavailable")
			if mode == "save failed" {
				store.saveErr = failure
			}
			token, options, err := s.BeginRegistration(context.Background())
			if mode == "save failed" {
				if !errors.Is(err, failure) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if options.Response.AuthenticatorSelection.ResidentKey != protocol.ResidentKeyRequirementRequired || options.Response.AuthenticatorSelection.UserVerification != protocol.VerificationRequired {
				t.Fatal("discoverable credential and user verification must be required")
			}
			challenge := options.Response.Challenge.String()
			origin, rpID, flags := "http://localhost:3000", "localhost", byte(0x45)
			switch mode {
			case "wrong challenge":
				challenge = "wrong"
			case "wrong origin":
				origin = "https://evil.example"
			case "wrong RP ID":
				rpID = "evil.example"
			case "no user verification":
				flags = 0x41
			case "expired", "wrong user", "zero user":
				for hash, registration := range store.entries {
					if mode == "expired" {
						registration.Session.Expires = time.Now().Add(-time.Second)
					}
					if mode == "wrong user" {
						registration.Session.UserID = []byte("other")
					}
					if mode == "zero user" {
						registration.UserID = [16]byte{}
					}
					store.entries[hash] = registration
				}
			case "consume failed":
				store.consumeErr = failure
			case "account failed":
				accounts.err = failure
			}
			response := registrationResponse(t, challenge, origin, rpID, flags)
			parsed, err := response.Parse()
			if err != nil {
				t.Fatal(err)
			}
			err = s.FinishRegistration(context.Background(), token, parsed)
			switch mode {
			case "success", "replayed":
				if err != nil || accounts.calls != 1 || len(accounts.credential.PublicKey) == 0 {
					t.Fatalf("registration failed: %v, calls %d", err, accounts.calls)
				}
				if mode == "replayed" {
					if err := s.FinishRegistration(context.Background(), token, parsed); !errors.Is(err, ErrInvalidRegistration) {
						t.Fatalf("replay accepted: %v", err)
					}
					if accounts.calls != 1 {
						t.Fatal("replay saved another account")
					}
				}
			case "consume failed", "account failed":
				if !errors.Is(err, failure) {
					t.Fatalf("expected DB error, got %v", err)
				}
			default:
				if !errors.Is(err, ErrInvalidRegistration) || accounts.calls != 0 {
					t.Fatalf("invalid registration saved: %v, calls %d", err, accounts.calls)
				}
				if len(store.entries) != 0 {
					t.Fatal("failed verification did not consume challenge")
				}
			}
		})
	}
}

func TestFinishRegistrationInvalidInput(t *testing.T) {
	s, accounts, _ := newTestService(t)
	for _, token := range []string{"", "!", "YQ"} {
		if err := s.FinishRegistration(context.Background(), token, &protocol.ParsedCredentialCreationData{}); !errors.Is(err, ErrInvalidRegistration) {
			t.Fatalf("got %v", err)
		}
	}
	if err := s.FinishRegistration(context.Background(), base64.RawURLEncoding.EncodeToString(make([]byte, 32)), nil); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("got %v", err)
	}
	if accounts.calls != 0 {
		t.Fatal("invalid input saved")
	}
}

type fakeVerifier struct {
	beginErr   error
	credential *webauthn.Credential
}

func (f fakeVerifier) BeginRegistration(u webauthn.User, _ ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	if f.beginErr != nil {
		return nil, nil, f.beginErr
	}
	return &protocol.CredentialCreation{}, &webauthn.SessionData{UserID: u.WebAuthnID()}, nil
}
func (f fakeVerifier) CreateCredential(webauthn.User, webauthn.SessionData, *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	return f.credential, nil
}

func TestVerifierErrorsAndInvalidCredential(t *testing.T) {
	s, accounts, _ := newTestService(t)
	failure := errors.New("verifier unavailable")
	s.verifier = fakeVerifier{beginErr: failure}
	if _, _, err := s.BeginRegistration(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
	for _, credential := range []*webauthn.Credential{nil, {}, {ID: []byte{1}}, {PublicKey: []byte{1}}} {
		s.verifier = fakeVerifier{credential: credential}
		token, _, err := s.BeginRegistration(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.FinishRegistration(context.Background(), token, &protocol.ParsedCredentialCreationData{}); !errors.Is(err, ErrInvalidRegistration) {
			t.Fatalf("got %v", err)
		}
	}
	if accounts.calls != 0 {
		t.Fatal("invalid credential saved")
	}
}
