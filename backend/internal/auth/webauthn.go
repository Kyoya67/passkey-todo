package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"
)

// NewWebAuthn creates the relying party configuration used by registration and login.
// rpID is a domain without a scheme or port; origin is the browser-facing origin.
func NewWebAuthn(rpID, origin string) (*webauthn.WebAuthn, error) {
	if strings.TrimSpace(rpID) == "" || strings.TrimSpace(origin) == "" {
		return nil, fmt.Errorf("RP ID and origin are required")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() != rpID || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(rpID == "localhost" && parsed.Scheme == "http")) {
		return nil, fmt.Errorf("origin must be an HTTPS origin for the RP ID (HTTP is allowed only for localhost)")
	}
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: "Kyoya67 TODO",
		RPOrigins:     []string{origin},
	})
}
