package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

var errInvalidDeviceAuthority = errors.New("invalid device authority assertion")

type deviceAuthorityFailure string

const (
	deviceAuthorityFormat         deviceAuthorityFailure = "format"
	deviceAuthorityHeader         deviceAuthorityFailure = "header"
	deviceAuthorityKeyID          deviceAuthorityFailure = "key_id"
	deviceAuthoritySignature      deviceAuthorityFailure = "signature"
	deviceAuthorityIssuerAudience deviceAuthorityFailure = "issuer_audience"
	deviceAuthorityClaims         deviceAuthorityFailure = "claims"
)

func (e deviceAuthorityFailure) Error() string { return errInvalidDeviceAuthority.Error() }
func (e deviceAuthorityFailure) Unwrap() error { return errInvalidDeviceAuthority }

func deviceAuthorityFailureStage(err error) string {
	var failure deviceAuthorityFailure
	if errors.As(err, &failure) {
		return string(failure)
	}
	return string(deviceAuthorityClaims)
}

var deviceAssertionFields = map[string]struct{}{
	"version": {}, "iss": {}, "aud": {}, "jti": {}, "application_id": {}, "environment_id": {},
	"account_id": {}, "actor_id": {}, "binding_id": {}, "device_id": {}, "key_id": {}, "public_jwk": {},
	"key_thumbprint": {}, "device_generation": {}, "authority_revision": {}, "status": {}, "not_after": {},
	"iat": {}, "exp": {},
}

func parseForwardedDeviceAuthority(raw []byte) (registry.GameDeviceAuthorityClaims, error) {
	var empty registry.GameDeviceAuthorityClaims
	if len(raw) == 0 || len(raw) > 16<<10 {
		return empty, deviceAuthorityFormat
	}
	parts := bytes.Split(raw, []byte("."))
	if len(parts) != 3 || len(parts[0]) == 0 || len(parts[1]) == 0 || len(parts[2]) == 0 {
		return empty, deviceAuthorityFormat
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(string(parts[0]))
	if err != nil || base64.RawURLEncoding.EncodeToString(headerBytes) != string(parts[0]) {
		return empty, deviceAuthorityFormat
	}
	header, err := decodeUniqueJSONObject(headerBytes)
	if err != nil || len(header) != 3 {
		return empty, deviceAuthorityHeader
	}
	if string(header["alg"]) != `"RS256"` || string(header["typ"]) != `"voice.game-device-status+jwt"` {
		return empty, deviceAuthorityHeader
	}
	keyID, err := rawString(header["kid"])
	if err != nil || !validAuthSigningKeyID(keyID) {
		return empty, deviceAuthorityKeyID
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(string(parts[1]))
	if err != nil || base64.RawURLEncoding.EncodeToString(payloadBytes) != string(parts[1]) {
		return empty, deviceAuthorityFormat
	}
	payload, err := decodeUniqueJSONObject(payloadBytes)
	if err != nil || len(payload) != len(deviceAssertionFields) {
		return empty, deviceAuthorityClaims
	}
	for name := range payload {
		if _, ok := deviceAssertionFields[name]; !ok {
			return empty, deviceAuthorityClaims
		}
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(string(parts[2]))
	if err != nil || len(signatureBytes) == 0 || base64.RawURLEncoding.EncodeToString(signatureBytes) != string(parts[2]) {
		// The assertion signature's issuer authentication is the signed Auth
		// workload proof on this private route; Auth validates it before forwarding.
		return empty, deviceAuthoritySignature
	}
	version, err := rawInt64(payload["version"])
	if err != nil || version != 1 {
		return empty, deviceAuthorityClaims
	}
	issuer, err := rawString(payload["iss"])
	if err != nil {
		return empty, deviceAuthorityIssuerAudience
	}
	audience, err := rawString(payload["aud"])
	if err != nil || issuer != "auth" || audience != "voice.game-message" {
		return empty, deviceAuthorityIssuerAudience
	}
	status, err := rawString(payload["status"])
	if err != nil || status != "active" {
		return empty, deviceAuthorityClaims
	}
	thumbprint, err := rawString(payload["key_thumbprint"])
	if err != nil || thumbprint == "" {
		return empty, deviceAuthorityClaims
	}
	if _, err := decodeUniqueJSONObject(payload["public_jwk"]); err != nil {
		return empty, deviceAuthorityClaims
	}
	claims := registry.GameDeviceAuthorityClaims{Issuer: issuer, Audience: audience, Version: version,
		Status: status}
	for name, target := range map[string]*uuid.UUID{
		"jti": &claims.AssertionID, "application_id": &claims.ApplicationID,
		"environment_id": &claims.EnvironmentID, "account_id": &claims.AccountID,
		"actor_id": &claims.ActorID, "binding_id": &claims.BindingID, "device_id": &claims.DeviceID,
		"key_id": &claims.KeyID,
	} {
		*target, err = rawUUID(payload[name])
		if err != nil {
			return empty, deviceAuthorityClaims
		}
	}
	claims.DeviceGeneration, err = rawInt64(payload["device_generation"])
	if err != nil || claims.DeviceGeneration <= 0 {
		return empty, deviceAuthorityClaims
	}
	claims.AuthorityRevision, err = rawInt64(payload["authority_revision"])
	if err != nil || claims.AuthorityRevision <= 0 {
		return empty, deviceAuthorityClaims
	}
	notAfter, err := rawInt64(payload["not_after"])
	if err != nil || notAfter <= 0 {
		return empty, deviceAuthorityClaims
	}
	issuedAt, err := rawInt64(payload["iat"])
	if err != nil || issuedAt <= 0 {
		return empty, deviceAuthorityClaims
	}
	expiresAt, err := rawInt64(payload["exp"])
	if err != nil || expiresAt <= issuedAt || expiresAt > notAfter || expiresAt > issuedAt+4000 {
		return empty, deviceAuthorityClaims
	}
	claims.NotAfter = time.UnixMilli(notAfter).UTC()
	claims.IssuedAt = time.UnixMilli(issuedAt).UTC()
	claims.ExpiresAt = time.UnixMilli(expiresAt).UTC()
	claims.AssertionSHA256 = sha256.Sum256(raw)
	return claims, nil
}

// The JWS header kid identifies Auth's signing-key rotation slot. The payload
// key_id is a different UUID identifying the player's device key.
func validAuthSigningKeyID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index, character := range value {
		alphaNumeric := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
		if !alphaNumeric && character != '.' && character != '_' && character != '-' {
			return false
		}
		if index == 0 && !alphaNumeric {
			return false
		}
	}
	return true
}

func decodeUniqueJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errInvalidDeviceAuthority
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, errInvalidDeviceAuthority
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errInvalidDeviceAuthority
		}
		if _, exists := object[key]; exists {
			return nil, errInvalidDeviceAuthority
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errInvalidDeviceAuthority
		}
		object[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, errInvalidDeviceAuthority
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errInvalidDeviceAuthority
	}
	return object, nil
}

func rawUUID(raw json.RawMessage) (uuid.UUID, error) {
	value, err := rawString(raw)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errInvalidDeviceAuthority
	}
	return id, nil
}

func rawString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", errInvalidDeviceAuthority
	}
	return value, nil
}

func rawInt64(raw json.RawMessage) (int64, error) {
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, errInvalidDeviceAuthority
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || strconv.FormatInt(value, 10) != number.String() {
		return 0, errInvalidDeviceAuthority
	}
	return value, nil
}
