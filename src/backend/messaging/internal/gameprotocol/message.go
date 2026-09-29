package gameprotocol

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	messageAudience = "voice.game-message"
	messageType     = "voice.game-message+jws"
	maxSafeInteger  = int64(9007199254740991)
)

var messageFields = map[string]struct{}{
	"version": {}, "operation": {}, "audience": {}, "application_id": {}, "environment_id": {},
	"account_id": {}, "actor_id": {}, "binding_id": {}, "device_id": {}, "operation_id": {},
	"authority_revision": {}, "chat_id": {}, "message_id": {}, "revision": {},
	"previous_revision_hash": {}, "issued_at": {}, "expires_at": {}, "content_type": {},
	"content_b64": {}, "content_sha256": {}, "attachment_manifest_b64": {}, "attachment_manifest_sha256": {},
}

type Expected struct {
	ApplicationID string
	EnvironmentID string
	AccountID     string
	ActorID       string
	BindingID     string
	DeviceID      string
	ChatID        string
}

type Attachment struct {
	FileID         uuid.UUID
	ObjectRevision int64
	ByteLength     int64
	ContentSHA256  string
	MediaType      string
}

type Message struct {
	Compact                  string
	KeyID                    uuid.UUID
	Operation                string
	ApplicationID            uuid.UUID
	EnvironmentID            uuid.UUID
	AccountID                uuid.UUID
	ActorID                  uuid.UUID
	BindingID                uuid.UUID
	DeviceID                 uuid.UUID
	OperationID              uuid.UUID
	AuthorityRevision        int64
	ChatID                   uuid.UUID
	MessageID                uuid.UUID
	Revision                 int64
	PreviousRevisionHash     string
	IssuedAt                 time.Time
	ExpiresAt                time.Time
	Content                  []byte
	ContentSHA256            string
	Attachments              []Attachment
	AttachmentManifestSHA256 string
	RawPayload               []byte
}

type DeviceAuthority struct {
	AssertionJWS      string
	AssertionJTI      uuid.UUID
	KeyID             uuid.UUID
	ApplicationID     uuid.UUID
	EnvironmentID     uuid.UUID
	AccountID         uuid.UUID
	ActorID           uuid.UUID
	BindingID         uuid.UUID
	DeviceID          uuid.UUID
	PublicJWK         map[string]any
	KeyThumbprint     string
	DeviceGeneration  int64
	AuthorityRevision int64
	NotAfter          time.Time
	IssuedAt          time.Time
	ExpiresAt         time.Time
}

// AuthKeySnapshot is a recent complete principal JWKS view used only to verify
// Auth-owned device authority assertions at the Messaging receiver.
type AuthKeySnapshot struct {
	Keys             map[string]*rsa.PublicKey
	RefreshedAt      time.Time
	ClockUncertainty time.Duration
}

type ReceiptKey struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID   uuid.UUID
	ChatID        uuid.UUID
	MessageID     uuid.UUID
	Revision      int64
	Compact       string
}

