package prekey

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	curvePointLen = 33
	signatureLen  = 64
	minOTPKPool   = 1
)

// OTPKEntry is one one-time pre-key in the server-side pool.
type OTPKEntry struct {
	ID     int
	Public []byte
}

// Wire is the parsed Signal pre-key bundle uploaded by the Flutter client
// (see src/frontend/lib/e2e/e2e_store_factory.dart serializePreKeyBundle).
type Wire struct {
	RegistrationID        int
	DeviceID              int
	PreKeyID              int
	PreKeyPublic          []byte
	SignedPreKeyID        int
	SignedPreKeyPublic    []byte
	SignedPreKeySignature []byte
	IdentityKey           []byte
	OTPKPool              []OTPKEntry
}

// ParseWire decodes the outer base64 JSON envelope.
func ParseWire(outerB64 string) (*Wire, error) {
	raw, err := base64.StdEncoding.DecodeString(outerB64)
	if err != nil {
		return nil, fmt.Errorf("prekey: invalid base64: %w", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("prekey: invalid json: %w", err)
	}
	w := &Wire{}
	if err := decodeInt(payload, "registration_id", &w.RegistrationID); err != nil {
		return nil, err
	}
	if err := decodeInt(payload, "device_id", &w.DeviceID); err != nil {
		return nil, err
	}
	if err := decodeInt(payload, "signed_pre_key_id", &w.SignedPreKeyID); err != nil {
		return nil, err
	}
	if err := decodeBytes(payload, "signed_pre_key_public", &w.SignedPreKeyPublic); err != nil {
		return nil, err
	}
	if err := decodeBytes(payload, "signed_pre_key_signature", &w.SignedPreKeySignature); err != nil {
		return nil, err
	}
	if err := decodeBytes(payload, "identity_key", &w.IdentityKey); err != nil {
		return nil, err
	}
	if rawPool, ok := payload["pre_keys"]; ok {
		pool, err := decodeOTPKPool(rawPool)
		if err != nil {
			return nil, err
		}
		w.OTPKPool = pool
	}
	// Legacy / active OTPK fields (optional after consumption).
	if rawID, ok := payload["pre_key_id"]; ok {
		var id int
		if err := json.Unmarshal(rawID, &id); err != nil {
			return nil, fmt.Errorf("prekey: pre_key_id: %w", err)
		}
		w.PreKeyID = id
	}
	if rawPub, ok := payload["pre_key_public"]; ok {
		var encoded string
		if err := json.Unmarshal(rawPub, &encoded); err != nil {
			return nil, fmt.Errorf("prekey: pre_key_public: %w", err)
		}
		b, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("prekey: pre_key_public base64: %w", err)
		}
		w.PreKeyPublic = b
	}
	if len(w.OTPKPool) == 0 && w.HasOTPK() {
		w.OTPKPool = []OTPKEntry{{ID: w.PreKeyID, Public: append([]byte(nil), w.PreKeyPublic...)}}
	}
	return w, nil
}

func decodeOTPKPool(raw json.RawMessage) ([]OTPKEntry, error) {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("prekey: pre_keys: %w", err)
	}
	out := make([]OTPKEntry, 0, len(items))
	for i, item := range items {
		var id int
		if err := decodeInt(item, "pre_key_id", &id); err != nil {
			return nil, fmt.Errorf("prekey: pre_keys[%d]: %w", i, err)
		}
		var pub []byte
		if err := decodeBytes(item, "pre_key_public", &pub); err != nil {
			return nil, fmt.Errorf("prekey: pre_keys[%d]: %w", i, err)
		}
		if err := validateCurvePoint(pub, "pre_key_public"); err != nil {
			return nil, err
		}
		out = append(out, OTPKEntry{ID: id, Public: pub})
	}
	return out, nil
}

func decodeInt(payload map[string]json.RawMessage, key string, out *int) error {
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("prekey: missing %s", key)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("prekey: %s: %w", key, err)
	}
	return nil
}

func decodeBytes(payload map[string]json.RawMessage, key string, out *[]byte) error {
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("prekey: missing %s", key)
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return fmt.Errorf("prekey: %s: %w", key, err)
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("prekey: %s base64: %w", key, err)
	}
	*out = b
	return nil
}

// ValidateForUpload checks structural requirements for a fresh bundle upload.
func ValidateForUpload(w *Wire) error {
	if w == nil {
		return errors.New("prekey: bundle is nil")
	}
	if w.RegistrationID <= 0 {
		return errors.New("prekey: registration_id must be positive")
	}
	if w.DeviceID <= 0 {
		return errors.New("prekey: device_id must be positive")
	}
	if w.OTPKPoolSize() < minOTPKPool {
		return errors.New("prekey: pre_key_id required on upload")
	}
	if w.SignedPreKeyID <= 0 {
		return errors.New("prekey: signed_pre_key_id must be positive")
	}
	if err := validateCurvePoint(w.SignedPreKeyPublic, "signed_pre_key_public"); err != nil {
		return err
	}
	if len(w.SignedPreKeySignature) != signatureLen {
		return fmt.Errorf("prekey: signed_pre_key_signature must be %d bytes", signatureLen)
	}
	if err := validateCurvePoint(w.IdentityKey, "identity_key"); err != nil {
		return err
	}
	for i, entry := range w.OTPKPool {
		if entry.ID <= 0 {
			return fmt.Errorf("prekey: pre_keys[%d] id must be positive", i)
		}
		if err := validateCurvePoint(entry.Public, "pre_key_public"); err != nil {
			return fmt.Errorf("prekey: pre_keys[%d]: %w", i, err)
		}
	}
	return nil
}

