package prekey_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/e2e/prekey"
)

// Wire format matches src/frontend/lib/e2e/e2e_store_factory.dart serializePreKeyBundle.
func validTestPreKeyBundleB64(t *testing.T) string {
	t.Helper()
	return loadLibsignalGoldenPreKeyBundleB64(t)
}

func TestParseWire_ValidBundle(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	require.Equal(t, 42_001, wire.RegistrationID)
	require.Equal(t, 1, wire.DeviceID)
	require.Equal(t, 1, wire.PreKeyID)
	require.Len(t, wire.PreKeyPublic, 33)
	require.Equal(t, 1, wire.SignedPreKeyID)
	require.Len(t, wire.SignedPreKeyPublic, 33)
	require.Len(t, wire.SignedPreKeySignature, 64)
	require.Len(t, wire.IdentityKey, 33)
}

func TestParseWire_RejectsInvalidBase64(t *testing.T) {
	_, err := prekey.ParseWire("%%%not-base64%%%")
	require.Error(t, err)
}

func TestParseWire_RejectsMissingFields(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"registration_id": 1})
	require.NoError(t, err)
	wire := base64.StdEncoding.EncodeToString(raw)
	_, err = prekey.ParseWire(wire)
	require.Error(t, err)
}

func TestValidateForUpload_AcceptsValidWire(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	require.NoError(t, prekey.ValidateForUpload(wire))
}

func TestValidateForUpload_RejectsShortPreKeyPublic(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	wire.PreKeyPublic = []byte{0x05, 0x01}
	require.Len(t, wire.OTPKPool, 10)
	wire.OTPKPool[0].Public = []byte{0x05, 0x01}
	require.Error(t, prekey.ValidateForUpload(wire))
}

func TestValidateForUpload_RejectsShortSignature(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	wire.SignedPreKeySignature = make([]byte, 32)
	require.Error(t, prekey.ValidateForUpload(wire))
}

func TestVerifySignedPreKeySignature_RejectsTamperedSignature(t *testing.T) {
	wire, err := prekey.ParseWire(loadLibsignalGoldenPreKeyBundleB64(t))
	require.NoError(t, err)
	require.NoError(t, prekey.VerifySignedPreKeySignature(wire))

	wire.SignedPreKeySignature[0] ^= 0xff
	require.Error(t, prekey.VerifySignedPreKeySignature(wire))
}

func TestConsumeNextOTPKFromPool_ServesKeysInOrder(t *testing.T) {
	wire, err := prekey.ParseWire(loadLibsignalGoldenPreKeyBundleB64(t))
	require.NoError(t, err)
	require.Equal(t, 10, wire.OTPKPoolSize())

	first, err := prekey.ConsumeNextOTPKFromPool(wire)
	require.NoError(t, err)
	require.Equal(t, 1, first.PreKeyID)
	require.Equal(t, 9, first.OTPKPoolSize())

	second, err := prekey.ConsumeNextOTPKFromPool(first)
	require.NoError(t, err)
	require.Equal(t, 2, second.PreKeyID)
	require.Equal(t, 8, second.OTPKPoolSize())

	third, err := prekey.ConsumeNextOTPKFromPool(second)
	require.NoError(t, err)
	require.Equal(t, 3, third.PreKeyID)
	require.Equal(t, 7, third.OTPKPoolSize())
}

func TestPopOTPKForFetch_ReturnsEmptyWhenPoolExhausted(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	for wire.OTPKPoolSize() > 0 {
		_, wire, err = prekey.PopOTPKForFetch(wire)
		require.NoError(t, err)
	}
	response, stored, err := prekey.PopOTPKForFetch(wire)
	require.NoError(t, err)
	require.False(t, response.HasOTPK())
	require.False(t, stored.HasOTPK())
}

func TestValidateForUpload_RejectsNilWire(t *testing.T) {
	require.Error(t, prekey.ValidateForUpload(nil))
}

func TestEncodeWire_RejectsNilWire(t *testing.T) {
	_, err := prekey.EncodeWire(nil)
	require.Error(t, err)
}

func TestVerifySignedPreKeySignature_RejectsNilWire(t *testing.T) {
	require.Error(t, prekey.VerifySignedPreKeySignature(nil))
}

func TestPopOTPKForFetch_RejectsNilWire(t *testing.T) {
	_, _, err := prekey.PopOTPKForFetch(nil)
	require.Error(t, err)
}

func TestVerifySignedPreKeySignature_RejectsInvalidIdentityKey(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)
	wire.IdentityKey[1] ^= 0xff
	require.Error(t, prekey.VerifySignedPreKeySignature(wire))
}

func TestEncodeWire_RoundtripsMultiOTPKPool(t *testing.T) {
	original, err := prekey.ParseWire(loadLibsignalGoldenPreKeyBundleB64(t))
	require.NoError(t, err)
	encoded, err := prekey.EncodeWire(original)
	require.NoError(t, err)
	parsed, err := prekey.ParseWire(encoded)
	require.NoError(t, err)
	require.Equal(t, 10, parsed.OTPKPoolSize())
}

func TestConsumeOTPK_RemovesOneTimeKeyFields(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64(t))
	require.NoError(t, err)

	consumed, err := prekey.ConsumeOTPK(wire)
	require.NoError(t, err)
	require.Equal(t, wire.RegistrationID, consumed.RegistrationID)
	require.Equal(t, wire.DeviceID, consumed.DeviceID)
	require.Equal(t, wire.SignedPreKeyID, consumed.SignedPreKeyID)
	require.Zero(t, consumed.PreKeyID)
	require.Nil(t, consumed.PreKeyPublic)

	reencoded, err := prekey.EncodeWire(consumed)
	require.NoError(t, err)
	jsonBytes, err := base64.StdEncoding.DecodeString(reencoded)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &payload))
	_, hasPreKeyID := payload["pre_key_id"]
	_, hasPreKeyPublic := payload["pre_key_public"]
	require.False(t, hasPreKeyID, "consumed bundle must omit pre_key_id")
	require.False(t, hasPreKeyPublic, "consumed bundle must omit pre_key_public")
}