// ExtractDeviceAuthorityExpected reads only the exact identity claims needed
// to couple the two signed envelopes. The caller must still verify this Auth
// signature with VerifyDeviceAuthority before trusting any returned value.
func ExtractDeviceAuthorityExpected(compact string) (Expected, error) {
	var expected Expected
	parts := strings.Split(compact, ".")
	if len(parts) != 3 || len(compact) > 16*1024 {
		return expected, errors.New("invalid Auth device authority JWS")
	}
	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return expected, err
	}
	headerValue, err := parseStrictJSON(headerBytes)
	if err != nil {
		return expected, err
	}
	header, ok := headerValue.(map[string]any)
	if !ok || !exactKeys(header, map[string]struct{}{"alg": {}, "kid": {}, "typ": {}}) ||
		header["alg"] != "RS256" || header["typ"] != "voice.game-device-status+jwt" {
		return expected, errors.New("invalid Auth device authority header")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return expected, err
	}
	payloadValue, err := parseStrictJSON(payloadBytes)
	if err != nil {
		return expected, err
	}
	claims, ok := payloadValue.(map[string]any)
	fields := map[string]struct{}{
		"version": {}, "iss": {}, "aud": {}, "jti": {}, "application_id": {}, "environment_id": {},
		"account_id": {}, "actor_id": {}, "binding_id": {}, "device_id": {}, "key_id": {}, "public_jwk": {},
		"key_thumbprint": {}, "device_generation": {}, "authority_revision": {}, "status": {}, "not_after": {}, "iat": {}, "exp": {},
	}
	if !ok || !exactKeys(claims, fields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) || claims["iss"] != "auth" || claims["aud"] != messageAudience || claims["status"] != "active" {
		return expected, errors.New("invalid Auth device authority claims")
	}
	readID := func(name string) (string, error) {
		text, ok := claims[name].(string)
		if !ok {
			return "", fmt.Errorf("invalid Auth %s", name)
		}
		id, err := canonicalUUID(text)
		if err != nil {
			return "", err
		}
		return id.String(), nil
	}
	if expected.ApplicationID, err = readID("application_id"); err != nil {
		return expected, err
	}
	if expected.EnvironmentID, err = readID("environment_id"); err != nil {
		return expected, err
	}
	if expected.AccountID, err = readID("account_id"); err != nil {
		return expected, err
	}
	if expected.ActorID, err = readID("actor_id"); err != nil {
		return expected, err
	}
	if expected.BindingID, err = readID("binding_id"); err != nil {
		return expected, err
	}
	if expected.DeviceID, err = readID("device_id"); err != nil {
		return expected, err
	}
	return expected, nil
}

// VerifyMessage parses strict canonical game-message JWS and verifies its P-256 signature.
// Auth status assertion verification and durable receipt handling are separate receiver stages.
func VerifyMessage(compact string, publicJWK map[string]any, now time.Time, expected Expected) (Message, error) {
	var zero Message
	if compact == "" || len(compact) > 128*1024 {
		return zero, errors.New("invalid game message")
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return zero, errors.New("invalid compact JWS")
	}
	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return zero, err
	}
	headerValue, err := parseStrictJSON(headerBytes)
	if err != nil {
		return zero, err
	}
	header, ok := headerValue.(map[string]any)
	if !ok || len(header) != 3 || header["alg"] != "ES256" || header["typ"] != messageType {
		return zero, errors.New("invalid game message protected header")
	}
	keyIDText, ok := header["kid"].(string)
	if !ok {
		return zero, errors.New("missing device key id")
	}
	keyID, err := canonicalUUID(keyIDText)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(headerBytes, canonicalJSON(header)) {
		return zero, errors.New("noncanonical protected header")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return zero, err
	}
	payloadValue, err := parseStrictJSON(payloadBytes)
	if err != nil {
		return zero, err
	}
	claims, ok := payloadValue.(map[string]any)
	if !ok || !exactKeys(claims, messageFields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) {
		return zero, errors.New("invalid or noncanonical game message payload")
	}
	key, err := parseP256JWK(publicJWK)
	if err != nil {
		return zero, err
	}
	signature, err := decodeSegment(parts[2])
	if err != nil || len(signature) != 64 {
		return zero, errors.New("invalid ES256 signature")
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(key, hash[:], r, s) {
		return zero, errors.New("invalid device signature")
	}
	message, err := parseMessageClaims(claims, compact, keyID, payloadBytes, now.UTC(), true)
	if err != nil {
		return zero, err
	}
	if !matchesExpected(message, expected) {
		return zero, errors.New("message route or identity mismatch")
	}
	return message, nil
}

