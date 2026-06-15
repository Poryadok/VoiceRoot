package prekey_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/e2e/prekey"
)

// Wire format matches src/frontend/lib/e2e/e2e_store_factory.dart serializePreKeyBundle.
func validTestPreKeyBundleB64() string {
	seed := prekey.TestIdentitySeed(0x03)
	return prekey.TestBundleB64(map[string]any{
		"registration_id":       42_001,
		"device_id":             1,
		"pre_key_id":            7,
		"pre_key_public":        base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x01)),
		"signed_pre_key_id":     1,
		"signed_pre_key_public": base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x02)),
		"identity_key":          base64.StdEncoding.EncodeToString(prekey.TestIdentityKeyWire(seed)),
		"_test_identity_seed":   seed,
	})
}

func fakeCurvePoint33(tag byte) []byte {
	b := make([]byte, 33)
	b[0] = 0x05
	for i := 1; i < 33; i++ {
		b[i] = tag ^ byte(i)
	}
	return b
}

func fakeSignature64() []byte {
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i * 5)
	}
	return sig
}

func TestParseWire_ValidBundle(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	require.Equal(t, 42_001, wire.RegistrationID)
	require.Equal(t, 1, wire.DeviceID)
	require.Equal(t, 7, wire.PreKeyID)
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
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	require.NoError(t, prekey.ValidateForUpload(wire))
}

func TestValidateForUpload_RejectsShortPreKeyPublic(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	wire.PreKeyPublic = []byte{0x05, 0x01}
	require.Len(t, wire.OTPKPool, 1)
	wire.OTPKPool[0].Public = []byte{0x05, 0x01}
	require.Error(t, prekey.ValidateForUpload(wire))
}

func TestValidateForUpload_RejectsShortSignature(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	wire.SignedPreKeySignature = make([]byte, 32)
	require.Error(t, prekey.ValidateForUpload(wire))
}

func TestVerifySignedPreKeySignature_RejectsTamperedSignature(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	require.NoError(t, prekey.VerifySignedPreKeySignature(wire))

	wire.SignedPreKeySignature[0] ^= 0xff
	require.Error(t, prekey.VerifySignedPreKeySignature(wire))
}

func multiOTPKTestBundleB64() string {
	seed := prekey.TestIdentitySeed(0x03)
	return prekey.TestBundleB64(map[string]any{
		"registration_id":       42_001,
		"device_id":             1,
		"pre_key_id":            7,
		"pre_key_public":        base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x01)),
		"signed_pre_key_id":     1,
		"signed_pre_key_public": base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x02)),
		"identity_key":          base64.StdEncoding.EncodeToString(prekey.TestIdentityKeyWire(seed)),
		"_test_identity_seed":   seed,
		"pre_keys": []map[string]any{
			{"pre_key_id": 7, "pre_key_public": base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x11))},
			{"pre_key_id": 8, "pre_key_public": base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x12))},
			{"pre_key_id": 9, "pre_key_public": base64.StdEncoding.EncodeToString(fakeCurvePoint33(0x13))},
		},
	})
}

func TestConsumeNextOTPKFromPool_ServesKeysInOrder(t *testing.T) {
	wire, err := prekey.ParseWire(multiOTPKTestBundleB64())
	require.NoError(t, err)
	require.Equal(t, 3, wire.OTPKPoolSize())

	first, err := prekey.ConsumeNextOTPKFromPool(wire)
	require.NoError(t, err)
	require.Equal(t, 7, first.PreKeyID)
	require.Equal(t, 2, first.OTPKPoolSize())

	second, err := prekey.ConsumeNextOTPKFromPool(first)
	require.NoError(t, err)
	require.Equal(t, 8, second.PreKeyID)
	require.Equal(t, 1, second.OTPKPoolSize())

	third, err := prekey.ConsumeNextOTPKFromPool(second)
	require.NoError(t, err)
	require.Equal(t, 9, third.PreKeyID)
	require.Equal(t, 0, third.OTPKPoolSize())
	require.False(t, third.HasOTPK())
}

func TestPopOTPKForFetch_ReturnsEmptyWhenPoolExhausted(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
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
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
	require.NoError(t, err)
	wire.IdentityKey[1] ^= 0xff
	require.Error(t, prekey.VerifySignedPreKeySignature(wire))
}

func TestEncodeWire_RoundtripsMultiOTPKPool(t *testing.T) {
	original, err := prekey.ParseWire(multiOTPKTestBundleB64())
	require.NoError(t, err)
	encoded, err := prekey.EncodeWire(original)
	require.NoError(t, err)
	parsed, err := prekey.ParseWire(encoded)
	require.NoError(t, err)
	require.Equal(t, 3, parsed.OTPKPoolSize())
}

func TestConsumeOTPK_RemovesOneTimeKeyFields(t *testing.T) {
	wire, err := prekey.ParseWire(validTestPreKeyBundleB64())
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
