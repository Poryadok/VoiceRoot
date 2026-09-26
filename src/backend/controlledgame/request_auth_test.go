package controlledgame

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

const canonicalCommand = `{"action_id":"00000000-0000-4000-8000-000000000004","actor_proof":{"profile_id":"00000000-0000-4000-8000-000000000008","proof_id":"opaque"},"app_id":"00000000-0000-4000-8000-000000000005","arguments":{"encounter_id":"encounter-42"},"binding_revision":8,"card_revision":2,"command_id":"00000000-0000-4000-8000-000000000001","environment_id":"00000000-0000-4000-8000-000000000006","expires_at":1790500120,"installation_id":"00000000-0000-4000-8000-000000000007","invocation_id":"00000000-0000-4000-8000-000000000003","issued_at":1790500000,"message_id":"00000000-0000-4000-8000-00000000000b","operation_id":"00000000-0000-4000-8000-000000000002","schema_version":1,"state_version":"encounter-42:v3"}`

const vectorKeyID = "00000000-0000-4000-8000-00000000000a"
const vectorTimestamp = "1790500000"
const vectorPath = "/callbacks/game-commands/v1"
const vectorSignature = "v1=db75a725f07e9eedadb3a511f2ee871f640bfe395fac27168c0302ee8f6982e9"

// Test-facing seam for the implementation: authenticateRequest validates exact raw
// path grammar, canonical JSON bytes, timestamp skew, and the frozen v1 HMAC input.
func TestAuthenticateRequestAcceptsFrozenJCSAndHMACVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	now := time.Unix(1790500000, 0).UTC()

	if err := authenticateRequest("POST", vectorPath, vectorTimestamp, vectorKeyID, []byte(canonicalCommand), vectorSignature, key, now); err != nil {
		t.Fatalf("frozen contract vector rejected: %v", err)
	}

	bodyHash := sha256.Sum256([]byte(canonicalCommand))
	if got := hex.EncodeToString(bodyHash[:]); got != "dcdee56ea87770fd52707199e0f2f29fa7cbfb17e806b6c248308728c607e0a1" {
		t.Fatalf("fixture body hash drifted: %s", got)
	}
}

func TestAuthenticateRequestRejectsNonCanonicalOrInvalidOriginPath(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	now := time.Unix(1790500000, 0).UTC()
	for _, path := range []string{
		"/callbacks/game-commands/%76%31",
		"/callbacks//game-commands/v1",
		"/callbacks/./game-commands/v1",
		"/callbacks/../game-commands/v1",
		"/callbacks/game-commands/v1;extra",
		"/callbacks/game-commands/v1#fragment",
		"/callbacks/game-commands/v1?x=1",
		"/callbacks/game-commands/v1/\u00e9",
	} {
		t.Run(fmt.Sprintf("path_%x", []byte(path)), func(t *testing.T) {
			signature := testSignature(key, "POST", path, vectorTimestamp, vectorKeyID, []byte(canonicalCommand))
			if err := authenticateRequest("POST", path, vectorTimestamp, vectorKeyID, []byte(canonicalCommand), signature, key, now); err == nil {
				t.Fatalf("accepted invalid raw path %q", path)
			}
		})
	}
}

func TestAuthenticateRequestRejectsTamperedBodyAndSignature(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	now := time.Unix(1790500000, 0).UTC()

	wrongWellFormedSignature := vectorSignature[:len(vectorSignature)-1] + "0"
	if err := authenticateRequest("POST", vectorPath, vectorTimestamp, vectorKeyID, []byte(canonicalCommand), wrongWellFormedSignature, key, now); err == nil {
		t.Fatal("accepted a modified HMAC signature")
	}

	changed := strings.Replace(canonicalCommand, `"encounter-42"`, `"encounter-43"`, 1)
	if err := authenticateRequest("POST", vectorPath, vectorTimestamp, vectorKeyID, []byte(changed), vectorSignature, key, now); err == nil {
		t.Fatal("accepted body bytes that do not match the signed hash")
	}

	whitespace := canonicalCommand + " "
	signature := testSignature(key, "POST", vectorPath, vectorTimestamp, vectorKeyID, []byte(whitespace))
	if err := authenticateRequest("POST", vectorPath, vectorTimestamp, vectorKeyID, []byte(whitespace), signature, key, now); err == nil {
		t.Fatal("accepted JSON bytes that are not the exact JCS serialization")
	}
}

func testSignature(key []byte, method, path, timestamp, keyID string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	input := strings.Join([]string{"v1", method, path, timestamp, keyID, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(input))
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}