// VerifySignedPreKeySignature checks the libsignal signed-pre-key signature
// (Curve25519 identity key signing the serialized signed-pre-key public bytes).
func VerifySignedPreKeySignature(w *Wire) error {
	if w == nil {
		return errors.New("prekey: bundle is nil")
	}
	if err := validateCurvePoint(w.IdentityKey, "identity_key"); err != nil {
		return err
	}
	if err := validateCurvePoint(w.SignedPreKeyPublic, "signed_pre_key_public"); err != nil {
		return err
	}
	if len(w.SignedPreKeySignature) != signatureLen {
		return fmt.Errorf("prekey: signed_pre_key_signature must be %d bytes", signatureLen)
	}
	if !verifyCurve25519Signature(w.IdentityKey[1:], w.SignedPreKeyPublic, w.SignedPreKeySignature) {
		return errors.New("prekey: invalid signed pre-key signature")
	}
	return nil
}

func validateCurvePoint(b []byte, field string) error {
	if len(b) != curvePointLen {
		return fmt.Errorf("prekey: %s must be %d bytes", field, curvePointLen)
	}
	if b[0] != 0x05 {
		return fmt.Errorf("prekey: %s must be an uncompressed Curve25519 point", field)
	}
	return nil
}

// OTPKPoolSize returns remaining one-time pre-keys in the pool.
func (w *Wire) OTPKPoolSize() int {
	if w == nil {
		return 0
	}
	return len(w.OTPKPool)
}

// HasOTPK reports whether the bundle still exposes a one-time pre-key for fetch.
func (w *Wire) HasOTPK() bool {
	return w != nil && w.PreKeyID > 0 && len(w.PreKeyPublic) == curvePointLen
}

// PopOTPKForFetch removes the next OTPK from the pool and returns a response bundle (with public key)
// plus the persisted wire (pool without the consumed key).
func PopOTPKForFetch(w *Wire) (response *Wire, stored *Wire, err error) {
	if w == nil {
		return nil, nil, errors.New("prekey: bundle is nil")
	}
	if len(w.OTPKPool) == 0 {
		resp := w.cloneShallow()
		resp.PreKeyID = 0
		resp.PreKeyPublic = nil
		return resp, w.cloneShallow(), nil
	}
	entry := w.OTPKPool[0]
	remaining := append([]OTPKEntry(nil), w.OTPKPool[1:]...)

	response = w.cloneShallow()
	response.PreKeyID = entry.ID
	response.PreKeyPublic = append([]byte(nil), entry.Public...)
	response.OTPKPool = remaining

	stored = w.cloneShallow()
	stored.PreKeyID = 0
	stored.PreKeyPublic = nil
	stored.OTPKPool = remaining
	return response, stored, nil
}

// ConsumeNextOTPKFromPool advances stored pool state (unit-test helper for persistence transitions).
func ConsumeNextOTPKFromPool(w *Wire) (*Wire, error) {
	_, stored, err := PopOTPKForFetch(w)
	if err != nil {
		return nil, err
	}
	if len(w.OTPKPool) > 0 {
		stored.PreKeyID = w.OTPKPool[0].ID
	}
	return stored, nil
}

// ConsumeOTPK returns a copy without one-time pre-key fields (legacy single-key path).
func ConsumeOTPK(w *Wire) (*Wire, error) {
	if w == nil {
		return nil, errors.New("prekey: bundle is nil")
	}
	out := w.cloneShallow()
	out.PreKeyID = 0
	out.PreKeyPublic = nil
	out.OTPKPool = nil
	return out, nil
}

func (w *Wire) cloneShallow() *Wire {
	if w == nil {
		return nil
	}
	out := *w
	if len(w.OTPKPool) > 0 {
		out.OTPKPool = append([]OTPKEntry(nil), w.OTPKPool...)
	}
	return &out
}

// EncodeWire serializes the bundle to the client wire format.
func EncodeWire(w *Wire) (string, error) {
	if w == nil {
		return "", errors.New("prekey: bundle is nil")
	}
	payload := map[string]any{
		"registration_id":          w.RegistrationID,
		"device_id":                w.DeviceID,
		"signed_pre_key_id":        w.SignedPreKeyID,
		"signed_pre_key_public":    base64.StdEncoding.EncodeToString(w.SignedPreKeyPublic),
		"signed_pre_key_signature": base64.StdEncoding.EncodeToString(w.SignedPreKeySignature),
		"identity_key":             base64.StdEncoding.EncodeToString(w.IdentityKey),
	}
	if w.HasOTPK() {
		payload["pre_key_id"] = w.PreKeyID
		payload["pre_key_public"] = base64.StdEncoding.EncodeToString(w.PreKeyPublic)
	}
	if len(w.OTPKPool) > 0 {
		items := make([]map[string]any, 0, len(w.OTPKPool))
		for _, entry := range w.OTPKPool {
			items = append(items, map[string]any{
				"pre_key_id":     entry.ID,
				"pre_key_public": base64.StdEncoding.EncodeToString(entry.Public),
			})
		}
		payload["pre_keys"] = items
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
