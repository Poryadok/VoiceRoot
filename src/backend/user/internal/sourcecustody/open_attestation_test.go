package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAttestationBindsChallengeSourceAndEvidence(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	other, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	binding := custodyBinding()
	controller := &Controller{binding: binding, controlSystem: "123", controlDatabase: "control_db", controlDatabaseOID: 41, activationChallenge: [32]byte{1}}
	now := time.Now().UTC()
	claim := OpenAttestation{Challenge: controller.activationChallenge, ControlSystem: controller.controlSystem, ControlDatabase: controller.controlDatabase, ControlDatabaseOID: controller.controlDatabaseOID, Binding: binding, Physical: PhysicalSource{SystemIdentifier: "456", Database: "user_db", DatabaseOID: 42}, WriterRole: "user_writer", SourceEvidenceDigest: [32]byte{1}, WriterManifestDigest: [32]byte{2}, AuthEvidenceDigest: [32]byte{3}, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	claim.Signature = ed25519.Sign(private, payload)
	require.NoError(t, controller.verifyOpenAttestation(claim, public, now))
	missingSourceEvidence := claim
	missingSourceEvidence.SourceEvidenceDigest = [32]byte{}
	_, err = OpenAttestationPayload(missingSourceEvidence)
	require.Error(t, err)
	require.Error(t, controller.verifyOpenAttestation(claim, other, now))
	for name, mutate := range map[string]func(*OpenAttestation){
		"challenge":         func(a *OpenAttestation) { a.Challenge[0]++ },
		"control database":  func(a *OpenAttestation) { a.ControlDatabaseOID++ },
		"source generation": func(a *OpenAttestation) { a.Binding.UserSourceGeneration[0]++ },
		"physical database": func(a *OpenAttestation) { a.Physical.DatabaseOID++ },
		"writer role":       func(a *OpenAttestation) { a.WriterRole = "other_writer" },
		"writer evidence":   func(a *OpenAttestation) { a.WriterManifestDigest[0]++ },
		"source evidence":   func(a *OpenAttestation) { a.SourceEvidenceDigest[0]++ },
		"auth evidence":     func(a *OpenAttestation) { a.AuthEvidenceDigest[0]++ },
		"guard cutover":     func(a *OpenAttestation) { a.GuardCutover = !a.GuardCutover },
		"expiry":            func(a *OpenAttestation) { a.ExpiresAtUnix++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := claim
			mutate(&changed)
			require.Error(t, controller.verifyOpenAttestation(changed, public, now))
		})
	}
	require.Error(t, controller.verifyOpenAttestation(claim, public, now.Add(2*time.Minute)))
}

func TestControllerRestartChangesActivationChallenge(t *testing.T) {
	ctx, control, _, binding, first := custodyFixture(t)
	firstChallenge, err := first.ActivationChallenge()
	require.NoError(t, err)
	require.NoError(t, first.Shutdown(ctx))
	_, err = first.ActivationChallenge()
	require.Error(t, err)
	second, err := Start(ctx, control, binding)
	require.NoError(t, err)
	defer func() { require.NoError(t, second.Shutdown(ctx)) }()
	secondChallenge, err := second.ActivationChallenge()
	require.NoError(t, err)
	require.NotEqual(t, firstChallenge, secondChallenge)
}
