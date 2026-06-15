package testfixture

import (
	"encoding/base64"

	"voice/backend/messaging/internal/e2e/prekey"
)

func fakeCurvePoint33(tag byte) []byte {
	b := make([]byte, 33)
	b[0] = 0x05
	for i := 1; i < 33; i++ {
		b[i] = tag ^ byte(i)
	}
	return b
}

// ComposePreKeyBundleB64 returns a signed pre-key wire bundle for gateway compose live tests.
func ComposePreKeyBundleB64() string {
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
