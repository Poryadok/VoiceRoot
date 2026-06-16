package prekey_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/e2e/prekey"
)

// Golden bundle is produced by src/frontend/test/tools/export_prekey_golden_test.dart
// (libsignal serializePreKeyBundle wire). Backend must verify libsignal signatures.
func TestVerifySignedPreKeySignature_AcceptsLibsignalGoldenBundle(t *testing.T) {
	wireB64 := loadLibsignalGoldenPreKeyBundleB64(t)
	wire, err := prekey.ParseWire(wireB64)
	require.NoError(t, err)
	require.NoError(t, prekey.ValidateForUpload(wire))
	require.NoError(t, prekey.VerifySignedPreKeySignature(wire))
}

func loadLibsignalGoldenPreKeyBundleB64(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "testfixture", "prekey_libsignal_golden.b64")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "run src/frontend/test/tools/export_prekey_golden_test.dart to generate golden")
	return string(raw)
}
