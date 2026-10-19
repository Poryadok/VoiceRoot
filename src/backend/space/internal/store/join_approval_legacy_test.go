package store

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
)

// Historical parent/prospective context only. No private call is manufactured,
// and neither this fixture nor its stored observation hashes grants approval.
func TestJoinApprovalLegacyProposalPreservesOriginalV2Family(t *testing.T) {
	f := newJoinGroupFixture(t)
	applyJoinV2Migration(t, f.store)
	a, q := f.child(t)
	var binding, submission []byte
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
	parent := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyPending, OperationOrigin: JoinOperationServerNoKey, SpaceID: f.space, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ActorAccountID: f.account, ActorProfileID: f.profile, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinByInvite_FullMethodName, RequestSHA256: bytes.Repeat([]byte{1}, 32)}, ApplicantObservationSHA256: bytes.Repeat([]byte{2}, 32)}
	parentSHA := seedLegacyJoinDecisionHistory(t, f.store, parent)
	selected := a.ID.String()
	request := &spacev1.ApproveJoinRequestRequest{SpaceId: f.space.String(), RequestId: q.ID.String(), OperationId: uuid.NewString(), AttemptId: &selected}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	sha := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: f.space, ActorAccountID: uuid.New(), ActorProfileID: f.owner, OperationID: uuid.MustParse(request.OperationId), Method: JoinOperationApprove, RequestSHA256: sha[:]}
	_, err = f.store.ResolveJoinRequestApproval(f.ctx, op, q.ID, &a.ID)
	require.NoError(t, err)
	p, err := f.store.prepareJoinApprovalProposal(f.ctx, op, request, nil)
	require.NoError(t, err, "legacy parent needs its own kind6 proposal, not a v1 cast")
	legacy, err := p.LegacyContext()
	require.NoError(t, err)
	require.Equal(t, JoinApproveLegacyReady, legacy.Kind)
	require.Equal(t, JoinOperationClientKey, legacy.OperationOrigin)
	require.Equal(t, parentSHA, legacy.PendingBasisSHA256)
	require.Equal(t, parent.Context.AttemptBindingSHA256, legacy.AttemptBindingSHA256)
	require.Equal(t, parent.Context.SubmissionSHA256, legacy.SubmissionSHA256)
	require.Equal(t, op.ActorAccountID, legacy.ActorAccountID)
	require.Equal(t, op.OperationID, legacy.OperationID)
	require.Equal(t, JoinOperationApprove, legacy.RequestMethod)
	require.Equal(t, sha[:], legacy.RequestSHA256)
	require.Zero(t, p.Context().Kind, "legacy data must not be represented as a v1 ready context")
	legacy.PendingBasisSHA256[0] ^= 1
	copy, err := p.LegacyContext()
	require.NoError(t, err)
	require.Equal(t, parentSHA, copy.PendingBasisSHA256)
	_, _, err = p.OwnerRequests()
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "stored data cannot mint current Auth requests without original private ingress")
	_, err = p.Seal(&admissionauthority.VerifiedSubject{}, &admissionauthority.VerifiedReviewerPermission{})
	require.Error(t, err)
	var state string
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
}
