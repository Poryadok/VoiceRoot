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
)

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
	// OTPK fields are optional after consumption.
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
	return w, nil
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
	if w.PreKeyID <= 0 {
		return errors.New("prekey: pre_key_id required on upload")
	}
	if err := validateCurvePoint(w.PreKeyPublic, "pre_key_public"); err != nil {
		return err
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

// HasOTPK reports whether the bundle still exposes a one-time pre-key.
func (w *Wire) HasOTPK() bool {
	return w != nil && w.PreKeyID > 0 && len(w.PreKeyPublic) == curvePointLen
}

// ConsumeOTPK returns a copy without one-time pre-key fields.
func ConsumeOTPK(w *Wire) (*Wire, error) {
	if w == nil {
		return nil, errors.New("prekey: bundle is nil")
	}
	out := *w
	out.PreKeyID = 0
	out.PreKeyPublic = nil
	return &out, nil
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
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
