package testfixture

import "voice/backend/pkg/composefixture"

// LibsignalGoldenPreKeyBundleB64 returns the committed libsignal pre-key wire golden.
func LibsignalGoldenPreKeyBundleB64() string {
	return composefixture.LibsignalGoldenPreKeyBundleB64()
}

// ComposePreKeyBundleB64 returns a libsignal-signed pre-key wire bundle for compose live tests.
func ComposePreKeyBundleB64() string {
	return composefixture.ComposePreKeyBundleB64()
}
