package store

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
)

type continuationRoleRPC func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f continuationRoleRPC) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(ctx, r)
}

func TestMemberAdmissionContinuationManualOriginalReviewerAndLocalLoss(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	for _, loseReviewer := range []bool{false, true} {
		t.Run(map[bool]string{false: "original reviewer", true: "reviewer loss"}[loseReviewer], func(t *testing.T) {
			create, reviewerAccount, reviewerProfile := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Original reviewer", Visibility: "public"})
			space, err := create()
			require.NoError(t, err)
			newOwner := uuid.New()
			if loseReviewer {
				publishAdmissionFixture(t, st, space.ID, reviewerProfile, newOwner)
			}
			_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: reviewerProfile, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
			require.NoError(t, err)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
			require.NoError(t, err)
			parent, parentSHA := seedJoinDecisionHistory(t, st, in, a)
			approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: reviewerAccount, ActorProfileID: reviewerProfile, OperationID: uuid.New()}
			_, err = st.ReserveJoinAttemptApproval(ctx, approval)
			require.NoError(t, err)
			// Storage-only historical decision, not a live approval capability.
			a, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
			require.NoError(t, err)
			op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: reviewerAccount, ActorProfileID: reviewerProfile, OperationID: approval.OperationID, Method: JoinOperationApprove, RequestSHA256: joinDigest("native manual fixture", "approval")}
			_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
			require.NoError(t, err)
			basis := JoinDecisionBasis{Context: parent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{11}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{12}, 32)}
			basis.Context.Kind, basis.Context.ReadyEvidenceSHA256 = JoinDecisionApproveReady, a.EvidenceSHA256
			basis.Context.ActorAccountID, basis.Context.ActorProfileID = reviewerAccount, reviewerProfile
			basis.Context.OperationID, basis.Context.RequestMethod, basis.Context.RequestSHA256 = op.OperationID, op.Method, op.RequestSHA256
			basis.Context.SubmitBasisSHA256 = parentSHA
			basisSHA := persistJoinDecisionHistory(t, st, op, basis)
			i, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: in.AccountID, ProfileID: in.ProfileID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basisSHA})
			require.NoError(t, err)
			role := uuid.New()
			receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
			require.NoError(t, err)
			now := time.Now()
			client := continuationClient(t, &now, "regular")
			originalAuth := client.Auth
			subjectCalls, permissionCalls := 0, 0
			client.Auth = continuationAuthRPC(func(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				subjectCalls++
				if r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER {
					require.Equal(t, reviewerAccount.String(), r.AccountId)
					require.Equal(t, reviewerProfile.String(), r.ProfileId)
				}
				return originalAuth.ObserveSpaceAdmissionSubject(ctx, r)
			})
			reviewers := &admissionauthority.ReviewerClient{Issuer: client.Issuer, Clock: func() time.Time { return now }, Role: continuationRoleRPC(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
				permissionCalls++
				require.Equal(t, reviewerProfile.String(), r.ProfileId)
				require.Equal(t, space.ID.String(), r.SpaceId)
				require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
				return &rolev1.CheckPermissionResponse{Allowed: true}, nil
			})}
			proof, err := st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, client, reviewers)
			require.NoError(t, err)
			require.Equal(t, 2, subjectCalls)
			require.Equal(t, 1, permissionCalls)
			if loseReviewer {
				_, err = st.Pool.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, space.ID, newOwner)
				require.NoError(t, err)
				require.NoError(t, st.RemoveMember(ctx, space.ID, reviewerProfile))
			}
			_, err = st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt[:], proof, nil)
			if loseReviewer {
				require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "fresh Role allow cannot replace current local reviewer membership")
				saved, err := st.LookupMemberAdmission(ctx, i.Input.OperationID)
				require.NoError(t, err)
				require.Equal(t, "PREPARING", saved.State)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 2, subjectCalls, "no owner RPC under publication locks")
			require.Equal(t, 1, permissionCalls)
		})
	}
}
