package gameprotocol

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVerifyMessageAcceptsCanonicalSignedCreateAndReturnsExactContent(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, keyID := testDeviceKey(t)
	content := []byte("héllo")
	compact := signedEnvelope(t, key, keyID, now, content, nil)

	got, err := VerifyMessage(compact, publicJWK(key), now, testExpected())

	require.NoError(t, err)
	require.Equal(t, "create", got.Operation)
	require.Equal(t, content, got.Content)
	require.Equal(t, int64(1), got.Revision)
	require.Equal(t, uuid.MustParse(keyID), got.KeyID)
	require.Equal(t, compact, got.Compact)
}

func TestVerifyMessageRejectsNoncanonicalPayloadAndRouteSubstitution(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, keyID := testDeviceKey(t)
	canonical := signedEnvelope(t, key, keyID, now, []byte("hello"), nil)
	parts := strings.Split(canonical, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var object map[string]any
	require.NoError(t, json.Unmarshal(payload, &object))
	mutated, err := json.MarshalIndent(object, "", " ")
	require.NoError(t, err)
	nonCanonical := signCompact(t, key, parts[0], base64.RawURLEncoding.EncodeToString(mutated))

	_, err = VerifyMessage(nonCanonical, publicJWK(key), now, testExpected())
	require.Error(t, err)

	expected := testExpected()
	expected.ChatID = uuid.NewString()
	_, err = VerifyMessage(canonical, publicJWK(key), now, expected)
	require.Error(t, err)
}

func TestVerifyMessageRejectsTamperingExpiredEnvelopeAndUnknownFields(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, keyID := testDeviceKey(t)
	compact := signedEnvelope(t, key, keyID, now, []byte("hello"), nil)
	parts := strings.Split(compact, ".")
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	_, err := VerifyMessage(strings.Join(parts, "."), publicJWK(key), now, testExpected())
	require.Error(t, err)

	expired := signedEnvelopeAt(t, key, keyID, now.Add(-6*time.Minute), []byte("hello"), false)
	_, err = VerifyMessage(expired,
		publicJWK(key), now, testExpected())
	require.Error(t, err)
	receipt, err := ExtractReceiptKey(expired)
	require.NoError(t, err)
	require.Equal(t, testExpected().ChatID, receipt.ChatID.String())
	require.Equal(t, expired, receipt.Compact)

	_, err = VerifyMessage(signedEnvelopeAt(t, key, keyID, now, []byte("hello"), true),
		publicJWK(key), now, testExpected())
	require.Error(t, err)
}

func TestVerifyMessageEnforcesTerminalDeleteShapeAndRevisionLink(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, keyID := testDeviceKey(t)
	claims := baseClaims(now)
	claims["operation"] = "delete"
	claims["revision"] = 2
	claims["previous_revision_hash"] = strings.Repeat("a", 64)
	claims["content_type"] = nil
	claims["content_b64"] = nil
	claims["content_sha256"] = nil
	compact := signedMap(t, key, keyID, claims)

	got, err := VerifyMessage(compact, publicJWK(key), now, testExpected())
	require.NoError(t, err)
	require.Equal(t, "delete", got.Operation)
	require.Empty(t, got.Content)

	claims["previous_revision_hash"] = "not-a-hash"
	_, err = VerifyMessage(signedMap(t, key, keyID, claims), publicJWK(key), now, testExpected())
	require.Error(t, err)
}

func TestVerifyRequestRequiresFreshAuthStatusAssertionAndUsesItsDeviceKey(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	device, keyID := testDeviceKey(t)
	message := signedEnvelope(t, device, keyID, now, []byte("hello"))
	authKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	assertion := signedDeviceAuthority(t, authKey, device, keyID, now, now.Add(2*time.Second))
	publicKey := &authKey.PublicKey

	gotMessage, authority, err := VerifyRequest(message, assertion, map[string]*rsa.PublicKey{"auth-current": publicKey},
		now, 250*time.Millisecond, testExpected())
	require.NoError(t, err)
	require.Equal(t, "hello", string(gotMessage.Content))
	require.Equal(t, keyID, authority.KeyID.String())
	require.NotEqual(t, uuid.Nil, authority.AssertionJTI, "the verified Auth assertion JTI binds the execution permit")
	require.Equal(t, now.Add(2*time.Second), authority.ExpiresAt)

	_, _, err = VerifyRequest(message, assertion, map[string]*rsa.PublicKey{"auth-current": publicKey},
		now.Add(1750*time.Millisecond), 250*time.Millisecond, testExpected())
	require.Error(t, err)
	_, _, err = VerifyRequest(message, assertion, map[string]*rsa.PublicKey{"auth-current": publicKey},
		now, 251*time.Millisecond, testExpected())
	require.Error(t, err)
	wrong := testExpected()
	wrong.BindingID = uuid.NewString()
	_, _, err = VerifyRequest(message, assertion, map[string]*rsa.PublicKey{"auth-current": publicKey},
		now, 0, wrong)
	require.Error(t, err)
}

func TestExtractDeviceAuthorityExpectedReadsExactAuthIdentityForEnvelopeCoupling(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	device, keyID := testDeviceKey(t)
	authKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	assertion := signedDeviceAuthority(t, authKey, device, keyID, now, now.Add(2*time.Second))
	got, err := ExtractDeviceAuthorityExpected(assertion)
	require.NoError(t, err)
	require.Equal(t, testExpected().ApplicationID, got.ApplicationID)
	require.Equal(t, testExpected().EnvironmentID, got.EnvironmentID)
	require.Equal(t, testExpected().BindingID, got.BindingID)

	parts := strings.Split(assertion, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"auth"}`))
	_, err = ExtractDeviceAuthorityExpected(strings.Join(parts, "."))
	require.Error(t, err)
}

func testExpected() Expected {
	return Expected{
		ApplicationID: "11111111-1111-4111-8111-111111111111",
		EnvironmentID: "22222222-2222-4222-8222-222222222222",
		AccountID:     "33333333-3333-4333-8333-333333333333",
		ActorID:       "44444444-4444-4444-8444-444444444444",
		BindingID:     "55555555-5555-4555-8555-555555555555",
		DeviceID:      "66666666-6666-4666-8666-666666666666",
		ChatID:        "88888888-8888-4888-8888-888888888888",
	}
}

func testDeviceKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key, uuid.NewString()
}

func publicJWK(key *ecdsa.PrivateKey) map[string]any {
	return map[string]any{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))),
		"y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))}
}

func signedEnvelope(t *testing.T, key *ecdsa.PrivateKey, keyID string, now time.Time, content []byte,
	extras ...map[string]any) string {
	t.Helper()
	return signedEnvelopeAt(t, key, keyID, now, content, false, extras...)
}

func signedEnvelopeAt(t *testing.T, key *ecdsa.PrivateKey, keyID string, now time.Time, content []byte,
	unknown bool, extras ...map[string]any) string {
	t.Helper()
	claims := baseClaims(now)
	claims["content_type"] = "text/plain"
	claims["content_b64"] = base64.RawURLEncoding.EncodeToString(content)
	digest := sha256.Sum256(content)
	claims["content_sha256"] = hex.EncodeToString(digest[:])
	if unknown {
		claims["unexpected"] = true
	}
	for _, extra := range extras {
		for name, value := range extra {
			claims[name] = value
		}
	}
	return signedMap(t, key, keyID, claims)
}

func baseClaims(now time.Time) map[string]any {
	messageID, _ := uuid.NewV7()
	return map[string]any{
		"version": 1, "operation": "create", "audience": "voice.game-message",
		"application_id": testExpected().ApplicationID, "environment_id": testExpected().EnvironmentID,
		"account_id": testExpected().AccountID, "actor_id": testExpected().ActorID,
		"binding_id": testExpected().BindingID, "device_id": testExpected().DeviceID,
		"operation_id": uuid.NewString(), "authority_revision": 1,
		"chat_id": testExpected().ChatID, "message_id": messageID.String(), "revision": 1,
		"previous_revision_hash": nil,
		"issued_at":              now.Add(-time.Second).UTC().Format("2006-01-02T15:04:05Z"),
		"expires_at":             now.Add(4 * time.Minute).UTC().Format("2006-01-02T15:04:05Z"),
		"content_type":           nil, "content_b64": nil, "content_sha256": nil,
		"attachment_manifest_b64": nil, "attachment_manifest_sha256": nil,
	}
}

func signedMap(t *testing.T, key *ecdsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "ES256", "kid": keyID, "typ": "voice.game-message+jws"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return signCompact(t, key, base64.RawURLEncoding.EncodeToString(header), base64.RawURLEncoding.EncodeToString(payload))
}

func signCompact(t *testing.T, key *ecdsa.PrivateKey, header, payload string) string {
	t.Helper()
	input := header + "." + payload
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	require.NoError(t, err)
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func signedDeviceAuthority(t *testing.T, key *rsa.PrivateKey, device *ecdsa.PrivateKey, keyID string,
	now, notAfter time.Time) string {
	t.Helper()
	jwk := publicJWK(device)
	thumb, err := p256Thumbprint(jwk)
	require.NoError(t, err)
	expires := now.Add(4 * time.Second)
	if notAfter.Before(expires) {
		expires = notAfter
	}
	e := testExpected()
	claims := map[string]any{
		"version": 1, "iss": "auth", "aud": "voice.game-message", "jti": uuid.NewString(),
		"application_id": e.ApplicationID, "environment_id": e.EnvironmentID,
		"account_id": e.AccountID, "actor_id": e.ActorID, "binding_id": e.BindingID,
		"device_id": e.DeviceID, "key_id": keyID, "public_jwk": jwk, "key_thumbprint": thumb,
		"device_generation": 1, "authority_revision": 1, "status": "active",
		"not_after": notAfter.UnixMilli(), "iat": now.UnixMilli(), "exp": expires.UnixMilli(),
	}
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": "auth-current", "typ": "voice.game-device-status+jwt"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	headerSegment := base64.RawURLEncoding.EncodeToString(header)
	payloadSegment := base64.RawURLEncoding.EncodeToString(payload)
	input := headerSegment + "." + payloadSegment
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}
