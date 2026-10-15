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
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"sync"
	"testing"
	"time"
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

type fakeLoginVerifier struct {
	lookup     bool
	verifyErr  error
	beginErr   error
	emptyBegin bool
}

func (f *fakeLoginVerifier) BeginDiscoverableLogin(...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	if f.beginErr != nil {
		return nil, nil, f.beginErr
	}
	if f.emptyBegin {
		return nil, nil, nil
	}
	return &protocol.CredentialAssertion{}, &webauthn.SessionData{Challenge: "challenge"}, nil
}

func (f *fakeLoginVerifier) ValidatePasskeyLogin(lookup webauthn.DiscoverableUserHandler, _ webauthn.SessionData, response *protocol.ParsedCredentialAssertionData) (webauthn.User, *webauthn.Credential, error) {
	if f.verifyErr != nil {
		return nil, nil, f.verifyErr
	}
	f.lookup = true
	user, err := lookup(response.RawID, response.Response.UserHandle)
	if err != nil {
		return nil, nil, err
	}
	credential := user.WebAuthnCredentials()[0]
	return user, &credential, nil
}

type fakeLoginAccounts struct {
	id         [16]byte
	credential webauthn.Credential
	findErr    error
	updateErr  error
	updates    int
}

func (f *fakeLoginAccounts) FindCredential(context.Context, []byte) ([16]byte, webauthn.Credential, error) {
	return f.id, f.credential, f.findErr
}
func (f *fakeLoginAccounts) UpdateCredential(context.Context, [16]byte, webauthn.Credential) error {
	f.updates++
	return f.updateErr
}

type fakeLoginChallenges struct {
	sessions map[[32]byte]webauthn.SessionData
	saveErr  error
}

func (f *fakeLoginChallenges) Save(_ context.Context, hash [32]byte, session webauthn.SessionData) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.sessions[hash] = session
	return nil
}
func (f *fakeLoginChallenges) Consume(_ context.Context, hash [32]byte) (webauthn.SessionData, error) {
	session, ok := f.sessions[hash]
	delete(f.sessions, hash)
	if !ok {
		return webauthn.SessionData{}, ErrInvalidLogin
	}
	return session, nil
}

type fakeLoginSessions struct {
	calls       int
	userID      [16]byte
	err         error
	revokeToken string
}

func (f *fakeLoginSessions) Save(_ context.Context, _ [32]byte, id [16]byte) error {
	f.calls++
	f.userID = id
	return f.err
}

func (f *fakeLoginSessions) Revoke(_ context.Context, token string) error {
	f.revokeToken = token
	return f.err
}

func TestLogoutService(t *testing.T) {
	if err := NewService(nil, nil, nil).Logout(context.Background(), "token"); err == nil {
		t.Fatal("missing session store accepted")
	}
	sessions := &fakeLoginSessions{}
	svc := NewService(nil, nil, nil, WithLogin(nil, nil, nil, sessions))
	if err := svc.Logout(context.Background(), "session-token"); err != nil || sessions.revokeToken != "session-token" {
		t.Fatalf("logout token=%q err=%v", sessions.revokeToken, err)
	}
	sessions.err = errors.New("redis failed")
	if err := svc.Logout(context.Background(), "session-token"); !errors.Is(err, sessions.err) {
		t.Fatalf("logout error=%v", err)
	}
}

func TestLoginService(t *testing.T) {
	id := [16]byte{1}
	accounts := &fakeLoginAccounts{id: id, credential: webauthn.Credential{ID: []byte{2}}}
	challenges := &fakeLoginChallenges{sessions: map[[32]byte]webauthn.SessionData{}}
	sessions := &fakeLoginSessions{}
	verifier := &fakeLoginVerifier{}
	svc := NewService(nil, nil, nil, WithLogin(verifier, accounts, challenges, sessions))
	token, _, err := svc.BeginLogin(context.Background())
	if err != nil || len(challenges.sessions) != 1 {
		t.Fatalf("begin: token=%q err=%v", token, err)
	}
	response := &protocol.ParsedCredentialAssertionData{ParsedPublicKeyCredential: protocol.ParsedPublicKeyCredential{RawID: []byte{2}}}
	response.Response.UserHandle = id[:]
	sessionToken, err := svc.FinishLogin(context.Background(), token, response)
	if err != nil || sessionToken == "" || accounts.updates != 1 || sessions.calls != 1 || sessions.userID != id || !verifier.lookup {
		t.Fatalf("finish: token=%q err=%v", sessionToken, err)
	}
	if _, err := svc.FinishLogin(context.Background(), token, response); !errors.Is(err, ErrInvalidLogin) || sessions.calls != 1 {
		t.Fatalf("replayed login accepted: %v", err)
	}
	if _, err := svc.FinishLogin(context.Background(), "invalid", response); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("bad token accepted: %v", err)
	}
}

func TestBeginLoginFailures(t *testing.T) {
	verifier := &fakeLoginVerifier{beginErr: errors.New("verifier unavailable")}
	challenges := &fakeLoginChallenges{sessions: map[[32]byte]webauthn.SessionData{}}
	svc := NewService(nil, nil, nil, WithLogin(verifier, &fakeLoginAccounts{}, challenges, &fakeLoginSessions{}))
	if _, _, err := svc.BeginLogin(context.Background()); !errors.Is(err, verifier.beginErr) {
		t.Fatalf("verifier error: %v", err)
	}
	verifier.beginErr = nil
	verifier.emptyBegin = true
	if _, _, err := svc.BeginLogin(context.Background()); err == nil {
		t.Fatal("empty options accepted")
	}
	verifier.emptyBegin = false
	challenges.saveErr = errors.New("redis unavailable")
	if _, _, err := svc.BeginLogin(context.Background()); !errors.Is(err, challenges.saveErr) {
		t.Fatalf("Redis error: %v", err)
	}
}

