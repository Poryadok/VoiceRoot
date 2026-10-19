//go:build native_admission_integration

package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type approvalPermissionRPC func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f approvalPermissionRPC) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(ctx, r)
}

// Historical seed rows below are storage fixtures. Only the actual signed
// verifier and private owner/Role clients construct the new current capabilities.
// This does not exercise public Q selection, live owner servers or admission.
func TestJoinAttempt_ApprovalSealUsesCurrentReviewerAndStoredApplicant(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Private approval", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	_, parentSHA := seedJoinDecisionHistory(t, st, in, a)
	// Pending deliberately outlives collection and has no applicant-session TTL.
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET created_at=clock_timestamp()-interval '2 hours',collection_deadline=clock_timestamp()-interval '1 hour' WHERE id=$1`, a.ID)
	require.NoError(t, err)
	selected := a.ID.String()
	req := &spacev1.ApproveJoinRequestRequest{SpaceId: space.ID.String(), RequestId: uuid.NewString(), OperationId: uuid.NewString(), AttemptId: &selected}
	reviewerAccount := uuid.New()
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	require.NoError(t, err)
	requestSHA := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: reviewerAccount, ActorProfileID: owner, OperationID: uuid.MustParse(req.OperationId), Method: JoinOperationApprove, RequestSHA256: requestSHA[:]}
	_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	var advance atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(advance.Load())) }
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: JoinOperationApprove, RequestID: id, RequestHash: hash, AccountID: reviewerAccount.String(), ProfileID: owner.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	verifyService := func(ctx context.Context, r proto.Message, audience, rpc string) {
		md, _ := metadata.FromOutgoingContext(ctx)
		transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
		require.NoError(t, err)
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: audience, ExpectedRPC: rpc, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
	}
	ownerCalls, roleCalls := 0, 0
	client := &admissionauthority.Client{Issuer: spaceIssuer, Clock: clock, Auth: submissionOwnerRPC(func(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		verifyService(ctx, r, "auth", authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName)
		if r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT {
			require.Nil(t, r.InteractiveSessionEpoch)
			require.Equal(t, in.AccountID.String(), r.AccountId)
			require.Equal(t, in.ProfileID.String(), r.ProfileId)
		} else {
			require.Equal(t, reviewerAccount.String(), r.AccountId)
			require.Equal(t, owner.String(), r.ProfileId)
			require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
		}
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, err)
		binding := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: binding[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 2, ProfileRevision: 3, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	reviewerClient := &admissionauthority.ReviewerClient{Issuer: spaceIssuer, Clock: clock, Role: approvalPermissionRPC(func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		roleCalls++
		verifyService(ctx, r, "role", rolev1.RoleService_CheckPermission_FullMethodName)
		require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
		require.Equal(t, space.ID.String(), r.SpaceId)
		require.Equal(t, owner.String(), r.ProfileId)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	})}
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }, AdmissionEpoch: func(_ context.Context, account string, epoch int64) error {
		require.Equal(t, reviewerAccount.String(), account)
		require.EqualValues(t, 7, epoch)
		return nil
	}}
	incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	_, err = verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: JoinOperationApprove}, func(ctx context.Context, _ any) (any, error) {
		call, err := lifecycleprincipal.RequireAdmissionApproval(ctx, req)
		require.NoError(t, err)
		proposal, err := st.prepareJoinApprovalProposal(ctx, op, req, call)
		require.NoError(t, err)
		a, r, err := proposal.OwnerRequests()
		require.NoError(t, err)
		applicant, err := client.Observe(ctx, a)
		require.NoError(t, err)
		reviewerSubject, err := client.Observe(ctx, r)
		require.NoError(t, err)
		reviewer, err := reviewerClient.Observe(ctx, reviewerSubject, r)
		require.NoError(t, err)
		decision, err := proposal.Seal(applicant, reviewer)
		require.NoError(t, err)
		require.Equal(t, parentSHA, decision.basis.Context.SubmitBasisSHA256)
		identity, err := decision.currentApplicant()
		require.NoError(t, err)
		require.Equal(t, in.AccountID, identity.AccountID)
		projection := proposal.Context()
		projection.SubmitBasisSHA256[0] ^= 1
		require.Equal(t, parentSHA, decision.basis.Context.SubmitBasisSHA256)
		advance.Store(int64(5 * time.Second))
		_, err = decision.currentApplicant()
		require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, ownerCalls)
	require.Equal(t, 1, roleCalls)
	var state, operationState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&operationState))
	require.Equal(t, "RESERVED", operationState)
}
