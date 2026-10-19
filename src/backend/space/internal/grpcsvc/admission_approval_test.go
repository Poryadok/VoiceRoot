package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestAdmissionApprovalRealPendingSubmissionRequiresCurrentReviewerAndNativeGate(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	for _, name := range []string{"000040_join_decision_basis_v2.up.sql", "000043_join_public_operation_context.up.sql", "000045_join_legacy_selection_context.up.sql"} {
		raw, e := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, e)
		_, e = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, e)
	}
	var owner uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, space).Scan(&owner))
	_, err := st.SetEntryPolicy(ctx, store.SetEntryPolicyInput{SpaceID: space, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	applicant, profile, reviewer := uuid.New(), uuid.New(), uuid.New()
	a, err := st.StartJoinAttempt(ctx, store.JoinAttemptInput{SpaceID: space, AccountID: applicant, ProfileID: profile})
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
	invoke := func(account, actor uuid.UUID, method string, request proto.Message, handler func(context.Context) (any, error)) (any, error) {
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: actor.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		c := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		return verifier.Unary()(c, request, &grpc.UnaryServerInfo{FullMethod: method}, func(c context.Context, _ any) (any, error) { return handler(c) })
	}
	applicantObservations, reviewerObservations, permissionCalls := 0, 0, 0
	allow := false
	subjects := &admissionauthority.Client{Issuer: issuer, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		if r.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE {
			if r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT {
				applicantObservations++
				require.Equal(t, applicant.String(), r.AccountId)
				require.Equal(t, profile.String(), r.ProfileId)
				require.Nil(t, r.InteractiveSessionEpoch)
			} else {
				reviewerObservations++
				require.Equal(t, reviewer.String(), r.AccountId)
				require.Equal(t, owner.String(), r.ProfileId)
				require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
			}
		} else {
			require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, r.Phase)
		}
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, e)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	reviewers := &admissionauthority.ReviewerClient{Issuer: issuer, Role: settingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		permissionCalls++
		require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: allow}, nil
	})}
	api := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: subjects, Reviewers: reviewers})
	submit := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.String(), AttemptId: a.ID.String(), OperationId: uuid.NewString()}
	result, err := invoke(applicant, profile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, submit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, submit) })
	require.NoError(t, err)
	pending := result.(*spacev1.SubmitJoinAttemptResponse).Result
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, pending.State)
	selected := a.ID.String()
	request := &spacev1.ApproveJoinRequestRequest{SpaceId: space.String(), RequestId: pending.RequestId, OperationId: uuid.NewString(), AttemptId: &selected}
	_, err = invoke(reviewer, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, request, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, request) })
	require.Equal(t, codes.PermissionDenied, status.Code(err), "actual fixed42 deny, not inherited Unimplemented")
	allow = true
	_, err = invoke(reviewer, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, request, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, request) })
	require.Equal(t, codes.Unavailable, status.Code(err), "actual native45 unknown must roll back before approval publication")
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, owner, request.OperationId).Scan(&state))
	require.Equal(t, "RESERVED", state)
	require.Equal(t, 2, applicantObservations)
	require.Equal(t, 2, reviewerObservations)
	require.Equal(t, 2, permissionCalls)
	changed := proto.Clone(request).(*spacev1.ApproveJoinRequestRequest)
	changed.RequestId = uuid.NewString()
	_, err = invoke(reviewer, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, changed, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, changed) })
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Equal(t, 2, permissionCalls)
}