// ExtractReceiptKey strictly parses the immutable payload without checking signature or time.
// It is used only for read-only receipt lookup before freshness and authority verification.
func ExtractReceiptKey(compact string) (ReceiptKey, error) {
	var result ReceiptKey
	if compact == "" || len(compact) > 128*1024 {
		return result, errors.New("invalid game message")
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return result, errors.New("invalid compact JWS")
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
	if !ok || len(header) != 3 || header["alg"] != "ES256" || header["typ"] != messageType ||
		!bytes.Equal(headerBytes, canonicalJSON(header)) {
		return result, errors.New("invalid game message protected header")
	}
	keyText, ok := header["kid"].(string)
	if !ok {
		return result, errors.New("missing device key ID")
	}
	keyID, err := canonicalUUID(keyText)
	if err != nil {
		return result, err
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
	if !ok || !exactKeys(claims, messageFields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) {
		return result, errors.New("invalid game message payload")
	}
	message, err := parseMessageClaims(claims, compact, keyID, payloadBytes, time.Time{}, false)
	if err != nil {
		return result, err
	}
	return ReceiptKey{ApplicationID: message.ApplicationID, EnvironmentID: message.EnvironmentID,
		OperationID: message.OperationID, ChatID: message.ChatID, MessageID: message.MessageID,
		Revision: message.Revision, Compact: compact}, nil
}

// VerifyRequest validates the Auth RS256 device status first, then uses only its embedded device key.
// The caller supplies keys obtained from the fixed, mutually-authenticated Auth JWKS origin.
func VerifyRequest(compactMessage, compactAuthority string, authKeys map[string]*rsa.PublicKey,
	now time.Time, clockUncertainty time.Duration, expected Expected) (Message, DeviceAuthority, error) {
	authority, err := VerifyDeviceAuthority(compactAuthority, authKeys, now, clockUncertainty, expected)
	if err != nil {
		return Message{}, DeviceAuthority{}, err
	}
	message, err := VerifyMessage(compactMessage, authority.PublicJWK, now, expected)
	if err != nil {
		return Message{}, DeviceAuthority{}, err
	}
	if message.KeyID != authority.KeyID {
		return Message{}, DeviceAuthority{}, errors.New("device key does not match authority assertion")
	}
	return message, authority, nil
}

func VerifyDeviceAuthority(compact string, authKeys map[string]*rsa.PublicKey, now time.Time,
	clockUncertainty time.Duration, expected Expected) (DeviceAuthority, error) {
	var result DeviceAuthority
	if compact == "" || len(compact) > 16*1024 || clockUncertainty < 0 || clockUncertainty > 250*time.Millisecond {
		return result, errors.New("invalid Auth device authority")
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return result, errors.New("invalid Auth compact JWS")
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
		header["alg"] != "RS256" || header["typ"] != "voice.game-device-status+jwt" {
		return result, errors.New("invalid Auth protected header")
	}
	kid, ok := header["kid"].(string)
	if !ok || kid == "" {
		return result, errors.New("missing Auth key ID")
	}
	key := authKeys[kid]
	if key == nil {
		return result, errors.New("unknown Auth signing key")
	}
	signature, err := decodeSegment(parts[2])
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return result, errors.New("invalid Auth device authority signature")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return result, err
	}
	payload, err := parseStrictJSON(payloadBytes)
	if err != nil {
		return result, err
	}
	claims, ok := payload.(map[string]any)
	authorityFields := map[string]struct{}{
		"version": {}, "iss": {}, "aud": {}, "jti": {}, "application_id": {}, "environment_id": {},
		"account_id": {}, "actor_id": {}, "binding_id": {}, "device_id": {}, "key_id": {}, "public_jwk": {},
		"key_thumbprint": {}, "device_generation": {}, "authority_revision": {}, "status": {},
		"not_after": {}, "iat": {}, "exp": {},
	}
	if !ok || !exactKeys(claims, authorityFields) || !bytes.Equal(payloadBytes, canonicalJSON(claims)) ||
		claims["iss"] != "auth" || claims["aud"] != messageAudience || claims["status"] != "active" {
		return result, errors.New("invalid Auth device authority claims")
	}
	version, err := integer(claims["version"], 1, 1)
	if err != nil {
		return result, err
	}
	_ = version
	if result.AssertionJTI, err = canonicalUUID(asString(claims["jti"])); err != nil {
		return result, err
	}
	parseID := func(name string) (uuid.UUID, error) { return canonicalUUID(asString(claims[name])) }
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
	if result.DeviceID, err = parseID("device_id"); err != nil {
		return result, err
	}
	if result.KeyID, err = parseID("key_id"); err != nil {
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
	issued, err := integer(claims["iat"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	expires, err := integer(claims["exp"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	notAfter, err := integer(claims["not_after"], 0, maxSafeInteger)
	if err != nil {
		return result, err
	}
	expectedExpiry := issued + 4000
	if notAfter < expectedExpiry {
		expectedExpiry = notAfter
	}
	if expires != expectedExpiry || expires <= issued || notAfter <= issued {
		return result, errors.New("invalid Auth assertion lifetime")
	}
	nowMS := now.UTC().UnixMilli()
	if issued > nowMS+250 || nowMS >= expires-250 || nowMS >= notAfter {
		return result, errors.New("expired or not-yet-valid Auth assertion")
	}
	result.IssuedAt = time.UnixMilli(issued).UTC()
	result.ExpiresAt = time.UnixMilli(expires).UTC()
	result.NotAfter = time.UnixMilli(notAfter).UTC()
	result.KeyThumbprint, ok = claims["key_thumbprint"].(string)
	if !ok || len(result.KeyThumbprint) != 43 {
		return result, errors.New("invalid device key thumbprint")
	}
	result.PublicJWK, ok = claims["public_jwk"].(map[string]any)
	if !ok {
		return result, errors.New("missing device public JWK")
	}
	if _, err := parseP256JWK(result.PublicJWK); err != nil {
		return result, err
	}
	thumbprint, err := p256Thumbprint(result.PublicJWK)
	if err != nil || thumbprint != result.KeyThumbprint {
		return result, errors.New("device JWK thumbprint mismatch")
	}
	for _, pair := range []struct {
		got  uuid.UUID
		want string
	}{
		{result.ApplicationID, expected.ApplicationID}, {result.EnvironmentID, expected.EnvironmentID},
		{result.AccountID, expected.AccountID}, {result.ActorID, expected.ActorID},
		{result.BindingID, expected.BindingID}, {result.DeviceID, expected.DeviceID},
	} {
		want, err := canonicalUUID(pair.want)
		if err != nil || pair.got != want {
			return result, errors.New("auth assertion identity mismatch")
		}
	}
	result.AssertionJWS = compact
	return result, nil
}

func p256Thumbprint(jwk map[string]any) (string, error) {
	key, err := parseP256JWK(jwk)
	if err != nil {
		return "", err
	}
	x := base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32)))
	y := base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))
	canonical := []byte(`{"crv":"P-256","kty":"EC","x":"` + x + `","y":"` + y + `"}`)
	digest := sha256.Sum256(canonical)
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func asString(value any) string { text, _ := value.(string); return text }

func parseMessageClaims(c map[string]any, compact string, keyID uuid.UUID, payload []byte, now time.Time,
	checkFreshness bool) (Message, error) {
	var m Message
	m.Compact, m.KeyID, m.RawPayload = compact, keyID, append([]byte(nil), payload...)
	version, err := integer(c["version"], 1, 1)
	if err != nil {
		return m, err
	}
	_ = version
	if c["audience"] != messageAudience {
		return m, errors.New("wrong message audience")
	}
	operation, ok := c["operation"].(string)
	if !ok || (operation != "create" && operation != "edit" && operation != "delete") {
		return m, errors.New("unknown message operation")
	}
	m.Operation = operation
	ids := []struct {
		name string
		dst  *uuid.UUID
	}{
		{"application_id", &m.ApplicationID}, {"environment_id", &m.EnvironmentID}, {"account_id", &m.AccountID},
		{"actor_id", &m.ActorID}, {"binding_id", &m.BindingID}, {"device_id", &m.DeviceID},
		{"operation_id", &m.OperationID}, {"chat_id", &m.ChatID}, {"message_id", &m.MessageID},
	}
	for _, item := range ids {
		text, ok := c[item.name].(string)
		if !ok {
			return m, fmt.Errorf("invalid %s", item.name)
		}
		id, err := canonicalUUID(text)
		if err != nil {
			return m, fmt.Errorf("invalid %s", item.name)
		}
		*item.dst = id
	}
	if m.MessageID.Version() != 7 {
		return m, errors.New("message ID must be UUIDv7")
	}
	m.AuthorityRevision, err = integer(c["authority_revision"], 1, maxSafeInteger)
	if err != nil {
		return m, err
	}
	m.Revision, err = integer(c["revision"], 1, maxSafeInteger)
	if err != nil {
		return m, err
	}
	if operation == "create" {
		if m.Revision != 1 || c["previous_revision_hash"] != nil {
			return m, errors.New("invalid create revision")
		}
	} else {
		if m.Revision < 2 {
			return m, errors.New("invalid mutation revision")
		}
		previous, ok := c["previous_revision_hash"].(string)
		if !ok || !isLowerHexDigest(previous) {
			return m, errors.New("invalid previous revision hash")
		}
		m.PreviousRevisionHash = previous
	}
	issued, err := rfc3339Second(c["issued_at"])
	if err != nil {
		return m, err
	}
	expires, err := rfc3339Second(c["expires_at"])
	if err != nil || !issued.Before(expires) || expires.Sub(issued) > 5*time.Minute || checkFreshness &&
		(issued.After(now.Add(30*time.Second)) || now.Before(issued.Add(-30*time.Second)) || !now.Before(expires)) {
		return m, errors.New("invalid message envelope lifetime")
	}
	m.IssuedAt, m.ExpiresAt = issued, expires
	if operation == "delete" {
		for _, field := range []string{"content_type", "content_b64", "content_sha256", "attachment_manifest_b64", "attachment_manifest_sha256"} {
			if c[field] != nil {
				return m, errors.New("delete must not contain content or attachments")
			}
		}
		return m, nil
	}
	if c["content_type"] != "text/plain" {
		return m, errors.New("unsupported content type")
	}
	encoded, ok := c["content_b64"].(string)
	if !ok {
		return m, errors.New("missing content bytes")
	}
	content, err := decodeURLBase64(encoded)
	if err != nil || !utf8.Valid(content) || utf8.RuneCount(content) > 4000 {
		return m, errors.New("invalid message content")
	}
	digest := sha256.Sum256(content)
	digestText, ok := c["content_sha256"].(string)
	if !ok || digestText != hex.EncodeToString(digest[:]) {
		return m, errors.New("message content digest mismatch")
	}
	m.Content, m.ContentSHA256 = content, digestText
	manifest, manifestDigest := c["attachment_manifest_b64"], c["attachment_manifest_sha256"]
	if manifest == nil || manifestDigest == nil {
		if manifest != nil || manifestDigest != nil {
			return m, errors.New("incomplete attachment manifest")
		}
		return m, nil
	}
	manifestText, ok := manifest.(string)
	if !ok {
		return m, errors.New("invalid attachment manifest")
	}
	manifestBytes, err := decodeURLBase64(manifestText)
	if err != nil {
		return m, err
	}
	manifestHash := sha256.Sum256(manifestBytes)
	hashText, ok := manifestDigest.(string)
	if !ok || hashText != hex.EncodeToString(manifestHash[:]) {
		return m, errors.New("attachment manifest digest mismatch")
	}
	if err := parseAttachments(manifestBytes, &m); err != nil {
		return m, err
	}
	m.AttachmentManifestSHA256 = hashText
	return m, nil
}

func parseAttachments(raw []byte, result *Message) error {
	value, err := parseStrictJSON(raw)
	if err != nil || !bytes.Equal(raw, canonicalJSON(value)) {
		return errors.New("attachment manifest is not canonical JCS")
	}
	items, ok := value.([]any)
	if !ok {
		return errors.New("attachment manifest must be an array")
	}
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok || !exactKeys(item, map[string]struct{}{"file_id": {}, "object_revision": {}, "byte_length": {}, "content_sha256": {}, "media_type": {}}) {
			return errors.New("invalid attachment record")
		}
		fileText, ok := item["file_id"].(string)
		fileID, idErr := canonicalUUID(fileText)
		revision, revErr := integer(item["object_revision"], 0, maxSafeInteger)
		length, lenErr := integer(item["byte_length"], 0, maxSafeInteger)
		digest, digestOK := item["content_sha256"].(string)
		mediaType, mediaOK := item["media_type"].(string)
		if !ok || idErr != nil || revErr != nil || lenErr != nil || !digestOK || !isLowerHexDigest(digest) ||
			!mediaOK || mediaType == "" || strings.ContainsAny(mediaType, "\r\n") {
			return errors.New("invalid attachment record")
		}
		result.Attachments = append(result.Attachments, Attachment{FileID: fileID, ObjectRevision: revision,
			ByteLength: length, ContentSHA256: digest, MediaType: mediaType})
	}
	return nil
}

func matchesExpected(m Message, expected Expected) bool {
	for _, pair := range []struct {
		got  uuid.UUID
		want string
	}{
		{m.ApplicationID, expected.ApplicationID}, {m.EnvironmentID, expected.EnvironmentID},
		{m.AccountID, expected.AccountID}, {m.ActorID, expected.ActorID}, {m.BindingID, expected.BindingID},
		{m.DeviceID, expected.DeviceID}, {m.ChatID, expected.ChatID},
	} {
		want, err := canonicalUUID(pair.want)
		if err != nil || pair.got != want {
			return false
		}
	}
	return true
}

func parseP256JWK(jwk map[string]any) (*ecdsa.PublicKey, error) {
	if !exactKeys(jwk, map[string]struct{}{"kty": {}, "crv": {}, "x": {}, "y": {}}) ||
		jwk["kty"] != "EC" || jwk["crv"] != "P-256" {
		return nil, errors.New("invalid Auth device JWK")
	}
	xText, xOK := jwk["x"].(string)
	yText, yOK := jwk["y"].(string)
	if !xOK || !yOK {
		return nil, errors.New("invalid Auth device JWK")
	}
	xBytes, xErr := decodeURLBase64(xText)
	yBytes, yErr := decodeURLBase64(yText)
	if xErr != nil || yErr != nil || len(xBytes) != 32 || len(yBytes) != 32 {
		return nil, errors.New("invalid Auth device JWK")
	}
	key := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}
	if !key.IsOnCurve(key.X, key.Y) {
		return nil, errors.New("invalid Auth device JWK")
	}
	return key, nil
}

func parseStrictJSON(data []byte) (any, error) {
	if len(data) == 0 || !utf8.Valid(data) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("invalid UTF-8 JSON")
	}
	if hasUnpairedSurrogateEscape(data) {
		return nil, errors.New("invalid Unicode surrogate escape")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON data")
	}
	return value, nil
}

