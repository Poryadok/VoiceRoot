package testfixture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LibsignalGoldenPreKeyBundleB64 returns the committed libsignal pre-key wire golden.
// Generate via: cd src/frontend && flutter test test/tools/export_prekey_golden_test.dart
func LibsignalGoldenPreKeyBundleB64() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testfixture: runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "prekey_libsignal_golden.b64")
	raw, err := os.ReadFile(path)
	if err != nil {
		panic("testfixture: read golden pre-key: " + err.Error())
	}
	return strings.TrimSpace(string(raw))
}

// ComposePreKeyBundleB64 returns a libsignal-signed pre-key wire bundle for gateway compose live tests.
func ComposePreKeyBundleB64() string {
	return LibsignalGoldenPreKeyBundleB64()
}