func TestLoginFailureDoesNotCreateSession(t *testing.T) {
	for _, mode := range []string{"unknown credential", "database unavailable", "wrong user handle", "bad signature", "update failed", "session failed", "expired"} {
		t.Run(mode, func(t *testing.T) {
			id := [16]byte{1}
			accounts := &fakeLoginAccounts{id: id, credential: webauthn.Credential{ID: []byte{2}}}
			challenges := &fakeLoginChallenges{sessions: map[[32]byte]webauthn.SessionData{}}
			sessions := &fakeLoginSessions{}
			verifier := &fakeLoginVerifier{}
			svc := NewService(nil, nil, nil, WithLogin(verifier, accounts, challenges, sessions))
			token, _, err := svc.BeginLogin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			response := &protocol.ParsedCredentialAssertionData{ParsedPublicKeyCredential: protocol.ParsedPublicKeyCredential{RawID: []byte{2}}}
			response.Response.UserHandle = id[:]
			switch mode {
			case "unknown credential":
				accounts.findErr = pgx.ErrNoRows
			case "database unavailable":
				accounts.findErr = errors.New("database unavailable")
			case "wrong user handle":
				response.Response.UserHandle = []byte{3}
			case "bad signature":
				verifier.verifyErr = errors.New("invalid signature")
			case "update failed":
				accounts.updateErr = errors.New("database unavailable")
			case "session failed":
				sessions.err = errors.New("redis unavailable")
			case "expired":
				for hash, data := range challenges.sessions {
					data.Expires = time.Now().Add(-time.Second)
					challenges.sessions[hash] = data
				}
			}
			got, err := svc.FinishLogin(context.Background(), token, response)
			if err == nil || got != "" {
				t.Fatalf("login accepted: %q %v", got, err)
			}
			if mode != "session failed" && sessions.calls != 0 {
				t.Fatal("session created after failure")
			}
			if mode == "unknown credential" && !errors.Is(err, ErrInvalidLogin) {
				t.Fatalf("wrong error: %v", err)
			}
			if want, ok := map[string]string{
				"unknown credential": "credential_not_found",
				"wrong user handle":  "user_handle_mismatch",
				"bad signature":      "webauthn_verification_failed",
				"expired":            "invalid_or_expired_challenge",
			}[mode]; ok && loginReason(err) != want {
				t.Fatalf("reason %q, want %q", loginReason(err), want)
			}
		})
	}
}

func TestLoginWithRealWebAuthnVerifier(t *testing.T) {
	verifier, err := NewWebAuthn("localhost", "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	id := [16]byte{1, 2, 3}
	credentialID := []byte{4, 5, 6}
	accounts := &fakeLoginAccounts{id: id, credential: webauthn.Credential{ID: credentialID, PublicKey: publicKey}}
	challenges := &fakeLoginChallenges{sessions: map[[32]byte]webauthn.SessionData{}}
	sessions := &fakeLoginSessions{}
	svc := NewService(nil, nil, nil, WithLogin(verifier, accounts, challenges, sessions))
	token, options, err := svc.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clientData, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": options.Response.Challenge.String(), "origin": "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte("localhost"))
	authData := append(append([]byte{}, rpHash[:]...), 0x05, 0, 0, 0, 1)
	clientHash := sha256.Sum256(clientData)
	signed := append(append([]byte{}, authData...), clientHash[:]...)
	signedHash := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, key, signedHash[:])
	if err != nil {
		t.Fatal(err)
	}
	response := protocol.CredentialAssertionResponse{
		PublicKeyCredential: protocol.PublicKeyCredential{Credential: protocol.Credential{ID: "BAUG", Type: "public-key"}, RawID: credentialID},
		AssertionResponse: protocol.AuthenticatorAssertionResponse{
			AuthenticatorResponse: protocol.AuthenticatorResponse{ClientDataJSON: clientData},
			AuthenticatorData:     authData, Signature: signature, UserHandle: id[:],
		},
	}
	parsed, err := response.Parse()
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range challenges.sessions {
		_, _, verifyErr := verifier.ValidatePasskeyLogin(func([]byte, []byte) (webauthn.User, error) {
			return loginUser{id: id, credential: accounts.credential}, nil
		}, session, parsed)
		if verifyErr != nil {
			t.Fatalf("real verifier: %v", verifyErr)
		}
	}
	if _, err := svc.FinishLogin(context.Background(), token, parsed); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if accounts.updates != 1 || sessions.calls != 1 {
		t.Fatal("login did not persist credential and session")
	}
	if _, err := svc.FinishLogin(context.Background(), token, parsed); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("replay accepted: %v", err)
	}
	if accounts.credential.Authenticator.SignCount != 0 {
		t.Fatal("fake account unexpectedly mutated")
	}

	badToken, badOptions, err := svc.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clientData, err = json.Marshal(map[string]any{"type": "webauthn.get", "challenge": badOptions.Response.Challenge.String(), "origin": "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	response.AssertionResponse.ClientDataJSON = clientData
	response.AssertionResponse.Signature = []byte{1, 2, 3}
	badParsed, err := response.Parse()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinishLogin(context.Background(), badToken, badParsed); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("bad signature accepted: %v", err)
	}
}
