package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
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
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type settingsPermissionFixture func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f settingsPermissionFixture) CheckPermission(c context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(c, r)
}

// A completed row is explicit historical storage, not proof of a new policy
// mutation. Replay uses real signed private ingress and no owner/Role client.
func TestAdmissionSettingsOriginalSafeReplayWithoutOwnerCalls(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	_, setupErr := st.GetEntryPolicy(ctx, space)
	require.NoError(t, setupErr, "historical fixture initializes its current policy before replay")
	for _, name := range []string{"000040_join_decision_basis_v2.up.sql", "000043_join_public_operation_context.up.sql", "000045_join_legacy_selection_context.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	account, profile := uuid.New(), uuid.New()
	request := &spacev1.SetEntryPolicyRequest{SpaceId: space.String(), OperationId: uuid.NewString(), ExpectedPolicyVersion: 1, PrivatePolicyJson: []byte(`{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":false,"questions":[]}`)}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	var elapsed atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Clock: clock, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
	invoke := func(req *spacev1.SetEntryPolicyRequest, acct uuid.UUID, handler func(context.Context) (any, error)) (any, error) {
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_SetEntryPolicy_FullMethodName, RequestID: id, RequestHash: hash, AccountID: acct.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, e)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		return verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_SetEntryPolicy_FullMethodName}, func(c context.Context, _ any) (any, error) { return handler(c) })
	}
	_, err = st.PrepareEntryPolicySet(ctx, request)
	require.Error(t, err, "raw context cannot reserve a settings operation")
	_, err = invoke(request, account, func(c context.Context) (any, error) { return st.PrepareEntryPolicySet(c, request) })
	require.NoError(t, err, "actual private call reserves canonical settings operation before owner I/O")
	var state, method string
	var selected *uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,request_method,selected_attempt_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId).Scan(&state, &method, &selected))
	require.Equal(t, "RESERVED", state)
	require.Equal(t, spacev1.SpaceService_SetEntryPolicy_FullMethodName, method)
	require.Nil(t, selected)
	ack := &spacev1.SetEntryPolicyResponse{OperationId: request.OperationId, EntryPolicyVersion: 2}
	rawAck, err := proto.MarshalOptions{Deterministic: true}.Marshal(ack)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId, rawAck)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE space_entry_policies SET version=9 WHERE space_id=$1`, space)
	require.NoError(t, err)
	api := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{})
	result, err := invoke(request, account, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, request) })
	require.NoError(t, err)
	require.True(t, proto.Equal(ack, result.(*spacev1.SetEntryPolicyResponse)))
	public, err := protojson.Marshal(result.(*spacev1.SetEntryPolicyResponse))
	require.NoError(t, err)
	require.NotContains(t, string(public), "privatePolicyJson")
	var version int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT version FROM space_entry_policies WHERE space_id=$1`, space).Scan(&version))
	require.EqualValues(t, 9, version, "replay never reapplies old policy")
	equivalent := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	equivalent.PrivatePolicyJson = []byte(`{ "questions":[],"require_manual_approval":false,"require_captcha":false,"require_phone":false,"schema_version":1 }`)
	result, err = invoke(equivalent, account, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, equivalent) })
	require.NoError(t, err, "canonical-equivalent policy has the same semantic operation H")
	require.True(t, proto.Equal(ack, result.(*spacev1.SetEntryPolicyResponse)))
	changed := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	changed.ExpectedPolicyVersion = 3
	_, err = invoke(changed, account, func(c context.Context) (any, error) { return api.SetEntryPolicy(c, changed) })
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	_, err = invoke(request, uuid.New(), func(c context.Context) (any, error) { return api.SetEntryPolicy(c, request) })
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same profile/op cannot disclose another account's outcome")
	// Fresh owner/permission capabilities are concrete private types. Their
	// controlled responses do not qualify the missing native45 readiness gate.
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	ownerCalls, permissionCalls := 0, 0
	subjects := &admissionauthority.Client{Issuer: spaceIssuer, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY, r.Phase)
		require.Equal(t, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR, r.SubjectRole)
		require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, e)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	allowSettings := true
	settings := &admissionauthority.SettingsClient{Issuer: spaceIssuer, Role: settingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		permissionCalls++
		require.Equal(t, admissionauthority.ManageEntryPolicyPermission, r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: allowSettings}, nil
	})}
	fresh := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	fresh.OperationId = uuid.NewString()
	fresh.ExpectedPolicyVersion = 9
	_, err = invoke(fresh, account, func(c context.Context) (any, error) {
		p, e := st.PrepareEntryPolicySet(c, fresh)
		if e != nil {
			return nil, e
		}
		expected, e := p.OwnerRequest()
		if e != nil {
			return nil, e
		}
		subject, e := subjects.Observe(c, expected)
		if e != nil {
			return nil, e
		}
		permission, e := settings.Observe(c, subject, expected)
		if e != nil {
			return nil, e
		}
		decision, e := p.Seal(subject, permission)
		require.NoError(t, e, "actual distinct settings proof must seal this exact private operation")
		other := proto.Clone(fresh).(*spacev1.SetEntryPolicyRequest)
		other.OperationId = uuid.NewString()
		_, reuseErr := invoke(other, account, func(otherContext context.Context) (any, error) {
			otherProposal, err := st.PrepareEntryPolicySet(otherContext, other)
			if err != nil {
				return nil, err
			}
			return otherProposal.Seal(subject, permission)
		})
		require.Error(t, reuseErr, "same actor cannot transplant proof to another operation")
		return st.CommitEntryPolicySet(c, decision)
	})
	require.ErrorIs(t, err, store.ErrEffectiveEpisodeUnavailable, "unknown native readiness must reject before policy/outcome writes")
	require.Equal(t, 1, ownerCalls)
	require.Equal(t, 1, permissionCalls)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT version FROM space_entry_policies WHERE space_id=$1`, space).Scan(&version))
	require.EqualValues(t, 9, version)
	var effects int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_entry_policy_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, fresh.OperationId).Scan(&effects))
	require.Zero(t, effects)
	configured := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: subjects, Settings: settings})
	denied := proto.Clone(fresh).(*spacev1.SetEntryPolicyRequest)
	denied.OperationId = uuid.NewString()
	allowSettings = false
	_, err = invoke(denied, account, func(c context.Context) (any, error) { return configured.SetEntryPolicy(c, denied) })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 2, ownerCalls)
	require.Equal(t, 2, permissionCalls)
	// Old30 receipt bytes are not sufficient original-actor/method evidence.
	legacy := proto.Clone(fresh).(*spacev1.SetEntryPolicyRequest)
	legacy.OperationId = uuid.NewString()
	policy, e := entrypolicy.Parse(legacy.PrivatePolicyJson)
	require.NoError(t, e)
	_, e = st.SetEntryPolicy(ctx, store.SetEntryPolicyInput{SpaceID: space, ActorProfileID: profile, OperationID: uuid.MustParse(legacy.OperationId), ExpectedVersion: 9, Policy: policy})
	require.NoError(t, e, "explicit historical legacy receipt fixture")
	_, err = invoke(legacy, account, func(c context.Context) (any, error) { return configured.SetEntryPolicy(c, legacy) })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, legacy.OperationId).Scan(&effects))
	require.Zero(t, effects, "old receipt cannot be backfilled from today's actor")
	require.Equal(t, 2, ownerCalls)
	require.Equal(t, 2, permissionCalls)
	// A database's old default isolation must not turn an exact concurrent
	// completion into serialization failure instead of its saved safe result.
	cfg := st.Pool.Config().Copy()
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_isolation"] = "repeatable read"
	rrPool, e := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, e)
	t.Cleanup(rrPool.Close)
	rrStore := &store.SpaceStore{Pool: rrPool}
	rrAPI := NewAdmissionServer(&SpaceGRPC{Store: rrStore}, rrStore, AdmissionDependencies{})
	raceRequest := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	raceRequest.OperationId = uuid.NewString()
	_, e = invoke(raceRequest, account, func(c context.Context) (any, error) { return rrStore.PrepareEntryPolicySet(c, raceRequest) })
	require.NoError(t, e)
	raceAck := &spacev1.SetEntryPolicyResponse{OperationId: raceRequest.OperationId, EntryPolicyVersion: 2}
	raceRaw, e := proto.MarshalOptions{Deterministic: true}.Marshal(raceAck)
	require.NoError(t, e)
	winner, e := st.Pool.Begin(ctx)
	require.NoError(t, e)
	defer winner.Rollback(ctx)
	_, e = winner.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2`, profile, raceRequest.OperationId, raceRaw)
	require.NoError(t, e)
	var winnerPID int32
	require.NoError(t, winner.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&winnerPID))
	type replayResult struct {
		response any
		err      error
	}
	raceDone := make(chan replayResult, 1)
	go func() {
		r, e := invoke(raceRequest, account, func(c context.Context) (any, error) { return rrAPI.SetEntryPolicy(c, raceRequest) })
		raceDone <- replayResult{r, e}
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, winnerPID).Scan(&blocked) == nil && blocked
	}, 3*time.Second, 5*time.Millisecond)
	require.NoError(t, winner.Commit(ctx))
	var raced replayResult
	select {
	case raced = <-raceDone:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent completed replay did not resume")
	}
	require.NoError(t, raced.err)
	require.True(t, proto.Equal(raceAck, raced.response.(*spacev1.SetEntryPolicyResponse)))
	require.Equal(t, 2, ownerCalls)
	require.Equal(t, 2, permissionCalls)
	// A completed race still requires the original ingress to remain live after
	// operation-row waits. No Subject/Role/current membership recheck is added.
	holder, e := st.Pool.Begin(ctx)
	require.NoError(t, e)
	defer holder.Rollback(ctx)
	var locked uuid.UUID
	require.NoError(t, holder.QueryRow(ctx, `SELECT operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, profile, request.OperationId).Scan(&locked))
	var pid int32
	require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() {
		_, e := invoke(request, account, func(c context.Context) (any, error) { return configured.SetEntryPolicy(c, request) })
		done <- e
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked) == nil && blocked
	}, 3*time.Second, 5*time.Millisecond)
	elapsed.Store(int64(5 * time.Second))
	require.NoError(t, holder.Commit(ctx))
	select {
	case err = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("completed replay did not finish after row release")
	}
	require.Error(t, err, "completed acknowledgement cannot outlive its current ingress")
	require.Equal(t, 2, ownerCalls)
	require.Equal(t, 2, permissionCalls)
}
