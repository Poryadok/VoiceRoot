package gameprotocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const tombstoneType = "voice.game-message-tombstone+jws"

// GameMessageTombstoneKey is a Messaging-owned Ed25519 key. NotAfter is
// exclusive; RevokedAt prevents new signatures but preserves historical
// verification of signatures issued before revocation.
type GameMessageTombstoneKey struct {
	KeyID      uuid.UUID
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	NotBefore  time.Time
	NotAfter   time.Time
	RevokedAt  *time.Time
}

type GameMessageTombstone struct {
	ApplicationID        uuid.UUID
	EnvironmentID        uuid.UUID
	ChatID               uuid.UUID
	MessageID            uuid.UUID
	Revision             int64
	PreviousRevisionHash string
	ActionID             uuid.UUID
	ReasonClass          string
	IssuedAt             time.Time
}

func SignGameMessageTombstone(tombstone GameMessageTombstone, key GameMessageTombstoneKey, now time.Time) (string, error) {
	if key.KeyID == uuid.Nil || len(key.PrivateKey) != ed25519.PrivateKeySize || len(key.PublicKey) != ed25519.PublicKeySize ||
		!key.NotBefore.Before(key.NotAfter) || now.Before(key.NotBefore) || !now.Before(key.NotAfter) ||
		(key.RevokedAt != nil && !now.Before(*key.RevokedAt)) {
		return "", errors.New("tombstone signing key is unavailable or invalid")
	}
	if tombstone.ApplicationID == uuid.Nil || tombstone.EnvironmentID == uuid.Nil || tombstone.ChatID == uuid.Nil || tombstone.MessageID == uuid.Nil || tombstone.ActionID == uuid.Nil ||
		tombstone.Revision < 2 || !isLowerHexDigest(tombstone.PreviousRevisionHash) ||
		(tombstone.ReasonClass != "moderation" && tombstone.ReasonClass != "system_retention") {
		return "", errors.New("invalid game message tombstone")
	}
	issued := tombstone.IssuedAt.UTC()
	if issued.IsZero() || issued.After(now.UTC().Add(30*time.Second)) || now.UTC().Sub(issued) > 30*time.Second || issued.Before(key.NotBefore) || !issued.Before(key.NotAfter) ||
		(key.RevokedAt != nil && !issued.Before(*key.RevokedAt)) {
		return "", errors.New("tombstone issue time is outside signing key validity")
	}
	header := map[string]any{"alg": "EdDSA", "kid": key.KeyID.String(), "typ": tombstoneType}
	payload := map[string]any{
		"version": json.Number("1"), "operation": "moderator_delete", "issuer": "messaging", "audience": messageAudience,
		"application_id": tombstone.ApplicationID.String(), "environment_id": tombstone.EnvironmentID.String(),
		"chat_id": tombstone.ChatID.String(), "message_id": tombstone.MessageID.String(), "revision": json.Number(fmt.Sprintf("%d", tombstone.Revision)),
		"previous_revision_hash": tombstone.PreviousRevisionHash, "action_id": tombstone.ActionID.String(),
		"reason_class": tombstone.ReasonClass, "issued_at": issued.Format(time.RFC3339),
	}
	headerPart := base64.RawURLEncoding.EncodeToString(canonicalJSON(header))
	payloadPart := base64.RawURLEncoding.EncodeToString(canonicalJSON(payload))
	signingInput := headerPart + "." + payloadPart
	signature := ed25519.Sign(key.PrivateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func VerifyGameMessageTombstone(compact string, keys map[string]GameMessageTombstoneKey, now time.Time) (GameMessageTombstone, error) {
	var result GameMessageTombstone
	parts := strings.Split(compact, ".")
	if len(parts) != 3 || compact == "" || len(compact) > 16*1024 {
		return result, errors.New("invalid tombstone compact JWS")
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
	if !ok || !exactKeys(header, map[string]struct{}{"alg": {}, "kid": {}, "typ": {}}) || header["alg"] != "EdDSA" || header["typ"] != tombstoneType || !bytes.Equal(headerBytes, canonicalJSON(header)) {
		return result, errors.New("invalid tombstone protected header")
	}
	kid, ok := header["kid"].(string)
	if !ok {
		return result, errors.New("missing tombstone key ID")
	}
	key, ok := keys[kid]
	keyID, keyErr := canonicalUUID(kid)
	if !ok || keyErr != nil || keyID != key.KeyID || len(key.PublicKey) != ed25519.PublicKeySize || now.Before(key.NotBefore) {
		return result, errors.New("unknown or invalid tombstone verification key")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return result, err
	}
	payloadValue, err := parseStrictJSON(payloadBytes)
	if err != nil {
		return result, err
	}
	fields := map[string]struct{}{"version": {}, "operation": {}, "issuer": {}, "audience": {}, "application_id": {}, "environment_id": {}, "chat_id": {}, "message_id": {}, "revision": {}, "previous_revision_hash": {}, "action_id": {}, "reason_class": {}, "issued_at": {}}
	claims, ok := payloadValue.(map[string]any)
	version, versionErr := integer(claims["version"], 1, 1)
	if !ok || !exactKeys(claims, fields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) || versionErr != nil || version != 1 || claims["operation"] != "moderator_delete" || claims["issuer"] != "messaging" || claims["audience"] != messageAudience {
		return result, errors.New("invalid tombstone claims")
	}
	signature, err := decodeSegment(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(key.PublicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return result, errors.New("invalid tombstone signature")
	}
	parseID := func(name string) (uuid.UUID, error) {
		text, ok := claims[name].(string)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid %s", name)
		}
		return canonicalUUID(text)
	}
	if result.ApplicationID, err = parseID("application_id"); err != nil {
		return result, err
	}
	if result.EnvironmentID, err = parseID("environment_id"); err != nil {
		return result, err
	}
	if result.ChatID, err = parseID("chat_id"); err != nil {
		return result, err
	}
	if result.MessageID, err = parseID("message_id"); err != nil {
		return result, err
	}
	if result.ActionID, err = parseID("action_id"); err != nil {
		return result, err
	}
	result.Revision, err = integer(claims["revision"], 2, maxSafeInteger)
	if err != nil {
		return result, err
	}
	result.PreviousRevisionHash, ok = claims["previous_revision_hash"].(string)
	if !ok || !isLowerHexDigest(result.PreviousRevisionHash) {
		return result, errors.New("invalid previous revision hash")
	}
	result.ReasonClass, ok = claims["reason_class"].(string)
	if !ok || (result.ReasonClass != "moderation" && result.ReasonClass != "system_retention") {
		return result, errors.New("invalid tombstone reason class")
	}
	issued, err := rfc3339Second(claims["issued_at"])
	if err != nil || issued.After(now.Add(30*time.Second)) || now.Sub(issued) > 30*time.Second || issued.Before(key.NotBefore) || !issued.Before(key.NotAfter) || (key.RevokedAt != nil && !issued.Before(*key.RevokedAt)) {
		return result, errors.New("tombstone issue time is outside key validity")
	}
	result.IssuedAt = issued
	return result, nil
}

func TombstoneHash(compact string) string {
	digest := sha256.Sum256([]byte(compact))
	return fmt.Sprintf("%x", digest[:])
}