// encoding/json replaces lone UTF-16 surrogate escapes with U+FFFD. JCS
// requires invalid Unicode data to be rejected instead of silently changed.
func hasUnpairedSurrogateEscape(data []byte) bool {
	inString := false
	for i := 0; i < len(data); {
		if !inString {
			if data[i] == '"' {
				inString = true
			}
			i++
			continue
		}
		switch data[i] {
		case '"':
			inString = false
			i++
		case '\\':
			if i+1 >= len(data) {
				return false // The JSON decoder reports the malformed escape.
			}
			if data[i+1] != 'u' {
				i += 2
				continue
			}
			if i+6 > len(data) {
				return false // The JSON decoder reports the truncated escape.
			}
			code, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
			if err != nil {
				i += 6
				continue
			}
			switch {
			case code >= 0xd800 && code <= 0xdbff:
				if i+12 > len(data) || data[i+6] != '\\' || data[i+7] != 'u' {
					return true
				}
				low, err := strconv.ParseUint(string(data[i+8:i+12]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return true
				}
				i += 12
			case code >= 0xdc00 && code <= 0xdfff:
				return true
			default:
				i += 6
			}
		default:
			i++
		}
	}
	return false
}

func readValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			object := map[string]any{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("invalid JSON object key")
				}
				if _, exists := object[key]; exists {
					return nil, errors.New("duplicate JSON object key")
				}
				child, err := readValue(decoder)
				if err != nil {
					return nil, err
				}
				object[key] = child
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				child, err := readValue(decoder)
				if err != nil {
					return nil, err
				}
				array = append(array, child)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return array, nil
		default:
			return nil, errors.New("unexpected JSON delimiter")
		}
	default:
		return token, nil
	}
}

