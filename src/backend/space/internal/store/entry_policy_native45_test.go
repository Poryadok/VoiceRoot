package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"path/filepath"
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
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
)

type native44SettingsSubjectFixture func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f native44SettingsSubjectFixture) ObserveSpaceAdmissionSubject(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, r)
}

type native44SettingsPermissionFixture func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f native44SettingsPermissionFixture) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(ctx, r)
}

// Exact trusted44 -> actual45 migration -> administrative seal transition.
// Auth/Role/nonce reads below remain controlled protocol fixtures, not assembled
// Java/Role authority. No version rewrite, skipped migration or raw seal shim.
func TestEntryPolicyPrivateWriterWithRealNative45AndSealTransition(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	admin := &SpaceStore{Pool: db.Pool}
	create, account, profile := signedNativeCreatorCurrent(t, admin, &spacev1.CreateSpaceRequest{Name: "Settings current native member", Visibility: "private"})
	space, err := create()
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID), "real administrative UNKNOWN cutover, not a caller readiness flag")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.RequireScopedNativeMembershipReadiness(ctx, space.ID) }))
	db.Run(true, "up")
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.RequireScopedNativeMembershipReadiness(ctx, space.ID) })
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "actual45 remains closed before its fixed installer transition")
	transition, err := admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, transition.Changed)
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.RequireScopedNativeMembershipReadiness(ctx, space.ID) }))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ownerCalls, permissionCalls := 0, 0
	prepare := func(request *spacev1.SetEntryPolicyRequest) (*EntryPolicySetProposal, *atomic.Int64, func() (*VerifiedEntryPolicySet, error)) {
		t.Helper()
		now := time.Now().UTC().Truncate(time.Second)
		elapsed := &atomic.Int64{}
		clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
		gateway, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
		require.NoError(t, e)
		issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
		require.NoError(t, e)
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_SetEntryPolicy_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, e)
		verifier := lifecycleprincipal.Verifier{Clock: clock, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		result, e := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_SetEntryPolicy_FullMethodName}, func(c context.Context, _ any) (any, error) { return st.PrepareEntryPolicySet(c, request) })
		require.NoError(t, e)
		p := result.(*EntryPolicySetProposal)
		seal := func() (*VerifiedEntryPolicySet, error) {
			expected, e := p.OwnerRequest()
			if e != nil {
				return nil, e
			}
			subjectClient := &admissionauthority.Client{Issuer: issuer, Clock: clock, Auth: native44SettingsSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				ownerCalls++
				require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY, r.Phase)
				require.Equal(t, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR, r.SubjectRole)
				require.Equal(t, account.String(), r.AccountId)
				require.Equal(t, profile.String(), r.ProfileId)
				raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
				require.NoError(t, e)
				sha := sha256.Sum256(raw)
				return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
			})}
			settingsClient := &admissionauthority.SettingsClient{Issuer: issuer, Clock: clock, Role: native44SettingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
				permissionCalls++
				require.Equal(t, admissionauthority.ManageEntryPolicyPermission, r.PermissionName)
				require.Equal(t, space.ID.String(), r.SpaceId)
				require.Equal(t, profile.String(), r.ProfileId)
				return &rolev1.CheckPermissionResponse{Allowed: true}, nil
			})}
			subject, e := subjectClient.Observe(ctx, expected)
			if e != nil {
				return nil, e
			}
			permission, e := settingsClient.Observe(ctx, subject, expected)
			if e != nil {
				return nil, e
			}
			return p.Seal(subject, permission)
		}
		return p, elapsed, seal
	}
	request := &spacev1.SetEntryPolicyRequest{SpaceId: space.ID.String(), OperationId: uuid.NewString(), ExpectedPolicyVersion: 1, PrivatePolicyJson: []byte(`{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":true,"questions":[]}`)}
	p, _, seal := prepare(request)
	decision, err := seal()
	require.NoError(t, err)
	ack, err := st.CommitEntryPolicySet(ctx, decision)
	require.NoError(t, err, "actual native45 readiness and source permit real policy CAS")
	require.EqualValues(t, 2, ack.EntryPolicyVersion)
	var state string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId).Scan(&state))
	require.Equal(t, "COMPLETED", state)
	p, _, _ = prepare(request)
	replay, done, err := st.ReplayEntryPolicySet(ctx, p)
	require.NoError(t, err)
	require.True(t, done)
	require.True(t, proto.Equal(ack, replay))
	require.Equal(t, 1, ownerCalls)
	require.Equal(t, 1, permissionCalls)
	changed := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	changed.OperationId = uuid.NewString()
	changed.ExpectedPolicyVersion = 2
	changed.PrivatePolicyJson = []byte(`{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":false,"questions":[]}`)
	_, elapsed, seal := prepare(changed)
	decision, err = seal()
	require.NoError(t, err)
	holder, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(ctx)
	var locked uuid.UUID
	require.NoError(t, holder.QueryRow(ctx, `SELECT space_id FROM space_entry_policies WHERE space_id=$1 FOR UPDATE`, space.ID).Scan(&locked))
	var pid int32
	require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	doneWrite := make(chan error, 1)
	go func() { _, e := st.CommitEntryPolicySet(ctx, decision); doneWrite <- e }()
	require.Eventually(t, func() bool {
		var blocked bool
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked) == nil && blocked
	}, 3*time.Second, 5*time.Millisecond)
	elapsed.Store(int64(5 * time.Second))
	require.NoError(t, holder.Commit(ctx))
	require.Error(t, <-doneWrite, "post-write guard must roll back CAS plus both receipts")
	var version int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version FROM space_entry_policies WHERE space_id=$1`, space.ID).Scan(&version))
	require.EqualValues(t, 2, version)
	var receipts int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_entry_policy_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, changed.OperationId).Scan(&receipts))
	require.Zero(t, receipts)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, changed.OperationId).Scan(&state))
	require.Equal(t, "RESERVED", state)
	_, _, seal = prepare(changed)
	decision, err = seal()
	require.NoError(t, err)
	ack, err = st.CommitEntryPolicySet(ctx, decision)
	require.NoError(t, err)
	require.EqualValues(t, 3, ack.EntryPolicyVersion, "fresh exact retry resumes same reserved operation")
	noop := proto.Clone(changed).(*spacev1.SetEntryPolicyRequest)
	noop.OperationId = uuid.NewString()
	noop.ExpectedPolicyVersion = 3
	_, _, seal = prepare(noop)
	decision, err = seal()
	require.NoError(t, err)
	ack, err = st.CommitEntryPolicySet(ctx, decision)
	require.NoError(t, err)
	require.EqualValues(t, 3, ack.EntryPolicyVersion, "new semantic noop still consumes fresh private proofs")
	stale := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	stale.OperationId = uuid.NewString()
	_, _, seal = prepare(stale)
	decision, err = seal()
	require.NoError(t, err)
	_, err = st.CommitEntryPolicySet(ctx, decision)
	require.ErrorIs(t, err, ErrEntryPolicyVersion)
	require.Equal(t, 5, ownerCalls)
	require.Equal(t, 5, permissionCalls)
	secondTransition, err := admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.False(t, secondTransition.Changed, "exact maintenance replay never recuts current membership")
	require.Equal(t, transition.Epoch, secondTransition.Epoch)
	// The low-level helper is not permission to spoof a serving catalog via SET.
	var actual string
	require.NoError(t, serving.QueryRow(ctx, `SELECT session_user`).Scan(&actual))
	require.NotEqual(t, db.Pool.Config().ConnConfig.User, actual)
}
