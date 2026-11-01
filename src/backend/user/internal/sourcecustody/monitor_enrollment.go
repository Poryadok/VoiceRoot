package sourcecustody

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"github.com/google/uuid"
)

// newDecisionResponder checks the signed opening decision and monitor key.
// It does not establish source identity, exclusive custody or watchdog health;
// an independently qualified caller must check those before using the signer.
func newDecisionResponder(claim OpenAttestation, operator ed25519.PublicKey, pinnedMonitor ed25519.PublicKey, private ed25519.PrivateKey, incarnation uuid.UUID, now time.Time) (*monitorResponder, error) {
	if len(operator) != ed25519.PublicKeySize || len(pinnedMonitor) != ed25519.PublicKeySize || len(private) != ed25519.PrivateKeySize || len(claim.Signature) != ed25519.SignatureSize || now.IsZero() || claim.IssuedAtUnix > now.Unix() || claim.IssuedAtUnix < now.Add(-5*time.Minute).Unix() || claim.ExpiresAtUnix <= now.Unix() || claim.ExpiresAtUnix > now.Add(5*time.Minute).Unix() {
		return nil, ErrUnavailable
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil || !ed25519.Verify(operator, payload, claim.Signature) || subtle.ConstantTimeCompare(operator, pinnedMonitor) == 1 || subtle.ConstantTimeCompare(pinnedMonitor, private[ed25519.SeedSize:]) != 1 {
		return nil, ErrUnavailable
	}
	return newMonitorResponder(claim.Challenge, sha256.Sum256(payload), incarnation, private)
}
