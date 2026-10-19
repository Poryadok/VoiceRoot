package grpcsvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"sync/atomic"
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
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/privacy"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
	"voice/backend/space/internal/store"
)

type legacyNativeSubjectRPC func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f legacyNativeSubjectRPC) ObserveSpaceAdmissionSubject(ctx context.Context, req *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, req)
}

type legacyNativeRoleRPC func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error)

func (f legacyNativeRoleRPC) EnsureDefaultMemberRole(ctx context.Context, req *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	return f(ctx, req)
}

type legacyNativeAccounts struct{ account uuid.UUID }

func (f legacyNativeAccounts) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.account, nil
}

type legacyNativeBlocks struct{ blocked bool }

func (f *legacyNativeBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.blocked, nil
}

type legacyNativePrivacy struct{ audience privacy.Audience }

func (f *legacyNativePrivacy) AllowChatSpaceInvitesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return f.audience, nil
}

// Real private ingress + real PostgreSQL18/native45 frame/publication. Auth and
// Role responses below are explicitly controlled protocol fixtures, not live
// owner authorization, protected transport or a public end-to-end acceptance.
func TestLegacyAdmissionDispatchNative45ReadyReplayAndGuestInvite(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join("..", "..", "..", "migrations", "space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &store.SpaceStore{Pool: db.Pool}
	owner, ownerAccount := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, owner, "native legacy dispatcher", "", "public")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `UPDATE public.spaces SET allow_guests=true WHERE id=$1`, space.ID)
	require.NoError(t, err, "explicit pre-native fixture configuration only")
	invite, err := admin.CreateInvite(ctx, store.CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	manual, err := admin.CreateSpace(ctx, owner, "manual legacy dispatcher", "", "public")
	require.NoError(t, err)
	_, err = admin.SetEntryPolicy(ctx, store.SetEntryPolicyInput{SpaceID: manual.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}, RequireManualApproval: true}})
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	require.NoError(t, admin.InitializeNativeMembership(ctx, manual.ID))
	db.Run(true, "goto", "45")
	_, err = admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	login := "legacy_dispatch_" + uuid.NewString()[:8]
	_, err = db.Pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{login}.Sanitize()+" LOGIN PASSWORD 'isolated-fixture-only'; GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{login}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{login}.Sanitize())
	require.NoError(t, err)
	config := db.Pool.Config().Copy()
	config.ConnConfig.User, config.ConnConfig.Password = login, "isolated-fixture-only"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var actual string
	require.NoError(t, pool.QueryRow(ctx, `SELECT session_user`).Scan(&actual))
	require.Equal(t, login, actual, "independent nonowner LOGIN, not SET ROLE or exported stale DSN")
	st := &store.SpaceStore{Pool: pool}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	var ownerCalls, roleCalls atomic.Int64
	var invokeStarted atomic.Int64
	var ownerOutage atomic.Bool
	var roleOutage atomic.Bool
	guests := map[string]bool{}
	subjects := &admissionauthority.Client{Issuer: spaceIssuer, Auth: legacyNativeSubjectRPC(func(_ context.Context, req *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls.Add(1)
		t.Logf("owner phase %s after %s", req.Phase, time.Since(time.Unix(0, invokeStarted.Load())))
		if ownerOutage.Load() {
			return nil, errors.New("controlled owner outage")
		}
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(req)
		require.NoError(t, e)
		sha := sha256.Sum256(raw)
		kind := "regular"
		if guests[req.ProfileId] {
			kind = "guest"
		}
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: req.AccountId, ProfileId: req.ProfileId, AccountType: kind, AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	roleID := uuid.NewString()
	roles := &memberadmissionrole.Client{Issuer: spaceIssuer, Role: legacyNativeRoleRPC(func(ctx context.Context, req *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
		roleCalls.Add(1)
		if roleOutage.Load() {
			return nil, errors.New("controlled Role outage")
		}
		t.Logf("Role fixture begins after %s", time.Since(time.Unix(0, invokeStarted.Load())))
		facts, e := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId})
		require.NoError(t, e)
		require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING, facts.Ordinary.State, "controlled receipt still requires genuine committed native PREPARING")
		intent, e := st.LookupMemberAdmission(ctx, uuid.MustParse(req.AdmissionOperationId))
		require.NoError(t, e)
		require.Equal(t, req.MembershipGeneration, intent.Generation)
		require.True(t, bytes.Equal(req.AdmissionBindingSha256, intent.BindingSHA256))
		t.Logf("Role native observation completes after %s", time.Since(time.Unix(0, invokeStarted.Load())))
		binding := memberadmission.EnsureBinding{ProtocolVersion: req.ProtocolVersion, SpaceID: req.SpaceId, ProfileID: req.ProfileId, AdmissionOperationID: req.AdmissionOperationId, MembershipGeneration: req.MembershipGeneration, AdmissionBindingSHA256: req.AdmissionBindingSha256}
		digest, e := memberadmission.EnsureReceiptFingerprint(binding, roleID)
		require.NoError(t, e)
		return &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: req.ProtocolVersion, SpaceId: req.SpaceId, ProfileId: req.ProfileId, AdmissionOperationId: req.AdmissionOperationId, MembershipGeneration: req.MembershipGeneration, AdmissionBindingSha256: req.AdmissionBindingSha256, RoleId: roleID, ReceiptSha256: digest[:]}}, nil
	})}
	blocks := &legacyNativeBlocks{}
	svc := &SpaceGRPC{Store: st, ProfileAccounts: legacyNativeAccounts{ownerAccount}, Blocks: blocks, Privacy: &legacyNativePrivacy{privacy.EveryoneWithGuests()}, LegacyAdmission: &LegacyAdmissionDependencies{Subjects: subjects, Roles: roles}}
	var dispatchOverride func(context.Context, proto.Message) (any, error)
	invoke := func(account, profile uuid.UUID, request proto.Message) (*spacev1.SpaceMembership, error) {
		method := spacev1.SpaceService_JoinSpace_FullMethodName
		id := uuid.NewString()
		switch req := request.(type) {
		case *spacev1.JoinSpaceRequest:
			if req.OperationId != nil {
				id = *req.OperationId
			}
		case *spacev1.JoinByInviteRequest:
			method = spacev1.SpaceService_JoinByInvite_FullMethodName
			if req.OperationId != nil {
				id = *req.OperationId
			}
		}
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: st.ClaimAdmissionPrincipalReplay}
		invokeStarted.Store(time.Now().UnixNano())
		result, e := v.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			if dispatchOverride != nil {
				return dispatchOverride(ctx, request)
			}
			if req, ok := request.(*spacev1.JoinSpaceRequest); ok {
				return svc.JoinSpace(ctx, req)
			}
			return svc.JoinByInvite(ctx, request.(*spacev1.JoinByInviteRequest))
		})
		if e != nil {
			return nil, e
		}
		if response, ok := result.(*spacev1.JoinSpaceResponse); ok {
			return response.SpaceMembership, nil
		}
		return result.(*spacev1.JoinByInviteResponse).SpaceMembership, nil
	}
	account, profile := uuid.New(), uuid.New()
	op := uuid.NewString()
	request := &spacev1.JoinSpaceRequest{SpaceId: space.ID.String(), OperationId: &op}
	member, err := invoke(account, profile, request)
	require.NoError(t, err, "actual protected dispatcher must reach native publication, not a disabled stub")
	require.Equal(t, profile.String(), member.ProfileId)
	beforeOwner, beforeRole := ownerCalls.Load(), roleCalls.Load()
	require.NoError(t, st.RemoveMember(ctx, space.ID, profile))
	ownerOutage.Store(true)
	replayed, err := invoke(account, profile, request)
	require.NoError(t, err, "historical ACK survives leave and owner outage without new membership")
	require.True(t, proto.Equal(member, replayed))
	require.Equal(t, beforeOwner, ownerCalls.Load())
	require.Equal(t, beforeRole, roleCalls.Load())
	ownerOutage.Store(false)
	guestAccount, guestProfile := uuid.New(), uuid.New()
	guests[guestProfile.String()] = true
	inviteOp := uuid.NewString()
	invited, err := invoke(guestAccount, guestProfile, &spacev1.JoinByInviteRequest{Code: invite.Code, OperationId: &inviteOp})
	require.NoError(t, err)
	require.Equal(t, guestProfile.String(), invited.ProfileId)
	var guest bool
	var exactInvite uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT guest,invite_id FROM public.space_join_attempts WHERE profile_id=$1`, guestProfile).Scan(&guest, &exactInvite))
	require.True(t, guest, "current Subject type overrides the regular signed ingress hint")
	require.Equal(t, invite.ID, exactInvite)
	var uses int
	require.NoError(t, pool.QueryRow(ctx, `SELECT use_count FROM public.invites WHERE id=$1`, invite.ID).Scan(&uses))
	require.Equal(t, 1, uses)
	guestDirect := uuid.New()
	guests[guestDirect.String()] = true
	_, err = invoke(uuid.New(), guestDirect, &spacev1.JoinSpaceRequest{SpaceId: space.ID.String()})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "fresh guest classification cannot join directly")

	// A failed inert Role call leaves the original durable A/basis/P, not a
	// second admission. A fresh private invocation resumes that exact P.
	retryAccount, retryProfile := uuid.New(), uuid.New()
	retryOp := uuid.NewString()
	retryRequest := &spacev1.JoinSpaceRequest{SpaceId: space.ID.String(), OperationId: &retryOp}
	roleOutage.Store(true)
	_, err = invoke(retryAccount, retryProfile, retryRequest)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var originalP uuid.UUID
	var originalState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT operation_id,state FROM public.space_member_admission_intents WHERE space_id=$1 AND profile_id=$2`, space.ID, retryProfile).Scan(&originalP, &originalState))
	require.Equal(t, "PREPARING", originalState)
	roleOutage.Store(false)
	_, err = invoke(retryAccount, retryProfile, retryRequest)
	require.NoError(t, err)
	var publishedP uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT admission_operation_id FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, retryProfile).Scan(&publishedP))
	require.Equal(t, originalP, publishedP)
	blocks.blocked = true
	blockedProfile := uuid.New()
	_, err = invoke(uuid.New(), blockedProfile, &spacev1.JoinSpaceRequest{SpaceId: space.ID.String()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, blockedProfile).Scan(&count))
	require.Zero(t, count)
	blocks.blocked = false
	manualAccount, manualProfile := uuid.New(), uuid.New()
	manualOp := uuid.NewString()
	_, err = invoke(manualAccount, manualProfile, &spacev1.JoinSpaceRequest{SpaceId: manual.ID.String(), OperationId: &manualOp})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "manual outcome is durable pending, not membership")
	aliasOp := uuid.NewString()
	aliasRequest := &spacev1.JoinSpaceRequest{SpaceId: manual.ID.String(), OperationId: &aliasOp}
	dispatchOverride = func(callctx context.Context, request proto.Message) (any, error) {
		p, e := st.PrepareLegacyJoinStart(callctx, request)
		if e != nil {
			return nil, e
		}
		r, e := p.OwnerRequest()
		if e != nil {
			return nil, e
		}
		proof, e := subjects.Observe(callctx, r)
		if e != nil {
			return nil, e
		}
		d, e := p.Seal(proof)
		if e != nil {
			return nil, e
		}
		a, e := st.CommitLegacyJoinStart(callctx, d)
		if e != nil {
			return nil, e
		}
		require.Equal(t, "PENDING", a.State)
		return nil, status.Error(codes.Unavailable, "simulated process loss after committed pending pin")
	}
	_, err = invoke(manualAccount, manualProfile, aliasRequest)
	require.Equal(t, codes.Unavailable, status.Code(err))
	dispatchOverride = nil
	beforeOwner, beforeRole = ownerCalls.Load(), roleCalls.Load()
	_, err = invoke(manualAccount, manualProfile, aliasRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "restarted pinned PENDING uses distinct fresh START resume, not Context2")
	require.Equal(t, beforeOwner+1, ownerCalls.Load())
	require.Equal(t, beforeRole, roleCalls.Load())
	ownerOutage.Store(true)
	_, err = invoke(manualAccount, manualProfile, aliasRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "completed pending alias replays without owner calls")
	require.Equal(t, beforeOwner+1, ownerCalls.Load())
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, manual.ID, manualProfile).Scan(&count))
	require.Zero(t, count)
}
