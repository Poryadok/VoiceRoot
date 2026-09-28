package gameprotocol

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVerifyExecutionPermitBindsCanonicalMutationAndCurrentAuthority(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	device := gameAuthorityForPermit()
	device.AssertionJTI = uuid.New()
	operationID := uuid.New()
	mutation := []byte(`{"version":1,"operation":"create","body":"hi"}`)
	requestDigest := sha256.Sum256(mutation)
	claims := validExecutionPermitClaims(device, operationID, hex.EncodeToString(requestDigest[:]), now)
	compact := signExecutionPermitForTest(t, key, claims)
	got, err := VerifyExecutionPermit(compact, map[string]*rsa.PublicKey{"auth-1": &key.PublicKey}, now, 0, ExecutionPermitExpected{
		Device: device, OperationID: operationID, RequestSHA256: hex.EncodeToString(requestDigest[:]), MutationBytes: mutation,
	})
	require.NoError(t, err)
	require.Equal(t, uuid.MustParse(claims["gis_permit_id"].(string)), got.GISPermitID)
	require.Equal(t, uuid.MustParse(claims["profile_id"].(string)), got.ProfileID)
	require.Equal(t, operationID, got.OperationID)
	require.Equal(t, "game.chat.send", got.Scope)
	require.Equal(t, now.Add(3750*time.Millisecond), got.ExpiresAt)

	for name, alter := range map[string]func(map[string]any){
		"wrong operation":          func(c map[string]any) { c["operation_id"] = uuid.NewString() },
		"wrong payload digest":     func(c map[string]any) { c["request_sha256"] = hex.EncodeToString(make([]byte, 32)) },
		"wrong assertion jti":      func(c map[string]any) { c["assertion_jti"] = uuid.NewString() },
		"invalid binding revision": func(c map[string]any) { c["binding_revision"] = float64(0) },
		"invalid selected profile": func(c map[string]any) { c["profile_id"] = "" },
		"wrong scope":              func(c map[string]any) { c["scope"] = "game.chat.read" },
		"wrong operation class":    func(c map[string]any) { c["operation"] = "moderator.delete" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneClaims(claims)
			alter(changed)
			_, err := VerifyExecutionPermit(signExecutionPermitForTest(t, key, changed), map[string]*rsa.PublicKey{"auth-1": &key.PublicKey}, now, 0, ExecutionPermitExpected{
				Device: device, OperationID: operationID, RequestSHA256: hex.EncodeToString(requestDigest[:]), MutationBytes: mutation,
			})
			require.Error(t, err)
		})
	}
}

func TestVerifyExecutionPermitRejectsExpiredOrUncertainDeadlineAndExtraClaims(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	device := gameAuthorityForPermit()
	device.AssertionJTI = uuid.New()
	operationID := uuid.New()
	mutation := []byte(`{"version":1}`)
	digest := sha256.Sum256(mutation)
	requestHash := hex.EncodeToString(digest[:])
	claims := validExecutionPermitClaims(device, operationID, requestHash, now)
	for name, test := range map[string]struct {
		now         time.Time
		uncertainty time.Duration
		mutate      func(map[string]any)
	}{
		"expiry equality with margin": {now: now.Add(3500 * time.Millisecond)},
		"uncertainty over bound":      {now: now, uncertainty: 251 * time.Millisecond},
		"extra claim":                 {now: now, mutate: func(c map[string]any) { c["chat_id"] = uuid.NewString() }},
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneClaims(claims)
			if test.mutate != nil {
				test.mutate(changed)
			}
			_, err := VerifyExecutionPermit(signExecutionPermitForTest(t, key, changed), map[string]*rsa.PublicKey{"auth-1": &key.PublicKey}, test.now, test.uncertainty, ExecutionPermitExpected{
				Device: device, OperationID: operationID, RequestSHA256: requestHash, MutationBytes: mutation,
			})
			require.Error(t, err)
		})
	}
}

func validExecutionPermitClaims(device DeviceAuthority, operationID uuid.UUID, digest string, now time.Time) map[string]any {
	expiry := now.Add(3750 * time.Millisecond)
	return map[string]any{
		"version": 1, "iss": "auth", "aud": "voice.game-message", "jti": uuid.NewString(),
		"operation": "message.send", "scope": "game.chat.send", "operation_id": operationID.String(), "request_sha256": digest,
		"application_id": device.ApplicationID.String(), "environment_id": device.EnvironmentID.String(),
		"account_id": device.AccountID.String(), "actor_id": device.ActorID.String(), "binding_id": device.BindingID.String(), "profile_id": uuid.NewString(),
		"device_id": device.DeviceID.String(), "key_id": device.KeyID.String(), "device_generation": int64(3),
		"authority_revision": device.AuthorityRevision, "gis_permit_id": uuid.NewString(), "binding_revision": int64(4),
		"assertion_jti": device.AssertionJTI.String(), "iat_ms": now.UnixMilli(), "expires_at_ms": expiry.UnixMilli(),
		"exp": expiry.Unix(),
	}
}

func gameAuthorityForPermit() DeviceAuthority {
	return DeviceAuthority{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), AccountID: uuid.New(), ActorID: uuid.New(), BindingID: uuid.New(),
		DeviceID: uuid.New(), KeyID: uuid.New(), DeviceGeneration: 3, AuthorityRevision: 7,
		ExpiresAt: time.Date(2026, 9, 28, 10, 0, 4, 0, time.UTC),
	}
}

func signExecutionPermitForTest(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": "auth-1", "typ": "voice.game-message-execution-permit+jwt"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func cloneClaims(source map[string]any) map[string]any {
	encoded, _ := json.Marshal(source)
	var result map[string]any
	_ = json.Unmarshal(encoded, &result)
	return result
}