func canonicalJSON(value any) []byte {
	var out bytes.Buffer
	writeCanonical(&out, value)
	return out.Bytes()
}

func writeCanonical(out *bytes.Buffer, value any) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return compareUTF16(keys[i], keys[j]) < 0 })
		out.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				out.WriteByte(',')
			}
			writeJSONString(out, key)
			out.WriteByte(':')
			writeCanonical(out, v[key])
		}
		out.WriteByte('}')
	case []any:
		out.WriteByte('[')
		for i, child := range v {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonical(out, child)
		}
		out.WriteByte(']')
	case string:
		writeJSONString(out, v)
	case json.Number:
		out.WriteString(canonicalNumber(v))
	case bool:
		if v {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case nil:
		out.WriteString("null")
	default:
		out.WriteString("null")
	}
}

// RFC 8785 sorts property names lexicographically by their UTF-16 code units,
// matching ECMAScript's string ordering (which differs from UTF-8/code-point
// order for supplementary characters).
func compareUTF16(left, right string) int {
	a, b := utf16.Encode([]rune(left)), utf16.Encode([]rune(right))
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

// canonicalNumber applies ECMAScript Number::toString formatting to a parsed
// binary64 value, as required by JCS. strconv supplies the shortest round-trip
// digits; this function applies ECMAScript's decimal/exponent cutovers.
func canonicalNumber(number json.Number) string {
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return ""
	}
	if value == 0 {
		return "0"
	}
	negative := math.Signbit(value)
	if negative {
		value = -value
	}
	text := strconv.FormatFloat(value, 'g', -1, 64)
	exponent := 0
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		parsed, err := strconv.Atoi(text[index+1:])
		if err != nil {
			return ""
		}
		exponent = parsed
		text = text[:index]
	}
	dot := strings.IndexByte(text, '.')
	if dot < 0 {
		dot = len(text)
	} else {
		text = text[:dot] + text[dot+1:]
	}
	digits := strings.TrimLeft(text, "0")
	decimalPoint := dot + exponent
	if digits == "" {
		return "0"
	}
	decimalPoint -= len(text) - len(digits)
	for len(digits) > 1 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
	}
	var result string
	switch {
	case len(digits) <= decimalPoint && decimalPoint <= 21:
		result = digits + strings.Repeat("0", decimalPoint-len(digits))
	case 0 < decimalPoint && decimalPoint <= 21:
		result = digits[:decimalPoint] + "." + digits[decimalPoint:]
	case -6 < decimalPoint && decimalPoint <= 0:
		result = "0." + strings.Repeat("0", -decimalPoint) + digits
	default:
		sign := "+"
		exponent = decimalPoint - 1
		if exponent < 0 {
			sign = ""
		}
		result = digits[:1]
		if len(digits) > 1 {
			result += "." + digits[1:]
		}
		result += "e" + sign + strconv.Itoa(exponent)
	}
	if negative {
		return "-" + result
	}
	return result
}

