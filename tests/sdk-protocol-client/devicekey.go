package sdkprotocolclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const coordinateSize = 32

// DeviceKey is a test-client device identity. The private key is intentionally
// not exported or included in the public JWK/JSON representation.
type DeviceKey struct{ private *ecdsa.PrivateKey }

// LifecycleProofContext contains the exact public Auth challenge tuple used by
// T15 key rotation and recovery proofs.
type LifecycleProofContext struct {
	RequestID        string
	ChallengeID      string
	Nonce            string
	ApplicationID    string
	EnvironmentID    string
	DeviceID         string
	ReplacesDeviceID string
	IssuedAtUnixMS   int64
}

func NewDeviceKey() (*DeviceKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return deviceKeyFromPrivate(key), nil
}

func deviceKeyFromPrivate(key *ecdsa.PrivateKey) *DeviceKey { return &DeviceKey{private: key} }

func (k *DeviceKey) PublicKey() *ecdsa.PublicKey {
	if k == nil || k.private == nil {
		return nil
	}
	return &k.private.PublicKey
}

func (k *DeviceKey) PublicJWK() string {
	if k == nil || k.private == nil {
		return ""
	}
	return `{"crv":"P-256","kty":"EC","x":"` + base64.RawURLEncoding.EncodeToString(padCoordinate(k.private.X)) + `","y":"` + base64.RawURLEncoding.EncodeToString(padCoordinate(k.private.Y)) + `"}`
}

func (k *DeviceKey) Thumbprint() string {
	if k == nil || k.private == nil {
		return ""
	}
	digest := sha256.Sum256([]byte(k.PublicJWK()))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (k *DeviceKey) SignRotateCurrentProof(context LifecycleProofContext, currentKeyID, replacementThumbprint string) (string, error) {
	claims := map[string]any{
		"version": int64(1), "purpose": "rotate_current", "audience": "voice.auth.device-key",
		"request_id": context.RequestID, "challenge_id": context.ChallengeID, "nonce": context.Nonce,
		"application_id": context.ApplicationID, "environment_id": context.EnvironmentID,
		"device_id": context.DeviceID, "key_id": currentKeyID, "key_thumbprint": k.Thumbprint(),
		"new_key_thumbprint": replacementThumbprint, "issued_at": context.IssuedAtUnixMS,
	}
	return k.SignProof(claims, currentKeyID, "voice.game-device-key-proof+jws")
}

func (k *DeviceKey) SignRotateNewProof(context LifecycleProofContext) (string, error) {
	return k.signNewLifecycleProof("rotate_new", context, false)
}

func (k *DeviceKey) SignRecoverNewProof(context LifecycleProofContext) (string, error) {
	return k.signNewLifecycleProof("recover_new", context, true)
}

func (k *DeviceKey) signNewLifecycleProof(purpose string, context LifecycleProofContext, recovery bool) (string, error) {
	claims := map[string]any{
		"version": int64(1), "purpose": purpose, "audience": "voice.auth.device-key",
		"request_id": context.RequestID, "challenge_id": context.ChallengeID, "nonce": context.Nonce,
		"application_id": context.ApplicationID, "environment_id": context.EnvironmentID,
		"key_thumbprint": k.Thumbprint(), "issued_at": context.IssuedAtUnixMS,
	}
	if recovery {
		claims["replaces_device_id"] = context.ReplacesDeviceID
	} else {
		claims["device_id"] = context.DeviceID
	}
	return k.SignProof(claims, "", "voice.game-device-key-proof+jws")
}

func (k *DeviceKey) SignRevokeProof(context LifecycleProofContext, keyID string) (string, error) {
	claims := map[string]any{
		"version": int64(1), "purpose": "revoke", "audience": "voice.auth.device-key",
		"request_id": context.RequestID, "application_id": context.ApplicationID,
		"environment_id": context.EnvironmentID, "device_id": context.DeviceID,
		"key_id": keyID, "key_thumbprint": k.Thumbprint(), "issued_at": context.IssuedAtUnixMS,
	}
	return k.SignProof(claims, keyID, "voice.game-device-key-proof+jws")
}

// SignProof signs a strict RFC 8785 payload using an ES256 compact JWS.
// Claims are limited to protocol strings and safe integer values; all T15
// device-key proof fields fit that profile.
func (k *DeviceKey) SignProof(claims map[string]any, keyID, typ string) (string, error) {
	if k == nil || k.private == nil || k.private.Curve != elliptic.P256() {
		return "", errors.New("device key must be P-256")
	}
	if !validHeaderValue(typ) || (keyID != "" && !validHeaderValue(keyID)) {
		return "", errors.New("invalid JWS protected header")
	}
	header := map[string]any{"alg": "ES256", "typ": typ}
	if keyID != "" {
		header["kid"] = keyID
	}
	headerBytes, err := canonicalJSON(header)
	if err != nil {
		return "", err
	}
	payloadBytes, err := canonicalJSON(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, k.private, digest[:])
	if err != nil {
		return "", err
	}
	if r.BitLen() > 256 || s.BitLen() > 256 {
		return "", errors.New("ES256 signature out of range")
	}
	signature := append(padCoordinate(r), padCoordinate(s)...)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func padCoordinate(n *big.Int) []byte {
	encoded := n.Bytes()
	if len(encoded) >= coordinateSize {
		return encoded
	}
	return append(make([]byte, coordinateSize-len(encoded)), encoded...)
}

func validHeaderValue(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func canonicalJSON(object map[string]any) ([]byte, error) {
	if object == nil {
		return nil, errors.New("claims object is required")
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		if key == "" || !isASCII(key) {
			return nil, fmt.Errorf("unsupported claim name %q", key)
		}
		keys = append(keys, key)
	}
	// T15 claim names are ASCII, for which UTF-16 and byte ordering agree.
	sortStrings(keys)
	var result strings.Builder
	result.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			result.WriteByte(',')
		}
		writeJSONString(&result, key)
		result.WriteByte(':')
		switch value := object[key].(type) {
		case string:
			if !isASCII(value) {
				return nil, fmt.Errorf("unsupported non-ASCII claim value for %q", key)
			}
			writeJSONString(&result, value)
		case int:
			if int64(value) > 9007199254740991 || int64(value) < -9007199254740991 {
				return nil, fmt.Errorf("unsafe JSON integer claim %q", key)
			}
			result.WriteString(fmt.Sprintf("%d", value))
		case int64:
			if value > 9007199254740991 || value < -9007199254740991 {
				return nil, fmt.Errorf("unsafe JSON integer claim %q", key)
			}
			result.WriteString(fmt.Sprintf("%d", value))
		default:
			return nil, fmt.Errorf("unsupported JSON claim value %T for %q", object[key], key)
		}
	}
	result.WriteByte('}')
	return []byte(result.String()), nil
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func writeJSONString(out *strings.Builder, value string) {
	out.WriteByte('"')
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch c {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				out.WriteString(fmt.Sprintf(`\u%04x`, c))
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
}
