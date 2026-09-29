package sdkprotocolclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDeviceKeysProduceIndependentRFC7638JWKsAndThumbprints(t *testing.T) {
	first, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	if first.Thumbprint() == second.Thumbprint() {
		t.Fatal("generated device keys must be independent")
	}
	var jwk map[string]string
	if err := json.Unmarshal([]byte(first.PublicJWK()), &jwk); err != nil {
		t.Fatal(err)
	}
	if len(jwk) != 4 || jwk["kty"] != "EC" || jwk["crv"] != "P-256" || jwk["d"] != "" {
		t.Fatalf("public JWK leaked private fields or has wrong members: %v", jwk)
	}
	canonical := `{"crv":"P-256","kty":"EC","x":"` + jwk["x"] + `","y":"` + jwk["y"] + `"}`
	digest := sha256.Sum256([]byte(canonical))
	want := base64.RawURLEncoding.EncodeToString(digest[:])
	if first.Thumbprint() != want {
		t.Fatalf("thumbprint = %s, want RFC 7638 hash %s", first.Thumbprint(), want)
	}
}

func TestSignProofUsesES256CompactJWSAndExactProtectedHeader(t *testing.T) {
	key, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	proof, err := key.SignProof(map[string]any{"purpose": "revoke", "version": int64(1)}, "device-key-7", "voice.game-device-key-proof+jws")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("compact JWS has %d parts", len(parts))
	}
	var header map[string]any
	if err := json.Unmarshal(decodeSegment(t, parts[0]), &header); err != nil {
		t.Fatal(err)
	}
	if len(header) != 3 || header["alg"] != "ES256" || header["kid"] != "device-key-7" || header["typ"] != "voice.game-device-key-proof+jws" {
		t.Fatalf("unexpected JWS protected header: %v", header)
	}
	var payload map[string]any
	if err := json.Unmarshal(decodeSegment(t, parts[1]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["purpose"] != "revoke" || payload["version"] != float64(1) {
		t.Fatalf("unexpected payload: %v", payload)
	}
	sig := decodeSegment(t, parts[2])
	if len(sig) != 64 {
		t.Fatalf("ES256 signature has %d bytes, want 64", len(sig))
	}
	public := key.PublicKey()
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(public, hash[:], r, s) {
		t.Fatal("signature did not verify with the published device key")
	}
}

func TestProofRejectsNonCanonicalOrUnsupportedClaims(t *testing.T) {
	key, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, claims := range []map[string]any{
		{"issued_at": 1.25},
		{"unexpected": []string{"value"}},
		{"bad": string([]byte{0xff})},
	} {
		if _, err := key.SignProof(claims, "kid", "voice.game-device-key-proof+jws"); err == nil {
			t.Fatalf("expected unsupported claim value to fail: %#v", claims)
		}
	}
}

func TestCanonicalJSONEscapesStringsAndSortsClaims(t *testing.T) {
	got, err := canonicalJSON(map[string]any{"z": "line\nbreak", "a": "quote\"slash\\"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"quote\"slash\\","z":"line\nbreak"}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
}

func TestLifecycleProofBuildersMatchT15ClaimSets(t *testing.T) {
	current, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := NewDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	ctx := LifecycleProofContext{
		RequestID: "request", ChallengeID: "challenge", Nonce: "nonce", ApplicationID: "app",
		EnvironmentID: "env", DeviceID: "device", ReplacesDeviceID: "lost-device", IssuedAtUnixMS: 123456,
	}
	currentProof, err := current.SignRotateCurrentProof(ctx, "current-key", replacement.Thumbprint())
	if err != nil {
		t.Fatal(err)
	}
	currentClaims := decodeProofClaims(t, currentProof)
	assertClaimKeys(t, currentClaims, "application_id", "audience", "challenge_id", "device_id", "environment_id", "issued_at", "key_id", "key_thumbprint", "new_key_thumbprint", "nonce", "purpose", "request_id", "version")
	if currentClaims["purpose"] != "rotate_current" || currentClaims["key_thumbprint"] != current.Thumbprint() || currentClaims["new_key_thumbprint"] != replacement.Thumbprint() {
		t.Fatalf("wrong current-key claims: %v", currentClaims)
	}

	newProof, err := replacement.SignRotateNewProof(ctx)
	if err != nil {
		t.Fatal(err)
	}
	newClaims := decodeProofClaims(t, newProof)
	assertClaimKeys(t, newClaims, "application_id", "audience", "challenge_id", "device_id", "environment_id", "issued_at", "key_thumbprint", "nonce", "purpose", "request_id", "version")
	if newClaims["purpose"] != "rotate_new" || newClaims["device_id"] != "device" {
		t.Fatalf("wrong new-key claims: %v", newClaims)
	}
	if decodeProofHeader(t, newProof)["kid"] != nil {
		t.Fatal("new-key proof must not name a current key ID")
	}

	recoveryProof, err := replacement.SignRecoverNewProof(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recoveryClaims := decodeProofClaims(t, recoveryProof)
	assertClaimKeys(t, recoveryClaims, "application_id", "audience", "challenge_id", "environment_id", "issued_at", "key_thumbprint", "nonce", "purpose", "replaces_device_id", "request_id", "version")
	if recoveryClaims["purpose"] != "recover_new" || recoveryClaims["replaces_device_id"] != "lost-device" {
		t.Fatalf("wrong recovery claims: %v", recoveryClaims)
	}

	revokeProof, err := current.SignRevokeProof(ctx, "current-key")
	if err != nil {
		t.Fatal(err)
	}
	revokeClaims := decodeProofClaims(t, revokeProof)
	assertClaimKeys(t, revokeClaims, "application_id", "audience", "device_id", "environment_id", "issued_at", "key_id", "key_thumbprint", "purpose", "request_id", "version")
	if revokeClaims["purpose"] != "revoke" {
		t.Fatalf("wrong revoke claims: %v", revokeClaims)
	}
}

func decodeProofHeader(t *testing.T, proof string) map[string]any {
	t.Helper()
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("compact JWS has %d parts", len(parts))
	}
	var header map[string]any
	if err := json.Unmarshal(decodeSegment(t, parts[0]), &header); err != nil {
		t.Fatal(err)
	}
	return header
}

func decodeProofClaims(t *testing.T, proof string) map[string]any {
	t.Helper()
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("compact JWS has %d parts", len(parts))
	}
	var claims map[string]any
	if err := json.Unmarshal(decodeSegment(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func assertClaimKeys(t *testing.T, claims map[string]any, expected ...string) {
	t.Helper()
	if len(claims) != len(expected) {
		t.Fatalf("claim count = %d, expected %d: %v", len(claims), len(expected), claims)
	}
	for _, key := range expected {
		if _, ok := claims[key]; !ok {
			t.Errorf("missing claim %q: %v", key, claims)
		}
	}
}

func TestClientPostsOnlyRelativeAuthSDKPathsAndReturnsRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/sdk/device-keys/challenges" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer provider-proof" {
			t.Errorf("missing provider proof")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"challenge_id":"c"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.PostJSON(context.Background(), "/api/v1/auth/sdk/device-keys/challenges", "provider-proof", map[string]any{"purpose": "recover"})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"challenge_id":"c"}` {
		t.Fatalf("response = %s", body)
	}
	if _, err := client.PostJSON(context.Background(), "https://attacker.invalid/steal", "provider-proof", map[string]any{}); err == nil {
		t.Fatal("absolute paths must not redirect protocol credentials")
	}
}

func TestPublicLifecycleClientUsesFrozenAuthRoutesAndJSONNames(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/sdk/device-keys/challenges":
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request["purpose"] != "rotate" || request["publicJwk"] == nil || request["applicationId"] == nil || request["environmentId"] == nil {
				t.Errorf("challenge request does not match public Auth fields: %v", request)
			}
			_, _ = w.Write([]byte(`{"challengeId":"challenge-1","nonce":"nonce-1","clientId":"client","expiresAt":"2026-01-01T00:00:00Z","purpose":"rotate","deviceId":"device-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/sdk/device-keys/rotate":
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			for _, field := range []string{"challengeId", "providerToken", "currentKeyProof", "newKeyProof", "requestId"} {
				if request[field] == nil {
					t.Errorf("rotate request is missing %s: %v", field, request)
				}
			}
			_, _ = w.Write([]byte(`{"deviceId":"device-1","keyId":"key-2","generation":2,"authorityRevision":2}`))
		default:
			t.Errorf("unexpected lifecycle request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := client.CreateDeviceKeyChallenge(context.Background(), "provider-token", DeviceKeyChallengeRequest{
		Purpose: "rotate", ApplicationID: "app-1", EnvironmentID: "env-1", PublicJWK: `{"kty":"EC"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.ChallengeID != "challenge-1" || challenge.Nonce != "nonce-1" || challenge.DeviceID != "device-1" {
		t.Fatalf("unexpected challenge: %+v", challenge)
	}
	result, err := client.RotateDeviceKey(context.Background(), RotateDeviceKeyRequest{
		ChallengeID: challenge.ChallengeID, ProviderToken: "provider-token", CurrentKeyProof: "current-jws",
		NewKeyProof: "new-jws", RequestID: "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.KeyID != "key-2" || result.Generation != 2 || requests != 2 {
		t.Fatalf("result=%+v requests=%d", result, requests)
	}
}

func TestPublicLifecycleClientUsesDeleteForDeviceRevocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/auth/sdk/devices/device-1" {
			t.Errorf("revoke request = %s %s", r.Method, r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["providerToken"] != "provider-proof" || request["currentKeyProof"] != "signed-proof" || request["requestId"] != "request-1" {
			t.Errorf("revoke body mismatch: %v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deviceId":"device-1","keyId":"key-1","generation":1,"authorityRevision":3}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.RevokeDevice(context.Background(), "device-1", RevokeDeviceRequest{
		ProviderToken: "provider-proof", CurrentKeyProof: "signed-proof", RequestID: "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AuthorityRevision != 3 {
		t.Fatalf("authority revision = %d", result.AuthorityRevision)
	}
}

func TestPublicLifecycleClientUsesRecoverRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/sdk/device-keys/recover" {
			t.Errorf("recovery request = %s %s", r.Method, r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		for _, field := range []string{"challengeId", "providerToken", "newKeyProof", "requestId"} {
			if request[field] == nil {
				t.Errorf("recovery body missing %s: %v", field, request)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deviceId":"device-2","keyId":"key-2","generation":1,"authorityRevision":4}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.RecoverDeviceKey(context.Background(), RecoverDeviceKeyRequest{
		ChallengeID: "challenge-1", ProviderToken: "provider-proof", NewKeyProof: "new-jws", RequestID: "request-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeviceID != "device-2" || result.AuthorityRevision != 4 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClientNeverFollowsRedirectsWithProtocolCredentials(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Store(true) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.PostJSON(context.Background(), "/api/v1/auth/sdk/device-keys/challenges", "provider-secret", map[string]any{}); err == nil {
		t.Fatal("redirect response must be an error")
	}
	if redirected.Load() {
		t.Fatal("Auth credentials were forwarded to a redirect target")
	}
}

func TestFixedScalarKeyCanVerifyClientSignature(t *testing.T) {
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()}, D: big.NewInt(1)}
	priv.PublicKey.X, priv.PublicKey.Y = elliptic.P256().ScalarBaseMult(priv.D.Bytes())
	key := deviceKeyFromPrivate(priv)
	proof, err := key.SignProof(map[string]any{"audience": "voice.auth.device-key"}, "fixed-kid", "voice.game-device-key-proof+jws")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(proof, ".")
	sig := decodeSegment(t, parts[2])
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(&priv.PublicKey, hash[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("fixed-key proof signature did not verify")
	}
}

func decodeSegment(t *testing.T, encoded string) []byte {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