func writeJSONString(out *bytes.Buffer, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
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
			if r < 0x20 {
				const digits = "0123456789abcdef"
				out.WriteString(`\u`)
				out.WriteByte(digits[(r>>12)&0xf])
				out.WriteByte(digits[(r>>8)&0xf])
				out.WriteByte(digits[(r>>4)&0xf])
				out.WriteByte(digits[r&0xf])
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

func exactKeys(value map[string]any, allowed map[string]struct{}) bool {
	if len(value) != len(allowed) {
		return false
	}
	for key := range value {
		if _, ok := allowed[key]; !ok {
			return false
		}
	}
	return true
}

func decodeSegment(value string) ([]byte, error) { return decodeURLBase64(value) }

func decodeURLBase64(value string) ([]byte, error) {
	if strings.Contains(value, "=") {
		return nil, errors.New("invalid unpadded base64url")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, errors.New("invalid unpadded base64url")
	}
	return decoded, nil
}

func canonicalUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id.String() != value {
		return uuid.Nil, errors.New("noncanonical UUID")
	}
	return id, nil
}

func integer(value any, minimum, maximum int64) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, errors.New("JSON integer required")
	}
	parsed, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || strconv.FormatInt(parsed, 10) != number.String() || parsed < minimum || parsed > maximum {
		return 0, errors.New("JSON integer out of range or noncanonical")
	}
	return parsed, nil
}

func rfc3339Second(value any) (time.Time, error) {
	text, ok := value.(string)
	if !ok || !strings.HasSuffix(text, "Z") || strings.Contains(text, ".") {
		return time.Time{}, errors.New("RFC3339 UTC seconds required")
	}
	parsed, err := time.Parse("2006-01-02T15:04:05Z", text)
	if err != nil || parsed.Format("2006-01-02T15:04:05Z") != text {
		return time.Time{}, errors.New("invalid RFC3339 UTC time")
	}
	return parsed.UTC(), nil
}

func isLowerHexDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
