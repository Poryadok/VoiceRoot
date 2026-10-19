package store

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/space/internal/admissionauthority"
)

func TestJoinApprovalAliasContextExactBinding(t *testing.T) {
	op := JoinOperationInput{ActorAccountID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ActorProfileID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), SpaceID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OperationID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), Method: JoinOperationApprove, RequestSHA256: bytes.Repeat([]byte{0x77}, 32)}
	q := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	a := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	basis := bytes.Repeat([]byte{0x88}, 32)
	want, _ := hex.DecodeString("a5749980f95c6a9fba85be021d25ac91fd694b1129da126639c6624d3aea16cd")
	got, err := joinApprovalAliasContextSHA256(op, q, a, basis)
	if err != nil || !bytes.Equal(got[:], want) {
		t.Fatalf("independent binary vector: %x %v", got, err)
	}
	for _, field := range []string{"account", "profile", "space", "operation", "method", "body", "q", "a", "basis"} {
		t.Run(field, func(t *testing.T) {
			changed := op
			changed.RequestSHA256 = bytes.Clone(op.RequestSHA256)
			cq, ca, cb := q, a, bytes.Clone(basis)
			switch field {
			case "account":
				changed.ActorAccountID = uuid.New()
			case "profile":
				changed.ActorProfileID = uuid.New()
			case "space":
				changed.SpaceID = uuid.New()
			case "operation":
				changed.OperationID = uuid.New()
			case "method":
				changed.Method = JoinOperationSubmit
			case "body":
				changed.RequestSHA256[0] ^= 1
			case "q":
				cq = uuid.New()
			case "a":
				ca = uuid.New()
			case "basis":
				cb[0] ^= 1
			}
			other, err := joinApprovalAliasContextSHA256(changed, cq, ca, cb)
			if err == nil && other == got {
				t.Fatal("changed binding accepted")
			}
		})
	}
	for _, bad := range [][]byte{nil, make([]byte, 31), make([]byte, 32), make([]byte, 33)} {
		if _, err := joinApprovalAliasContextSHA256(op, q, a, bad); err == nil {
			t.Fatal("invalid original basis accepted")
		}
	}
}

func assertJoinAliasOwnerProtocol(t *testing.T, p *JoinApprovalAliasProposal, published bool, account, profile uuid.UUID) {
	t.Helper()
	a, r, err := p.OwnerRequests()
	require.NoError(t, err)
	require.Equal(t, p.operation.input.ActorAccountID.String(), r.AccountId)
	require.Equal(t, p.operation.input.ActorProfileID.String(), r.ProfileId)
	require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
	require.Equal(t, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, r.SubjectRole)
	if published {
		require.Nil(t, a, "historical read never rechecks applicant or original reviewer eligibility")
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, r.Phase)
	} else {
		require.NotNil(t, a)
		require.Nil(t, a.InteractiveSessionEpoch)
		require.Equal(t, account.String(), a.AccountId)
		require.Equal(t, profile.String(), a.ProfileId)
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, a.Phase)
		require.Equal(t, a.Phase, r.Phase)
	}
	_, err = p.Seal(&admissionauthority.VerifiedSubject{}, &admissionauthority.VerifiedReviewerPermission{})
	require.Error(t, err, "data or empty exported structs never create alias authority")
	r.AdmissionBindingSha256[0] ^= 1
	_, unchanged, e := p.OwnerRequests()
	require.NoError(t, e)
	require.NotEqual(t, r.AdmissionBindingSha256, unchanged.AdmissionBindingSha256, "returned request is a copy")
}
