package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMonitorResponderRequiresSignedFreshDecisionAndPinnedKey(t *testing.T) {
	operatorPublic, operatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	monitorPublic, monitorPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	claim := OpenAttestation{
		Challenge: supervisionNonce(1), ControlSystem: "123", ControlDatabase: "custody_db", ControlDatabaseOID: 9,
		Binding:  Binding{Epoch: 1, SourceID: "source", HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: supervisionNonce(2)},
		Physical: PhysicalSource{SystemIdentifier: "456", Database: "user_db", DatabaseOID: 42}, WriterRole: "writer",
		SourceEvidenceDigest: supervisionNonce(3), WriterManifestDigest: supervisionNonce(4), AuthEvidenceDigest: supervisionNonce(5),
		IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(time.Minute).Unix(),
	}
	claim, err = SignOpenAttestation(claim, operatorPrivate)
	require.NoError(t, err)
	responder, err := newDecisionResponder(claim, operatorPublic, monitorPublic, monitorPrivate, uuid.New(), now)
	require.NoError(t, err)
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(payload), responder.decisionDigest)
	_, err = newDecisionResponder(claim, otherPublic, monitorPublic, monitorPrivate, uuid.New(), now)
	require.Error(t, err)
	_, err = newDecisionResponder(claim, operatorPublic, otherPublic, monitorPrivate, uuid.New(), now)
	require.Error(t, err)
	_, err = newDecisionResponder(claim, operatorPublic, operatorPublic, operatorPrivate, uuid.New(), now)
	require.Error(t, err, "the monitor must use an independently pinned key")
	_, err = newDecisionResponder(claim, operatorPublic, monitorPublic, monitorPrivate, uuid.New(), now.Add(time.Minute))
	require.Error(t, err)
	for _, tc := range []struct {
		name            string
		issued, expires int64
		valid           bool
	}{
		{"future issue", now.Unix() + 1, now.Unix() + 60, false},
		{"old issue", now.Unix() - 301, now.Unix() + 60, false},
		{"distant expiry", now.Unix(), now.Unix() + 301, false},
		{"inclusive outer bounds", now.Unix() - 300, now.Unix() + 300, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := claim
			candidate.Signature = nil
			candidate.IssuedAtUnix, candidate.ExpiresAtUnix = tc.issued, tc.expires
			candidate, signErr := SignOpenAttestation(candidate, operatorPrivate)
			require.NoError(t, signErr)
			_, enrollErr := newDecisionResponder(candidate, operatorPublic, monitorPublic, monitorPrivate, uuid.New(), now)
			if tc.valid {
				require.NoError(t, enrollErr)
			} else {
				require.Error(t, enrollErr)
			}
		})
	}
	claim.WriterRole = "other_writer"
	_, err = newDecisionResponder(claim, operatorPublic, monitorPublic, monitorPrivate, uuid.New(), now)
	require.Error(t, err)
}
