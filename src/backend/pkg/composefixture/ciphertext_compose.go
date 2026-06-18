package composefixture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// E2ECiphertextGoldenPlaintext is the deterministic libsignal plaintext token in the golden.
const E2ECiphertextGoldenPlaintext = "phase15-golden-plaintext"

// LibsignalGoldenE2ECiphertextB64 returns committed libsignal message wire from Flutter export.
func LibsignalGoldenE2ECiphertextB64() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("composefixture: runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "e2e_ciphertext_libsignal_golden.b64")
	raw, err := os.ReadFile(path)
	if err != nil {
		panic("composefixture: read golden e2e ciphertext: " + err.Error())
	}
	return strings.TrimSpace(string(raw))
}
