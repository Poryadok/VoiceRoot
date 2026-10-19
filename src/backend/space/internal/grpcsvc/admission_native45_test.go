package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
	"voice/backend/space/internal/store"
)

type publicAdmissionEnsureFixture func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error)

func (f publicAdmissionEnsureFixture) EnsureDefaultMemberRole(c context.Context, r *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	return f(c, r)
}

// The native database, real private ingress and public handler chain are actual.
// Auth and Role protocol fixtures remain controlled: this is not Java/RolePG,
// real default-grant, durable nonce or full protected startup qualification.
func TestPublicAdmissionNative45QuestionsAndIndependentManualApproval(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	admin := &store.SpaceStore{Pool: db.Pool}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	gateway, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, e)
	issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, e)
	mapReplay := func(err error) error {
		if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
			return lifecycleprincipal.ErrDependencyUnavailable
		}
		return err
	}
	admissionReplayStore := admin
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error { return nil }, CreatorReplay: func(c context.Context, issuer, jti string, expiry time.Time) error {
		return mapReplay(admin.ClaimCreatorPrincipalReplay(c, issuer, jti, expiry))
	}, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(c context.Context, issuer, jti string, expiry time.Time) error {
		return mapReplay(admissionReplayStore.ClaimAdmissionPrincipalReplay(c, issuer, jti, expiry))
	}}
	invoke := func(account, profile uuid.UUID, rpc string, request proto.Message, handler func(context.Context) (any, error)) (any, error) {
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: rpc, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		return verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: rpc}, func(c context.Context, _ any) (any, error) { return handler(c) })
	}
	ownerAccount, owner := uuid.New(), uuid.New()
	create := &spacev1.CreateSpaceRequest{Name: "Public admission native45", Visibility: "public"}
	created, e := invoke(ownerAccount, owner, spacev1.SpaceService_CreateSpace_FullMethodName, create, func(c context.Context) (any, error) {
		cap, e := lifecycleprincipal.RequireCreator(c, create)
		if e != nil {
			return nil, e
		}
		return admin.CreateSpaceWithCreator(c, cap, create)
	})
	require.NoError(t, e)
	space := created.(*store.SpaceRow).ID
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space))
	db.Run(true, "up")
	_, e = admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, e)
	// Same actual LOGIN provisioning as store/native_membership_readiness_test;
	// SET ROLE on a superuser would not exercise the real serving guard.
	role := "public_admission_" + uuid.NewString()[:8]
	_, e = db.Pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-fixture-only'")
	require.NoError(t, e)
	_, e = db.Pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, e)
	cfg := db.Pool.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "isolated-fixture-only"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, e)
	t.Cleanup(func() {
		pool.Close()
		_, _ = db.Pool.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = db.Pool.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	st := &store.SpaceStore{Pool: pool}
	admissionReplayStore = st
	ownerCalls, permissionCalls, ensureCalls := 0, 0, 0
	allow := true
	subjects := &admissionauthority.Client{Issuer: issuer, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, e)
		hash := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: hash[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	permissions := settingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		permissionCalls++
		require.Contains(t, []string{admissionauthority.ReviewJoinPermission, admissionauthority.ManageEntryPolicyPermission}, r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: allow && r.ProfileId == owner.String()}, nil
	})
	roles := &memberadmissionrole.Client{Issuer: issuer, Role: publicAdmissionEnsureFixture(func(_ context.Context, r *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
		ensureCalls++
		selected := uuid.NewString()
		binding := memberadmission.EnsureBinding{ProtocolVersion: r.ProtocolVersion, SpaceID: r.SpaceId, ProfileID: r.ProfileId, AdmissionOperationID: r.AdmissionOperationId, MembershipGeneration: r.MembershipGeneration, AdmissionBindingSHA256: r.AdmissionBindingSha256}
		sha, e := memberadmission.EnsureReceiptFingerprint(binding, selected)
		require.NoError(t, e)
		return &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: r.ProtocolVersion, SpaceId: r.SpaceId, ProfileId: r.ProfileId, AdmissionOperationId: r.AdmissionOperationId, MembershipGeneration: r.MembershipGeneration, AdmissionBindingSha256: append([]byte(nil), r.AdmissionBindingSha256...), RoleId: selected, ReceiptSha256: sha[:]}}, nil
	})}
	api := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: subjects, Reviewers: &admissionauthority.ReviewerClient{Issuer: issuer, Role: permissions}, Settings: &admissionauthority.SettingsClient{Issuer: issuer, Role: permissions}, Roles: roles})
	question := uuid.New()
	policy := entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2?", Kind: "exact_text", ExpectedAnswer: "4"}}}
	raw, e := json.Marshal(policy)
	require.NoError(t, e)
	settings := &spacev1.SetEntryPolicyRequest{SpaceId: space.String(), OperationId: uuid.NewString(), ExpectedPolicyVersion: 1, PrivatePolicyJson: raw}
	ack, e := invoke(ownerAccount, owner, spacev1.SpaceService_SetEntryPolicy_FullMethodName, settings, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, settings) })
	require.NoError(t, e)
	require.EqualValues(t, 2, ack.(*spacev1.SetEntryPolicyResponse).EntryPolicyVersion)
	applicant, profile := uuid.New(), uuid.New()
	start := func() *spacev1.JoinAttemptView {
		r := &spacev1.StartJoinAttemptRequest{OperationId: uuid.NewString(), Target: &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space.String()}}
		result, e := invoke(applicant, profile, spacev1.SpaceService_StartJoinAttempt_FullMethodName, r, func(c context.Context) (any, error) { return api.StartJoinAttempt(c, r) })
		require.NoError(t, e)
		return result.(*spacev1.StartJoinAttemptResponse).Attempt
	}
	bad := start()
	submit := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.String(), AttemptId: bad.AttemptId, OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: "5"}}}
	_, e = invoke(applicant, profile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, submit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, submit) })
	require.Equal(t, codes.PermissionDenied, status.Code(e))
	beforeOwners := ownerCalls
	_, e = invoke(applicant, profile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, submit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, submit) })
	require.Equal(t, codes.PermissionDenied, status.Code(e))
	require.Equal(t, beforeOwners, ownerCalls)
	good := start()
	require.NotEqual(t, bad.AttemptId, good.AttemptId)
	submit = &spacev1.SubmitJoinAttemptRequest{SpaceId: space.String(), AttemptId: good.AttemptId, OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: "4"}}}
	pending, e := invoke(applicant, profile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, submit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, submit) })
	require.NoError(t, e)
	pendingResponse := pending.(*spacev1.SubmitJoinAttemptResponse)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, pendingResponse.Result.State)
	require.Zero(t, ensureCalls, "correct automatic answer never replaces independent manual approval")
	list := &spacev1.ListJoinRequestsRequest{SpaceId: space.String()}
	listed, e := invoke(ownerAccount, owner, spacev1.SpaceService_ListJoinRequests_FullMethodName, list, func(c context.Context) (any, error) { return api.ListJoinRequests(c, list) })
	require.NoError(t, e)
	require.Len(t, listed.(*spacev1.ListJoinRequestsResponse).Requests, 1)
	get := &spacev1.GetJoinRequestRequest{SpaceId: space.String(), RequestId: pendingResponse.Result.RequestId}
	view, e := invoke(ownerAccount, owner, spacev1.SpaceService_GetJoinRequest_FullMethodName, get, func(c context.Context) (any, error) { return api.GetJoinRequest(c, get) })
	require.NoError(t, e)
	require.Len(t, view.(*spacev1.GetJoinRequestResponse).Attempts, 1)
	wire, e := protojson.Marshal(view.(proto.Message))
	require.NoError(t, e)
	require.NotContains(t, string(wire), "expectedAnswer")
	require.NotContains(t, string(wire), "expected_answer")
	selector := good.AttemptId
	approve := &spacev1.ApproveJoinRequestRequest{SpaceId: space.String(), RequestId: get.RequestId, OperationId: uuid.NewString(), AttemptId: &selector}
	admitted, e := invoke(ownerAccount, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, approve, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, approve) })
	require.NoError(t, e)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, admitted.(*spacev1.ApproveJoinRequestResponse).Result.State)
	require.Equal(t, profile.String(), admitted.(*spacev1.ApproveJoinRequestResponse).Result.SpaceMembership.ProfileId)
	require.Equal(t, 1, ensureCalls)
	alias := proto.Clone(approve).(*spacev1.ApproveJoinRequestRequest)
	alias.OperationId = uuid.NewString()
	aliased, e := invoke(ownerAccount, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, alias, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, alias) })
	require.NoError(t, e)
	require.True(t, proto.Equal(admitted.(proto.Message), aliased.(proto.Message)))
	require.Equal(t, 1, ensureCalls, "observing a published winner never re-runs Role Ensure")
	// Automatic question checking and manual review are independent AND gates.
	// Removing only manual approval allows a new correct applicant to publish
	// through the actual public Submit coordinator, not the approval adapter.
	policy.RequireManualApproval = false
	automaticPolicy, e := json.Marshal(policy)
	require.NoError(t, e)
	change := &spacev1.SetEntryPolicyRequest{SpaceId: space.String(), OperationId: uuid.NewString(), ExpectedPolicyVersion: 2, PrivatePolicyJson: automaticPolicy}
	changed, e := invoke(ownerAccount, owner, spacev1.SpaceService_SetEntryPolicy_FullMethodName, change, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, change) })
	require.NoError(t, e)
	require.EqualValues(t, 3, changed.(*spacev1.SetEntryPolicyResponse).EntryPolicyVersion)
	autoAccount, autoProfile := uuid.New(), uuid.New()
	autoStart := &spacev1.StartJoinAttemptRequest{OperationId: uuid.NewString(), Target: &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space.String()}}
	autoStarted, e := invoke(autoAccount, autoProfile, spacev1.SpaceService_StartJoinAttempt_FullMethodName, autoStart, func(c context.Context) (any, error) { return api.StartJoinAttempt(c, autoStart) })
	require.NoError(t, e)
	autoSubmit := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.String(), AttemptId: autoStarted.(*spacev1.StartJoinAttemptResponse).Attempt.AttemptId, OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: "4"}}}
	autoResult, e := invoke(autoAccount, autoProfile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, autoSubmit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, autoSubmit) })
	require.NoError(t, e)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, autoResult.(*spacev1.SubmitJoinAttemptResponse).Result.State)
	require.Equal(t, autoProfile.String(), autoResult.(*spacev1.SubmitJoinAttemptResponse).Result.SpaceMembership.ProfileId)
	require.Equal(t, 2, ensureCalls)
	allow = false
	beforeOwners, beforePermissions := ownerCalls, permissionCalls
	replayed, e := invoke(ownerAccount, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, approve, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, approve) })
	require.NoError(t, e)
	require.True(t, proto.Equal(admitted.(proto.Message), replayed.(proto.Message)))
	require.Equal(t, beforeOwners, ownerCalls)
	require.Equal(t, beforePermissions, permissionCalls)
	require.Equal(t, 2, ensureCalls)
	_, e = invoke(ownerAccount, owner, spacev1.SpaceService_ApproveJoinRequest_FullMethodName, alias, func(c context.Context) (any, error) { return api.ApproveJoinRequest(c, alias) })
	require.Equal(t, codes.PermissionDenied, status.Code(e))
	beforeOwners, beforePermissions = ownerCalls, permissionCalls
	oldSettings, e := invoke(ownerAccount, owner, spacev1.SpaceService_SetEntryPolicy_FullMethodName, settings, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, settings) })
	require.NoError(t, e)
	require.True(t, proto.Equal(ack.(proto.Message), oldSettings.(proto.Message)))
	require.Equal(t, beforeOwners, ownerCalls)
	require.Equal(t, beforePermissions, permissionCalls)
	beforeOwners = ownerCalls
	replayed, e = invoke(applicant, profile, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, submit, func(c context.Context) (any, error) { return api.SubmitJoinAttempt(c, submit) })
	require.NoError(t, e)
	require.True(t, proto.Equal(pendingResponse, replayed.(proto.Message)))
	require.Equal(t, beforeOwners, ownerCalls)
	read := &spacev1.GetJoinAttemptRequest{SpaceId: space.String(), AttemptId: good.AttemptId}
	current, e := invoke(applicant, profile, spacev1.SpaceService_GetJoinAttempt_FullMethodName, read, func(c context.Context) (any, error) { return api.GetJoinAttempt(c, read) })
	require.NoError(t, e)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, current.(*spacev1.GetJoinAttemptResponse).Attempt.Result.State)
}
