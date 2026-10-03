package grpcsvc

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
)

const minimumDownloadCapabilityKeyBytes = 32

type revocableDownloadCapability struct {
	ProfileID uuid.UUID
	FileID    uuid.UUID
	Variant   filev1.FileURLVariant
	Access    *filev1.FileAccessSelector
	ExpiresAt time.Time
}

type revocableDownloadPayload struct {
	ProfileID string `json:"profile_id"`
	FileID    string `json:"file_id"`
	Variant   int32  `json:"variant"`
	Access    string `json:"access"`
	ExpiresAt int64  `json:"expires_at_unix_nano"`
}

func signRevocableDownloadCapability(capability revocableDownloadCapability, key []byte) (string, error) {
	if len(key) < minimumDownloadCapabilityKeyBytes || capability.ProfileID == uuid.Nil ||
		capability.FileID == uuid.Nil || capability.ExpiresAt.IsZero() {
		return "", errors.New("invalid download capability")
	}
	if _, err := fileURLKeyVariantOnly(capability.Variant); err != nil {
		return "", err
	}
	if capability.Access == nil {
		return "", errors.New("download capability requires exact access selector")
	}
	if err := validateMessageDownloadSelector(capability.FileID, capability.Access); err != nil {
		return "", err
	}
	accessBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(capability.Access)
	if err != nil || len(accessBytes) == 0 {
		return "", errors.New("invalid download access selector")
	}
	payload, err := json.Marshal(revocableDownloadPayload{
		ProfileID: capability.ProfileID.String(), FileID: capability.FileID.String(),
		Variant: int32(capability.Variant), Access: base64.RawURLEncoding.EncodeToString(accessBytes),
		ExpiresAt: capability.ExpiresAt.UTC().UnixNano(),
	})
	if err != nil {
		return "", fmt.Errorf("encode download capability: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyRevocableDownloadCapability(token string, key []byte, now time.Time) (revocableDownloadCapability, error) {
	if len(key) < minimumDownloadCapabilityKeyBytes || token == "" || len(token) > 8192 {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	decoder := json.NewDecoder(bytes.NewReader(payloadBytes))
	decoder.DisallowUnknownFields()
	var payload revocableDownloadPayload
	if err := decoder.Decode(&payload); err != nil {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return revocableDownloadCapability{}, errors.New("invalid download capability")
	}
	profileID, profileErr := uuid.Parse(payload.ProfileID)
	fileID, fileErr := uuid.Parse(payload.FileID)
	accessBytes, accessErr := base64.RawURLEncoding.DecodeString(payload.Access)
	if profileErr != nil || fileErr != nil || accessErr != nil || payload.ExpiresAt <= 0 ||
		!now.UTC().Before(time.Unix(0, payload.ExpiresAt).UTC()) {
		return revocableDownloadCapability{}, errors.New("expired or invalid download capability")
	}
	selector := &filev1.FileAccessSelector{}
	if err := proto.Unmarshal(accessBytes, selector); err != nil || selector.GetSelector() == nil {
		return revocableDownloadCapability{}, errors.New("invalid download access selector")
	}
	if err := validateMessageDownloadSelector(fileID, selector); err != nil {
		return revocableDownloadCapability{}, err
	}
	variant := filev1.FileURLVariant(payload.Variant)
	if _, err := fileURLKeyVariantOnly(variant); err != nil {
		return revocableDownloadCapability{}, err
	}
	return revocableDownloadCapability{ProfileID: profileID, FileID: fileID, Variant: variant,
		Access: selector, ExpiresAt: time.Unix(0, payload.ExpiresAt).UTC()}, nil
}

func validateMessageDownloadSelector(fileID uuid.UUID, selector *filev1.FileAccessSelector) error {
	reference, ok := selector.GetSelector().(*filev1.FileAccessSelector_Reference)
	if !ok || reference.Reference.GetOwnerType() != filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE {
		return errors.New("download capability requires a message reference")
	}
	ids, err := parseReference(reference.Reference)
	if err != nil || ids.file != fileID || ids.owner == uuid.Nil {
		return errors.New("download capability reference mismatch")
	}
	return nil
}

func fileURLKeyVariantOnly(variant filev1.FileURLVariant) (string, error) {
	switch variant {
	case filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
		filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL:
		return "", nil
	default:
		return "", errors.New("invalid download variant")
	}
}
