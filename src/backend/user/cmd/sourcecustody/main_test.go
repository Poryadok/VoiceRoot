package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/sourcecustody"
)

func TestSignOpenChild(t *testing.T) {
	if os.Getenv("VOICE_SIGN_OPEN_TEST_CHILD") != "1" {
		return
	}
	if signOpen() != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestSignOpenUsesCanonicalStatementAndRejectsResigning(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "custodian-key.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600))
	now := time.Now().Unix()
	claim := sourcecustody.OpenAttestation{Challenge: [32]byte{1}, ControlSystem: "123", ControlDatabase: "control", ControlDatabaseOID: 10, Binding: sourcecustody.Binding{Epoch: 1, SourceID: "user-source", HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: [32]byte{2}}, Physical: sourcecustody.PhysicalSource{SystemIdentifier: "456", Database: "user_db", DatabaseOID: 20}, WriterRole: "user_writer", SourceEvidenceDigest: [32]byte{5}, WriterManifestDigest: [32]byte{3}, AuthEvidenceDigest: [32]byte{4}, GuardCutover: true, IssuedAtUnix: now, ExpiresAtUnix: now + 60}
	run := func(input []byte) ([]byte, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSignOpenChild$")
		cmd.Env = append(os.Environ(), "VOICE_SIGN_OPEN_TEST_CHILD=1", "CUSTODY_OPERATOR_SIGNER_PRIVATE_KEY_FILE="+path)
		cmd.Stdin = bytes.NewReader(input)
		return cmd.Output()
	}
	input, err := json.Marshal(claim)
	require.NoError(t, err)
	output, err := run(input)
	require.NoError(t, err)
	var signed sourcecustody.OpenAttestation
	require.NoError(t, json.Unmarshal(output, &signed))
	payload, err := sourcecustody.OpenAttestationPayload(signed)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(public, payload, signed.Signature))
	_, err = run(output)
	require.Error(t, err)
	signed.GuardCutover = false
	changed, err := sourcecustody.OpenAttestationPayload(signed)
	require.NoError(t, err)
	require.False(t, ed25519.Verify(public, changed, signed.Signature))
	_, err = run([]byte(`{"unknown":true}`))
	require.Error(t, err)
}
