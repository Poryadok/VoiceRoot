package gameprotocol

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const executionPermitType = "voice.game-message-execution-permit+jwt"

type ExecutionPermitExpected struct {
	Device        DeviceAuthority
	OperationID   uuid.UUID
	RequestSHA256 string
	MutationBytes []byte
}

type ExecutionPermit struct {
	ID                uuid.UUID
	OperationID       uuid.UUID
	RequestSHA256     string
	ApplicationID     uuid.UUID
	EnvironmentID     uuid.UUID
	AccountID         uuid.UUID
	ActorID           uuid.UUID
	BindingID         uuid.UUID
	ProfileID         uuid.UUID
	DeviceID          uuid.UUID
	KeyID             uuid.UUID
	DeviceGeneration  int64
	AuthorityRevision int64
	GISPermitID       uuid.UUID
	BindingRevision   int64
	AssertionJTI      uuid.UUID
	IssuedAt          time.Time
	ExpiresAt         time.Time
	Scope             string
}

// VerifyExecutionPermit verifies Auth's one-use permit against the exact
// canonical JCS mutation payload and the already-verified device assertion.
func VerifyExecutionPermit(compact string, authKeys map[string]*rsa.PublicKey, now time.Time, clockUncertainty time.Duration, expected ExecutionPermitExpected) (ExecutionPermit, error) {
	var result ExecutionPermit
	if compact == "" || len(compact) > 16*1024 || expected.OperationID == uuid.Nil || expected.Device.AssertionJTI == uuid.Nil || expected.MutationBytes == nil {
		return result, errors.New("invalid execution permit input")
	}
	if clockUncertainty < 0 || clockUncertainty > 250*time.Millisecond {
		return result, errors.New("execution permit clock uncertainty exceeds limit")
	}
	digest := sha256.Sum256(expected.MutationBytes)
	digestHex := hex.EncodeToString(digest[:])
	if expected.RequestSHA256 != digestHex {
		return result, errors.New("execution permit expected mutation digest is inconsistent")
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return result, errors.New("invalid compact execution permit JWS")
	}
	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return result, err
	}
	headerValue, err := parseStrictJSON(headerBytes)
	if err != nil {
		return result, err
	}
	header, ok := headerValue.(map[string]any)
	if !ok || !exactKeys(header, map[string]struct{}{"alg": {}, "kid": {}, "typ": {}}) ||
		header["alg"] != "RS256" || header["typ"] != executionPermitType || !bytes.Equal(headerBytes, canonicalJSON(header)) {
		return result, errors.New("invalid Auth execution permit protected header")
	}
	kid, ok := header["kid"].(string)
	if !ok || kid == "" {
		return result, errors.New("missing Auth execution permit key ID")
	}
	key := authKeys[kid]
	if key == nil {
		return result, errors.New("unknown Auth execution permit signing key")
	}
	signature, err := decodeSegment(parts[2])
	if err != nil {
		return result, err
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return result, errors.New("invalid Auth execution permit signature")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return result, err
	}
	payloadValue, err := parseStrictJSON(payloadBytes)
	if err != nil {
		return result, err
	}
	claims, ok := payloadValue.(map[string]any)
	fields := map[string]struct{}{
		"version": {}, "iss": {}, "aud": {}, "jti": {}, "operation": {}, "scope": {}, "operation_id": {}, "request_sha256": {},
		"application_id": {}, "environment_id": {}, "account_id": {}, "actor_id": {}, "binding_id": {}, "profile_id": {}, "device_id": {}, "key_id": {},
		"device_generation": {}, "authority_revision": {}, "gis_permit_id": {}, "binding_revision": {}, "assertion_jti": {},
		"iat_ms": {}, "expires_at_ms": {}, "exp": {},
	}
	if !ok || !exactKeys(claims, fields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) ||
		claims["iss"] != "auth" || claims["aud"] != messageAudience || claims["operation"] != "message.send" || claims["scope"] != "game.chat.send" {
		return result, errors.New("invalid Auth execution permit claims")
	}
	if _, err := integer(claims["version"], 1, 1); err != nil {
		return result, err
	}
	parseID := func(name string) (uuid.UUID, error) { return canonicalUUID(asString(claims[name])) }
	if result.ID, err = parseID("jti"); err != nil {
		return result, err
	}
	if result.OperationID, err = parseID("operation_id"); err != nil {
		return result, err
	}
	if result.ApplicationID, err = parseID("application_id"); err != nil {
		return result, err
	}
	if result.EnvironmentID, err = parseID("environment_id"); err != nil {
		return result, err
	}
	if result.AccountID, err = parseID("account_id"); err != nil {
		return result, err
	}
	if result.ActorID, err = parseID("actor_id"); err != nil {
		return result, err
	}
	if result.BindingID, err = parseID("binding_id"); err != nil {
		return result, err
	}
	if result.ProfileID, err = parseID("profile_id"); err != nil {
		return result, err
	}
	if result.DeviceID, err = parseID("device_id"); err != nil {
		return result, err
	}
	if result.KeyID, err = parseID("key_id"); err != nil {
		return result, err
	}
	if result.GISPermitID, err = parseID("gis_permit_id"); err != nil {
		return result, err
	}
	if result.AssertionJTI, err = parseID("assertion_jti"); err != nil {
		return result, err
	}
	result.DeviceGeneration, err = integer(claims["device_generation"], 1, maxSafeInteger)
	if err != nil {
		return result, err
	}
	result.AuthorityRevision, err = integer(claims["authority_revision"], 1, maxSafeInteger)
	if err != nil {
		return result, err
	}
	result.BindingRevision, err = integer(claims["binding_revision"], 1, maxSafeInteger)
	if err != nil {
		return result, err
	}
	issuedMS, err := integer(claims["iat_ms"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	expiresMS, err := integer(claims["expires_at_ms"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	expiresSeconds, err := integer(claims["exp"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	if expiresMS <= issuedMS || expiresMS-issuedMS > 3750 || expiresSeconds != expiresMS/1000 || expiresMS > expected.Device.ExpiresAt.UnixMilli() {
		return result, errors.New("invalid execution permit validity window")
	}
	issued := time.UnixMilli(issuedMS).UTC()
	expires := time.UnixMilli(expiresMS).UTC()
	now = now.UTC()
	if issued.After(now.Add(250*time.Millisecond)) || !now.Before(expires.Add(-250*time.Millisecond)) {
		return result, errors.New("execution permit is expired or not yet valid")
	}
	result.IssuedAt, result.ExpiresAt = issued, expires
	result.RequestSHA256 = asString(claims["request_sha256"])
	if len(result.RequestSHA256) != 64 || strings.ToLower(result.RequestSHA256) != result.RequestSHA256 || result.RequestSHA256 != digestHex {
		return result, errors.New("execution permit mutation digest mismatch")
	}
	result.Scope = asString(claims["scope"])
	if result.OperationID != expected.OperationID || result.AssertionJTI != expected.Device.AssertionJTI ||
		result.ApplicationID != expected.Device.ApplicationID || result.EnvironmentID != expected.Device.EnvironmentID ||
		result.AccountID != expected.Device.AccountID || result.ActorID != expected.Device.ActorID ||
		result.BindingID != expected.Device.BindingID || result.DeviceID != expected.Device.DeviceID ||
		result.KeyID != expected.Device.KeyID || result.DeviceGeneration != expected.Device.DeviceGeneration ||
		result.AuthorityRevision != expected.Device.AuthorityRevision {
		return result, errors.New("execution permit identity, operation, or authority revision mismatch")
	}
	return result, nil
}
