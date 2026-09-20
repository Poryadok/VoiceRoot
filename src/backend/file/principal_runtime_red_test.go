package main

import "testing"

func TestFilePrincipalRuntimeStaysDisabledWithoutFileSettings(t *testing.T) {
	runtime, err := loadFilePrincipalRuntime()
	if err != nil {
		t.Fatalf("disabled runtime: %v", err)
	}
	if runtime != nil {
		t.Fatal("File principal runtime must not activate without File-owned settings")
	}
}

// The File-to-User protected path must reject a partial explicit deployment.
// This stays red until File owns a signer, HTTPS JWKS endpoint, and the
// dedicated User TLS client as one atomic runtime.
func TestFilePrincipalRuntimeRejectsPartialExplicitConfiguration(t *testing.T) {
	t.Setenv("FILE_PRINCIPAL_SIGNING_KEYS_DIR", "testdata/keys")
	if _, err := loadFilePrincipalRuntime(); err == nil {
		t.Fatal("partial File principal configuration must fail closed")
	}
}

func TestFilePrincipalRuntimeRejectsPartialUserClientConfiguration(t *testing.T) {
	t.Setenv("USER_FILE_PRINCIPAL_GRPC_ADDR", "voice-user:9092")
	if _, err := loadFilePrincipalRuntime(); err == nil {
		t.Fatal("partial dedicated User client configuration must fail closed")
	}
}
